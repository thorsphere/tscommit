// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package cli

// Import packages
import (
	"context" // Import the context package for context handling.

	"github.com/thorsphere/lpcli"    // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tscommit" // Import the tscommit package for the generation seam types.
)

// The choice values promptUser returns, bridged for the external test package.
const (
	ChoiceAccept     = choiceAccept     // The user chose to commit.
	ChoiceEdit       = choiceEdit       // The user chose to edit the message.
	ChoiceRegenerate = choiceRegenerate // The user chose to regenerate the message.
	ChoiceCancel     = choiceCancel     // The user chose to cancel the commit.
)

var (
	NewPrompter        = newPrompter        // NewPrompter is the exported version of the internal newPrompter function.
	AbortOr            = abortOr            // AbortOr is the exported version of the internal abortOr function.
	PromptUser         = promptUser         // PromptUser is the exported version of the internal promptUser function.
	BreakingWarning    = breakingWarning    // BreakingWarning is the exported version of the internal breakingWarning variable.
	WarnBreakingChange = warnBreakingChange // WarnBreakingChange is the exported version of the internal warnBreakingChange function.
	CheckEdited        = checkEdited        // CheckEdited is the exported version of the internal checkEdited function.
	EditInEditor       = editInEditor       // EditInEditor is the exported version of the internal editInEditor function.
	HasBreakingChange  = hasBreakingChange  // HasBreakingChange is the exported version of the internal hasBreakingChange function.
)

// MaxDiffChars is the exported version of the internal maxDiffChars constant.
const MaxDiffChars = maxDiffChars

// SetEditMessage swaps the editor seam for tests and returns a restore
// function (pass it to t.Cleanup). Tests using it must not call t.Parallel.
func SetEditMessage(fn func(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error)) func() {
	// Save the current editor seam.
	prev := editMessage
	// Set the new editor seam.
	editMessage = fn
	// Return a function to restore the previous editor seam.
	return func() { editMessage = prev }
}

// SetNewPrompter swaps the prompter construction seam for tests and returns a
// restore function (pass it to t.Cleanup). Tests using it must not call
// t.Parallel.
func SetNewPrompter(fn func() *lpcli.Prompter) func() {
	// Save the current prompter construction seam.
	prev := newPrompter
	// Set the new prompter construction seam.
	newPrompter = fn
	// Return a function to restore the previous prompter construction seam.
	return func() { newPrompter = prev }
}

// SetSelectChoice swaps the prompt selection seam for tests and returns a
// restore function (pass it to t.Cleanup). Tests using it must not call
// t.Parallel.
func SetSelectChoice(fn func(ctx context.Context, p *lpcli.Prompter, opts lpcli.SelectOptions) (string, error)) func() {
	// Save the current selection seam.
	prev := selectChoice
	// Set the new selection seam.
	selectChoice = fn
	// Return a function to restore the previous selection seam.
	return func() { selectChoice = prev }
}

// SetStagedDiff swaps the git diff seam for tests and returns a restore
// function (pass it to t.Cleanup). Tests using it must not call t.Parallel.
func SetStagedDiff(fn func(ctx context.Context, maxLen int) (string, error)) func() {
	// Save the current git diff seam.
	prev := stagedDiff
	// Set the new git diff seam.
	stagedDiff = fn
	// Return a function to restore the previous git diff seam.
	return func() { stagedDiff = prev }
}

// SetCommit swaps the git commit seam for tests and returns a restore
// function (pass it to t.Cleanup). Tests using it must not call t.Parallel.
func SetCommit(fn func(ctx context.Context, msg string) error) func() {
	// Save the current git commit seam.
	prev := commit
	// Set the new git commit seam.
	commit = fn
	// Return a function to restore the previous git commit seam.
	return func() { commit = prev }
}

// SetGenerateMessage swaps the generation seam for tests and returns a
// restore function (pass it to t.Cleanup). Tests using it must not call
// t.Parallel.
func SetGenerateMessage(fn func(ctx context.Context, gen *tscommit.Generator, diff string) (tscommit.Message, error)) func() {
	// Save the current generation seam.
	prev := generateMessage
	// Set the new generation seam.
	generateMessage = fn
	// Return a function to restore the previous generation seam.
	return func() { generateMessage = prev }
}
