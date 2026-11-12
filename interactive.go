// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"bytes"   // Import the bytes package for buffering git output.
	"context" // Import the context package for cancellation-aware operations.
	"fmt"     // Import the fmt package for formatted output and error wrapping.
	"io"      // Import the io package for stream interfaces.
	"os"      // Import the os package for standard output and error streams.
	"os/exec" // Import the exec package for executing the git binary.
	"strings" // Import the strings package for string trimming and formatting.

	"github.com/thorsphere/lpcli" // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tserr" // Import the tserr package for error reporting.
)

// The choices the user can select when prompting for commit action.
const (
	choiceAccept     string = "accept"     // The default choice is to commit.
	choiceEdit       string = "edit"       // The user can also edit the message.
	choiceRegenerate string = "regenerate" // The user can also regenerate the message.
	choiceCancel     string = "cancel"     // The user can also cancel the commit.
)

// breakingWarning is the warning printed when a breaking change is detected.
const breakingWarning = "⚠ BREAKING CHANGE detected. This commit triggers a major version bump.\n\n" // The warning printed when a breaking change is detected.

// maxDiffChars is the maximum staged diff length in characters (runes)
// sent to the AI provider by the interactive CLI workflow.
const maxDiffChars = 30000 // The staged diff size limit.

// gitExec runs a git command with the given arguments, streaming stdout and
// stderr to the provided writers. It is a seam so unit tests can run without
// the git binary installed (swapped via SetExecGit in export_test.go).
type gitExec func(ctx context.Context, stdout, stderr io.Writer, args ...string) error

// commitChoices is the set of choices the user can select when prompting for
// commit action.
var commitChoices = []lpcli.Choice{
	// The default choice is to commit.
	{Value: choiceAccept, Key: "y", Aliases: []string{"yes"}, Label: "commit", IsDefault: true},
	// The user can also edit the message.
	{Value: choiceEdit, Key: "e", Aliases: []string{"edit"}, Label: "edit"},
	// The user can also regenerate the message.
	{Value: choiceRegenerate, Key: "r", Aliases: []string{"regen", "regenerate"}, Label: "regenerate"},
	// The user can also cancel the commit.
	{Value: choiceCancel, Key: "n", Aliases: []string{"no", "c", "cancel", "q", "quit"}, Label: "cancel"},
}

// newPrompter creates the shared lpcli prompter for the tscommit CLI. It is a
// package-level seam so tests can supply a custom prompter via SetNewPrompter.
var newPrompter = func() *lpcli.Prompter {
	// Create the prompter bound to the process standard streams.
	return lpcli.NewPrompter("tscommit")
}

// selectChoice asks the user to choose one option from opts using p. It is a
// package-level seam so unit tests can exercise every prompt outcome (including
// defensive branches) without depending on terminal input.
var selectChoice = func(ctx context.Context, p *lpcli.Prompter, opts lpcli.SelectOptions) (string, error) {
	// Delegate to the prompter's interactive selection prompt.
	return p.Prompt(ctx, opts)
}

// editMessage opens initialText in the user's editor and returns the edited
// content. It is the editor seam, swapped via SetEditMessage in
// export_test.go so unit tests never launch a real editor subprocess.
var editMessage = func(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
	// Open the editor and wait for the user to edit the message.
	return p.Edit(ctx, initialText)
}

// execGit executes a real git command via os/exec. It is the one piece of
// git execution that unit tests never run: tests must not depend on the git
// binary, so they swap this seam via SetExecGit.
var execGit gitExec = func(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	// Create the git command bound to the context.
	cmd := exec.CommandContext(ctx, "git", args...)
	// Redirect the command's output to the given writers.
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Run the command and return any error.
	return cmd.Run()
}

// Run executes the interactive tscommit workflow with a background context.
func Run() error {
	// Delegate to RunContext with a background context.
	return RunContext(context.Background())
}

