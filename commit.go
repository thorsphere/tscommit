// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context" // Import the context package for context handling.
	"fmt"     // Import the fmt package for printing messages.

	"github.com/thorsphere/tserr" // Import the tserr package for error reporting.
)

// Run executes the tscommit workflow with a background context.
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
	diff, err := stagedDiff(ctx, maxDiffChars)
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
		case choiceRegenerate:
			// Generate a fresh message for the same diff and session.
			msg, err = genMessage(ctx, client, diff, sessionID)
			// Return the error if message generation fails.
			if err != nil {
				return abortOr(ctx, err)
			}
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
