// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

import (
	"context"
	"fmt"
	"strings"

	"github.com/thorsphere/lpcli"
	"github.com/thorsphere/tserr"
)

const (
	choiceAccept     string = "accept"
	choiceEdit       string = "edit"
	choiceRegenerate string = "regenerate"
	choiceCancel     string = "cancel"
)

var commitChoices = []lpcli.Choice{
	{Value: choiceAccept, Key: "y", Aliases: []string{"yes"}, Label: "commit", IsDefault: true},
	{Value: choiceEdit, Key: "e", Aliases: []string{"edit"}, Label: "edit"},
	{Value: choiceRegenerate, Key: "r", Aliases: []string{"regen", "regenerate"}, Label: "regenerate"},
	{Value: choiceCancel, Key: "n", Aliases: []string{"no", "c", "cancel", "q", "quit"}, Label: "cancel"},
}

// newPrompter creates the shared lpcli prompter for the tscommit CLI.
func newPrompter() *lpcli.Prompter {
	return lpcli.NewPrompter("tscommit")
}

// warnBreakingChange prints a prominent warning when msg contains a
// Conventional Commits breaking-change marker.
func warnBreakingChange(p *lpcli.Prompter, msg string) {
	if hasBreakingChange(msg) {
		fmt.Fprintln(p.Out(), "\n⚠ BREAKING CHANGE detected — this commit will trigger a major version bump.")
	}
}

// promptUser asks the user whether to accept, edit, regenerate, or cancel.
// Returns the selected choice value, or an error if ctx is cancelled.
func promptUser(ctx context.Context, p *lpcli.Prompter, msg string) (string, error) {
	warnBreakingChange(p, msg)

	choice, err := p.Prompt(ctx, lpcli.SelectOptions{
		Message: "Apply this commit? [Y/e/r/n] (Y=commit, e=edit, r=regenerate, n=cancel): ",
		Choices: commitChoices,
	})
	if err != nil {
		return choiceCancel, err
	}
	return choice, nil
}

// editInEditor opens the message in the user's editor and returns the edited content.
func editInEditor(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
	edited, err := p.Edit(ctx, initialText)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(edited) == "" {
		return "", tserr.Empty("commit message after edit")
	}
	return edited, nil
}
