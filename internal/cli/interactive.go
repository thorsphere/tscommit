// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package cli

// Import packages
import (
	"context"      // Import the context package for cancellation-aware operations.
	"fmt"          // Import the fmt package for formatted output and error wrapping.
	"regexp"       // Import the regexp package for matching breaking change footers.
	"strconv"      // Import the strconv package for formatting floating-point cost values.
	"strings"      // Import the strings package for string trimming and formatting.
	"unicode/utf8" // Import the utf8 package for counting characters (runes) in diffs.

	"github.com/thorsphere/lpcli"                 // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tscommit"              // Import the tscommit package for commit message generation.
	"github.com/thorsphere/tscommit/internal/git" // Import the internal git package for git operations.
	"github.com/thorsphere/tserr"                 // Import the tserr package for error reporting.
)

// promptParams holds the candidate message and display settings for a commit prompt.
type promptParams struct {
	Message   tscommit.Message // The candidate commit message and AI completion metadata.
	Effort    string           // The configured reasoning effort level.
	DiffChars int              // The staged diff length in characters (runes).
	Truncated bool             // Whether the staged diff hit the character limit.
	ShowAI    bool             // Whether to display the AI summary header and confidence score.
}

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
const maxDiffChars = 100000 // The staged diff size limit.

// breakingFooterRe matches a "BREAKING CHANGE:" or "BREAKING-CHANGE:" footer
// line (anchored to the start of a line).
var breakingFooterRe = regexp.MustCompile(`(?m)^BREAKING[- ]CHANGE:`)

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

// stagedDiff reads the staged git diff bounded to maxLen characters. It is
// the git diff seam, swapped via SetStagedDiff in export_test.go so unit
// tests never run the git binary.
var stagedDiff = git.StagedDiff

// commit creates a git commit with the message msg. It is the git commit
// seam, swapped via SetCommit in export_test.go so unit tests never run the
// git binary.
var commit = git.Commit

// generateMessage generates a commit message for diff with gen. It is the
// generation seam, swapped via SetGenerateMessage in export_test.go so unit
// tests script generation outcomes without a network call.
var generateMessage = func(ctx context.Context, gen *tscommit.Generator, diff string) (tscommit.Message, error) {
	// Delegate to the generator.
	return gen.Generate(ctx, diff)
}

// Run executes the interactive tscommit workflow with a background context.
func Run() error {
	// Delegate to RunContext with a background context.
	return RunContext(context.Background())
}

