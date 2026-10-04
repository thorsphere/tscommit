// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context" // Import the context package for context handling.
	"os"      // Import the os package for file and directory operations.
	"os/exec" // Import the exec package for executing external commands.

	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

// StagedDiff returns the staged diff, truncated to maxLen bytes.
// Returns an empty string if nothing is staged.
func stagedDiff(ctx context.Context, maxLen int) (string, error) {
	// Retrieve the diff from git using the flags --staged and -W.
	outDiff, err := exec.CommandContext(ctx, "git", "diff", "--staged", "-W").Output()
	// If an error occurs, return an error.
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "diff", Fn: "git", Err: err})
	}
	// If the diff is empty, return an empty string and an error.
	if len(outDiff) == 0 {
		return "", tserr.Empty("git staged changes")
	}
	// Truncate the diff to maxLen bytes.
	diff := string(outDiff)
	if len(diff) > maxLen {
		diff = diff[:maxLen] + "\n... (truncated)"
	}
	// Return the diff and no error to indicate success.
	return diff, nil
}

func recentCommits(ctx context.Context, maxLen int) (string, error) {
	// Retrieve the recent commits using the flags -n 5 and -format.
	// -n 5 limits the number of commits to 5.
	// -format specifies the format of the output.
	outRecent, err := exec.CommandContext(ctx, "git", "log", "-n", "5", "--format=\"%s\"").Output()
	// If an error occurs, return an error.
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "log", Fn: "git", Err: err})
	}
	// Return the recent commits and no error to indicate success.
	recent := string(outRecent)
	return recent, nil
}

// Commit creates a git commit with the given message.
func commit(ctx context.Context, msg string) error {
	// Create a new git commit with the given message.
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", msg)
	// Redirect the command's output to os.Stdout.
	cmd.Stdout = os.Stdout
	// Redirect the command's error output to os.Stderr.
	cmd.Stderr = os.Stderr
	// Run the command.
	if err := cmd.Run(); err != nil {
		// If an error occurs, return an error.
		return tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: err})
	}
	// Return no error to indicate success.
	return nil
}