// RunContext executes the interactive tscommit workflow with ctx: builds a
// Generator from environment configuration, reads and truncates the staged git
// diff, generates a commit message, prompts the user to accept, edit,
// regenerate, or cancel, and creates the git commit upon acceptance.
func RunContext(ctx context.Context) error {
	// Build the non-interactive generator; it validates the provider and API key.
	gen, err := NewGenerator()
	// Return the error if the generator cannot be built.
	if err != nil {
		return err
	}
	// Create the interactive prompter for the run.
	p := newPrompter()
	// Read the staged git diff bounded to maxDiffChars characters.
	diff, err := stagedDiff(ctx, maxDiffChars)
	// Return the error if reading the staged diff fails.
	if err != nil {
		return err
	}
	// Generate the initial commit message and model confidence score.
	res, err := gen.Generate(ctx, diff)
	// Return the error if message generation fails.
	if err != nil {
		return err
	}
	// Track the active commit message text and confidence across loop turns.
	msg, confidence := res.Text, res.Confidence
	// Track whether the current message came directly from the AI model.
	showConfidence := true
	// Prompt the user until they accept, edit, regenerate, or cancel.
	for {
		// Ask the user what to do with the current message.
		choice, err := promptUser(ctx, p, msg, confidence, showConfidence)
		// Return the error if prompting fails.
		if err != nil {
			return abortOr(ctx, err)
		}
		// Handle the user's choice.
		switch choice {
		case choiceAccept:
			// Create the git commit with the current message.
			return commit(ctx, msg)
		case choiceEdit:
			// Open the message in the user's editor.
			edited, err := editInEditor(ctx, p, msg)
			// Return the error if editing fails.
			if err != nil {
				return abortOr(ctx, err)
			}
			// Adopt the user's edit as the new message; it is authoritative.
			msg = edited
			// Omit the AI confidence score once the user has edited the message.
			showConfidence = false
		case choiceRegenerate:
			// Generate a fresh message and confidence score on the same generator.
			res, err = gen.Generate(ctx, diff)
			// Return the error if message generation fails.
			if err != nil {
				return abortOr(ctx, err)
			}
			// Adopt the freshly generated message and confidence score.
			msg, confidence = res.Text, res.Confidence
			// Re-enable the AI confidence display for the newly generated message.
			showConfidence = true
		case choiceCancel:
			// Return an aborted error when the user cancels.
			return tserr.Aborted("commit")
		default:
			// Ignore an unhandled choice so the pending message survives;
			// warn the user and ask again.
			fmt.Fprintf(p.Out(), "Ignoring unhandled option %q; please choose again.\n", choice)
			// Re-prompt with the unchanged message.
			continue
		}
	}
}

// abortOr maps err to tserr.Aborted("commit") when ctx is done and the run
// was therefore aborted by the user (Ctrl-C, SIGTERM); otherwise it returns
// err unchanged so genuine failures surface as themselves. err must be
// non-nil.
func abortOr(ctx context.Context, err error) error {
	// If the context is done, the user aborted the run.
	if ctx.Err() != nil {
		// Return an aborted error to give every cancel path the same shape.
		return tserr.Aborted("commit")
	}
	// Return the original error to indicate a genuine failure.
	return err
}

// warnBreakingChange prints a prominent warning when msg contains a
// Conventional Commits breaking-change marker.
func warnBreakingChange(p *lpcli.Prompter, msg string) {
	// If the message contains a breaking change marker, print a prominent
	// warning.
	if hasBreakingChange(msg) {
		fmt.Fprint(p.Out(), breakingWarning)
	}
}

// promptUser returns the choice value selected by the user after showing msg,
// displaying confidence when showConfidence is true, and warning when msg
// carries a breaking-change marker, or an empty string and error if prompting
// fails.
func promptUser(ctx context.Context, p *lpcli.Prompter, msg string, confidence float64, showConfidence bool) (string, error) {
	// Show the candidate commit message so the user can review it.
	fmt.Fprintln(p.Out(), msg)
	// Separate the message from the metadata, warning, and prompt for readability.
	fmt.Fprintln(p.Out())
	// Display the AI confidence score when the message came from the model.
	if showConfidence {
		// Print the confidence score as a rounded integer percentage.
		fmt.Fprintf(p.Out(), "Confidence: %.0f%%\n\n", confidence*100)
	}
	// Warn the user if the message contains a breaking-change marker.
	warnBreakingChange(p, msg)
	// Ask the user what to do via the selection seam.
	choice, err := selectChoice(ctx, p, lpcli.SelectOptions{
		Message: "Apply this commit?",
		Choices: commitChoices,
	})
	// If prompting fails (including a user interrupt), return an empty
	// choice and the error; the caller normalizes user aborts.
	if err != nil {
		return "", err
	}
	// Return the selected choice value and nil, to indicate success.
	return choice, nil
}