// RunContext executes the interactive tscommit workflow with ctx: builds a
// Generator from environment configuration, reads the staged git diff bounded
// to maxDiffChars characters, generates a commit message, prompts the user to
// accept, edit, regenerate, or cancel, and creates the git commit upon
// acceptance.
func RunContext(ctx context.Context) error {
	// Build the default configuration carrying the model and reasoning effort.
	cfg := tscommit.NewConfig()
	// Build the non-interactive generator; it validates the provider and API key.
	gen, err := tscommit.NewGenerator(cfg)
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
	// Measure the diff length in characters and check whether StagedDiff truncated it.
	diffChars, truncated := diffInfo(diff)
	// Generate the initial commit message and completion metadata.
	res, err := generateMessage(ctx, gen, diff)
	// Return the error if message generation fails.
	if err != nil {
		return err
	}
	// Set the initial candidate and enable the AI summary and confidence display.
	params := promptParams{
		Message:   res,
		Effort:    cfg.ReasoningEffort(),
		DiffChars: diffChars,
		Truncated: truncated,
		ShowAI:    true,
	}
	// Prompt the user until they accept, edit, regenerate, or cancel.
	for {
		// Ask the user what to do with the current message.
		choice, err := promptUser(ctx, p, params)
		// Return the error if prompting fails.
		if err != nil {
			return abortOr(ctx, err)
		}
		// Handle the user's choice.
		switch choice {
		case choiceAccept:
			// Create the git commit with the current message.
			return commit(ctx, params.Message.Text)
		case choiceEdit:
			// Open the message in the user's editor.
			edited, err := editInEditor(ctx, p, params.Message.Text)
			// Return the error if editing fails.
			if err != nil {
				return abortOr(ctx, err)
			}
			// Adopt the user's edit as the active commit message; it is authoritative.
			params.Message.Text = edited
			// Omit the AI generation summary and confidence score once the user has edited the message.
			params.ShowAI = false
		case choiceRegenerate:
			// Generate a fresh message and completion metadata on the same generator.
			res, err := generateMessage(ctx, gen, diff)
			// Return the error if message generation fails.
			if err != nil {
				return abortOr(ctx, err)
			}
			// Replace the candidate message and completion metadata with the fresh result.
			params.Message = res
			// Re-enable the AI summary and confidence display for the newly generated message.
			params.ShowAI = true
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

// diffInfo returns the staged diff character count and reports whether diff
// carries the truncation marker appended by git.StagedDiff. When truncated,
// the marker itself is excluded from diffChars so the count reflects the
// actual diff characters sent to the model.
func diffInfo(diff string) (diffChars int, truncated bool) {
	// Strip the truncation marker when present to measure only the diff content.
	trimmed, wasTruncated := strings.CutSuffix(diff, git.TruncatedSuffix)
	// Return the character (rune) count and truncation status.
	return utf8.RuneCountInString(trimmed), wasTruncated
}

// formatSummary returns the single-line generation summary header for params:
// the diff character count with an optional truncation indicator, the serving
// model with reasoning effort in parentheses, billed tokens, and cost in USD.
func formatSummary(params promptParams) string {
	// Include the truncation tag only when the staged diff was truncated.
	truncTag := ""
	// Append the truncation tag when params.Truncated is true.
	if params.Truncated {
		// Mark that the staged diff hit the character limit.
		truncTag = " (Truncated)"
	}
	// Fall back to n/a when the provider did not report a serving model.
	model := params.Message.Model
	// Replace an empty model identifier with the fallback label.
	if model == "" {
		// Indicate that no serving model was reported.
		model = "n/a"
	}
	// Fall back to n/a when no reasoning effort was configured.
	if params.Effort == "" {
		// Indicate that no explicit reasoning effort was configured.
		params.Effort = "n/a"
	}
	// Render the cost in USD without scientific notation.
	cost := "$" + strconv.FormatFloat(params.Message.Cost, 'f', -1, 64)
	// Assemble and return the compact summary header line with the reasoning
	// effort in brackets after the model name.
	return fmt.Sprintf("Diff: %d%s chars | Model: %s (%s) | Tokens: %d | Cost: %s", params.DiffChars, truncTag, model, params.Effort, params.Message.TotalTokens, cost)
}

// promptUser returns the user's selected choice after displaying the candidate
// commit message from params, the AI summary and confidence score if
// params.ShowAI is true, and a warning if the message declares a breaking
// change. It returns an empty choice and an error if prompting fails.
func promptUser(ctx context.Context, p *lpcli.Prompter, params promptParams) (string, error) {
	// Show the AI generation summary before the commit message when enabled.
	if params.ShowAI {
		// Print the compact summary header followed by a blank line.
		fmt.Fprintf(p.Out(), "%s\n\n", formatSummary(params))
	}
	// Show the candidate commit message so the user can review it.
	fmt.Fprintln(p.Out(), params.Message.Text)
	// Separate the message from the metadata, warning, and prompt for readability.
	fmt.Fprintln(p.Out())
	// Display the AI confidence score when the message came from the model.
	if params.ShowAI {
		// Print the confidence score as a rounded integer percentage.
		fmt.Fprintf(p.Out(), "Confidence: %.0f%%\n\n", params.Message.Confidence*100)
	}
	// Warn the user if the message contains a breaking-change marker.
	warnBreakingChange(p, params.Message.Text)
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

// hasBreakingChange reports whether msg declares a breaking change via the
// "!" header marker or a "BREAKING CHANGE:" footer.
// It ignores trailing newlines (git convention), matching validateMessage.
func hasBreakingChange(msg string) bool {
	// Ignore trailing newlines (git convention), matching validateMessage.
	msg = strings.TrimRight(msg, "\n")
	// Split the message into header and remainder.
	header, _, _ := strings.Cut(msg, "\n")
	// Check for the breaking-change marker: a "!" immediately before the
	// ": " separator (e.g. "feat!: x" or "feat(api)!: x"). This ignores
	// exclamation marks elsewhere in the description.
	if i := strings.Index(header, ": "); strings.HasSuffix(header[:max(i, 0)], "!") {
		return true
	}
	// Otherwise check for a "BREAKING CHANGE:" footer line.
	return breakingFooterRe.MatchString(msg)
}
