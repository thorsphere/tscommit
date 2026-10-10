// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages.
import (
	"context" // context
	"fmt"     // fmt
	"strings" // strings

	"github.com/thorsphere/lpcli" // lpcli
	"github.com/thorsphere/tserr" // tserr
)

// The choices the user can select when prompting for commit action.
const (
	choiceAccept     string = "accept"     // The default choice is to commit.
	choiceEdit       string = "edit"       // The user can also edit the message.
	choiceRegenerate string = "regenerate" // The user can also regenerate the message.
	choiceCancel     string = "cancel"     // The user can also cancel the commit.
)

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
		fmt.Fprintln(p.Out(), "\n⚠ BREAKING CHANGE detected — this commit will trigger a major version bump.")
	}
}

// promptUser asks the user whether to accept, edit, regenerate, or cancel.
// Returns the selected choice value, or an error if ctx is cancelled.
func promptUser(ctx context.Context, p *lpcli.Prompter, msg string) (string, error) {
	// Warn the user if the message contains a breaking change marker.
	warnBreakingChange(p, msg)
	// Ask the user what to do. If the user cancels, return an error. Otherwise,
	// return the selected choice value.
	choice, err := p.Prompt(ctx, lpcli.SelectOptions{
		Message: "Apply this commit? [Y/e/r/n] (Y=commit, e=edit, r=regenerate, n=cancel): ",
		Choices: commitChoices,
	})
	// If the user cancels, return an error.
	if err != nil {
		return choiceCancel, err
	}
	// Return the selected choice value and nil, to indicate success.
	return choice, nil
}

// editInEditor opens the message in the user's editor and returns the edited content.
// Returns an error if the user cancels or the editor is empty.
func editInEditor(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
	// Open the editor and wait for the user to edit the message.
	edited, err := p.Edit(ctx, initialText)
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
	if strings.TrimSpace(edited) == "" {
		return "", tserr.Empty("commit message after edit")
	}
	return edited, nil
}
