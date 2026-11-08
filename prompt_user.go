// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context" // Import the context package for cancellation-aware prompting.
	"fmt"     // Import the fmt package for printing the breaking-change warning.
	"strings" // Import the strings package for trimming edited messages.

	"github.com/thorsphere/lpcli" // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

// The choices the user can select when prompting for commit action.
const (
	choiceAccept     string = "accept"     // The default choice is to commit.
	choiceEdit       string = "edit"       // The user can also edit the message.
	choiceRegenerate string = "regenerate" // The user can also regenerate the message.
	choiceCancel     string = "cancel"     // The user can also cancel the commit.
)

const breakingWarning = "⚠ BREAKING CHANGE detected. This commit triggers a major version bump.\n\n" // The warning printed when a breaking change is detected.

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

// newPrompter creates the shared lpcli prompter for the tscommit CLI.
func newPrompter() *lpcli.Prompter {
	// Create the prompter.
	return lpcli.NewPrompter("tscommit")
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

// promptUser shows the candidate commit message, warns when it carries a
// breaking-change marker, and asks the user whether to accept, edit,
// regenerate, or cancel. Returns the selected choice value, or an empty
// string if prompting fails.
func promptUser(ctx context.Context, p *lpcli.Prompter, msg string) (string, error) {
	// Show the candidate commit message so the user can review it.
	fmt.Fprintln(p.Out(), msg)
	// Separate the message from the warning and prompt for readability.
	fmt.Fprintln(p.Out())
	// Warn the user if the message contains a breaking-change marker.
	warnBreakingChange(p, msg)
	// Ask the user what to do.
	choice, err := p.Prompt(ctx, lpcli.SelectOptions{
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

// editMessage opens initialText in the user's editor and returns the edited
// content. It is the editor seam, swapped via SetEditMessage in
// export_test.go so unit tests never launch a real editor subprocess.
var editMessage = func(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
	// Open the editor and wait for the user to edit the message.
	return p.Edit(ctx, initialText)
}

// editInEditor opens the message in the user's editor and returns the edited content.
// Returns an error if the user cancels or the editor is empty.
func editInEditor(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
	// Open the editor and wait for the user to edit the message.
	edited, err := editMessage(ctx, p, initialText)
	// If the user cancels, return an error.
	if err != nil {
		return "", err
	}
	// If the user leaves the editor empty, return an error.
	// Otherwise, return the edited message.
	return checkEdited(edited)
}

// checkEdited returns the edited message, or an error if the user left it
// empty (only whitespace).
func checkEdited(edited string) (string, error) {
	// If the message is empty after trimming, return an error.
	if strings.TrimSpace(edited) == "" {
		return "", tserr.Empty("commit message after edit")
	}
	// Return the edited message and nil, to indicate success.
	return edited, nil
}