// editInEditor opens initialText in the user's editor and returns the edited
// content, or an error if the user cancels or leaves the editor empty.
func editInEditor(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
	// Open the editor and wait for the user to edit the message.
	edited, err := editMessage(ctx, p, initialText)
	// If the user cancels, return an error.
	if err != nil {
		return "", err
	}
	// If the user leaves the editor empty, return an error; otherwise return the edited message.
	return checkEdited(edited)
}

// checkEdited returns edited unchanged, or an error if the user left it empty
// (only whitespace).
func checkEdited(edited string) (string, error) {
	// If the message is empty after trimming, return an error.
	if strings.TrimSpace(edited) == "" {
		return "", tserr.Empty("commit message after edit")
	}
	// Return the edited message and nil, to indicate success.
	return edited, nil
}

// runGit runs git, streaming stdout and stderr to the given writers while
// capturing stderr so failures can carry git's diagnostic text.
func runGit(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	// Capture stderr alongside the caller's writer.
	var errBuf bytes.Buffer
	// Run git and enrich any error with the captured diagnostic.
	if err := execGit(ctx, stdout, io.MultiWriter(stderr, &errBuf), args...); err != nil {
		return gitErr(err, errBuf.String())
	}
	// Return no error to indicate success.
	return nil
}

// gitErr enriches a git command error with the command's stderr output, which
// carries git's diagnostic text (e.g. "not a git repository"). The original
// error is wrapped, so errors.Is and errors.As still reach it.
func gitErr(err error, stderr string) error {
	// If git wrote no diagnostic, return the error unchanged.
	if stderr == "" {
		return err
	}
	// Otherwise, append the diagnostic text to the error.
	return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
}

// stagedDiff returns the staged diff, truncated to the first maxLen
// characters when it is longer, with a truncation marker appended.
// Truncation counts characters (runes), matching the display-length
// convention used by validateMessage. It returns an error if nothing is
// staged or if maxLen is negative.
func stagedDiff(ctx context.Context, maxLen int) (string, error) {
	// If maxLen is negative, return an error.
	if maxLen < 0 {
		return "", tserr.Higher(&tserr.HigherArgs{Var: "maxLen", Actual: int64(maxLen), LowerBound: 0})
	}
	// Capture the diff from git using the flags --staged and -W. stderr is
	// discarded: runGit attaches any diagnostic text to the returned error.
	var outDiff bytes.Buffer
	// Run git diff against the staging area with full function context.
	if err := runGit(ctx, &outDiff, io.Discard, "diff", "--staged", "-W"); err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "diff", Fn: "git", Err: err})
	}
	// If the diff is empty, return an empty string and an error.
	if outDiff.Len() == 0 {
		return "", tserr.Empty("git staged changes")
	}
	// Truncate the diff to maxLen characters, counting runes rather than
	// bytes so multi-byte text is measured as a reader counts it and the
	// cut never splits a character.
	diff := outDiff.String()
	// If the diff exceeds maxLen characters, cut at the character boundary
	// and append the truncation marker.
	if r := []rune(diff); len(r) > maxLen {
		diff = string(r[:maxLen]) + "\n... (truncated)"
	}
	// Return the diff and no error to indicate success.
	return diff, nil
}

// commit creates a git commit with the message msg.
func commit(ctx context.Context, msg string) error {
	// Create a new git commit with the given message msg, streaming output to
	// os.Stdout and os.Stderr.
	if err := runGit(ctx, os.Stdout, os.Stderr, "commit", "-m", msg); err != nil {
		// If an error occurs, return an error.
		return tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: err})
	}
	// Return no error to indicate success.
	return nil
}
