// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package tscommit

import (
	"context"

	"github.com/thorsphere/tserr"
)

// Run executes the tscommit workflow using context.Background().
func Run() error {
	// Delegate to RunContext with a background context.
	return RunContext(context.Background())
}

// RunContext executes the tscommit workflow with the provided context:
// reads the staged diff, generates a commit message via the configured AI
// provider, and creates the commit. The provider API key is read from the
// environment by the AI client.
func RunContext(ctx context.Context) error {
	// Build the AI client for this run; it validates the provider and API key.
	client, err := newClient()
	// Return the error if the AI client cannot be built.
	if err != nil {
		return err
	}
	// Create the interactive prompter for the run.
	p := newPrompter()
	// Read the staged git diff.
	diff, err := stagedDiff(ctx, 30000)
	// Return the error if reading the staged diff fails.
	if err != nil {
		return err
	}
	// Generate one sticky routing key for the whole run so retries and
	// regenerations land on the same provider endpoint and can reuse the
	// prompt cache.
	sessionID := newSessionID()
	// Generate the initial commit message.
	msg, err := genMessage(ctx, client, diff, sessionID)
	// Return the error if message generation fails.
	if err != nil {
		return err
	}
	// Prompt the user until they accept, edit, regenerate, or cancel.
	for {
		// Ask the user what to do with the current message.
		choice, err := promptUser(ctx, p, msg)
		// Return the error if prompting fails.
		if err != nil {
			return err
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
				return err
			}
			// Adopt the user's edit as the new message; it is authoritative.
			msg = edited
		case choiceRegenerate:
			// Generate a fresh message for the same diff and session.
			msg, err = genMessage(ctx, client, diff, sessionID)
			// Return the error if message generation fails.
			if err != nil {
				return err
			}
		case choiceCancel:
			// Return an aborted error when the user cancels.
			return tserr.Aborted("commit")
		}
	}
}
