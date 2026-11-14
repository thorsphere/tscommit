// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context" // Import the context package for context handling.
	// Import the io package for I/O.
	// Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
)

// Exports for the external test package: bridges to unexported functions.
var (
	ValidateMessage = validateMessage // Validates a generated commit message.
	BuildPrompt     = buildPrompt     // Assembles the AI prompt from the staged diff.
	PromptTemplate  = promptTemplate  // The instruction text prepended to the staged diff.
	CleanMessage    = cleanMessage    // Strips wrapper formatting from AI output.
	NewSessionID    = newSessionID    // Generates the sticky routing key for a session.
	GenMessage      = genMessage      // Generates a commit message via the AI client.
	NewClient       = newClient       // Creates the AI client for commit message generation.
)

// SetCompleteChat swaps the AI completion seam for tests and returns a restore
// function (pass it to t.Cleanup). Tests using it must not call t.Parallel.
func SetCompleteChat(fn func(ctx context.Context, client *tsaiclient.Client, req *tsaiclient.Request) (tsaiclient.JSONResult, error)) func() {
	// Save the current completion seam.
	prev := completeChat
	// Set the new completion seam.
	completeChat = fn
	// Return a function to restore the previous completion seam.
	return func() { completeChat = prev }
}
