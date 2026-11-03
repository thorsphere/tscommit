// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context" // Import the context package for context handling.
	"io"      // Import the io package for I/O.

	"github.com/thorsphere/lpcli"      // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
)

// Exports for the external test package: bridges to unexported functions.
var (
	ValidateMessage    = validateMessage    // Validates a generated commit message.
	HasBreakingChange  = hasBreakingChange  // Reports breaking-change markers.
	CheckEdited        = checkEdited        // Validates a user-edited message.
	EditInEditor       = editInEditor       // Opens the message in the editor and validates the result.
	StagedDiff         = stagedDiff         // Reads the staged git diff.
	Commit             = commit             // Creates the git commit.
	WarnBreakingChange = warnBreakingChange // Prints the breaking-change warning when warranted.
	BreakingWarning    = breakingWarning    // The warning printed when a breaking change is detected.
	NewPrompter        = newPrompter        // Creates the CLI prompter.
	PromptUser         = promptUser         // Asks the user what to do with the message.
	BuildPrompt        = buildPrompt        // Assembles the AI prompt from the staged diff.
	PromptTemplate     = promptTemplate     // The instruction text prepended to the staged diff.
	MaxDiffChars       = maxDiffChars       // The maximum staged diff length in characters (runes) sent to the AI provider.
	CleanMessage       = cleanMessage       // Strips wrapper formatting from AI output.
	NewSessionID       = newSessionID       // Generates the sticky routing key for a session.
	GenMessage         = genMessage         // Generates a commit message via the AI client.
	NewClient          = newClient          // Creates the AI client for commit message generation.
)

// The choice values promptUser returns, bridged for the external test package.
const (
	ChoiceAccept     = choiceAccept     // The user chose to commit.
	ChoiceEdit       = choiceEdit       // The user chose to edit the message.
	ChoiceRegenerate = choiceRegenerate // The user chose to regenerate the message.
	ChoiceCancel     = choiceCancel     // The user chose to cancel the commit.
)

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

// SetCompleteChat swaps the AI completion seam for tests and returns a restore
// function (pass it to t.Cleanup). Tests using it must not call t.Parallel.
func SetCompleteChat(fn func(ctx context.Context, client *tsaiclient.Client, req *tsaiclient.Request) (tsaiclient.Result, error)) func() {
	// Save the current completion seam.
	prev := completeChat
	// Set the new completion seam.
	completeChat = fn
	// Return a function to restore the previous completion seam.
	return func() { completeChat = prev }
}

// SetExecGit swaps the git execution seam for tests and returns a restore
// function (pass it to t.Cleanup).
func SetExecGit(fn func(ctx context.Context, stdout, stderr io.Writer, args ...string) error) func() {
	// Save the current git execution seam.
	prev := execGit
	// Set the new git execution seam.
	execGit = fn
	// Return a function to restore the previous git execution seam.
	return func() { execGit = prev }
}
