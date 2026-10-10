// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context" // Import the context package for context handling.
	"io"      // Import the io package for I/O.
)

// Export functions for testing.
var (
	ValidateMessage   = validateMessage
	HasBreakingChange = hasBreakingChange
	CheckEdited       = checkEdited
	StagedDiff        = stagedDiff
	Commit            = commit
)

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
