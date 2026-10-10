// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"bytes"   // Import the bytes package for byte slices.
	"context" // Import the context package for context handling.
	"fmt"     // Import the fmt package for formatting.
	"io"      // Import the io package for I/O.
	"os"      // Import the os package for file and directory operations.
	"os/exec" // Import the exec package for executing external commands.
	"strings" // Import the strings package for string handling.

	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

// gitExec runs a git command with the given arguments, streaming stdout and
// stderr to the provided writers. It is a seam so unit tests can run without
// the git binary installed (swapped via SetExecGit in export_test.go).
type gitExec func(ctx context.Context, stdout, stderr io.Writer, args ...string) error

// execGit executes a real git command via os/exec.
var execGit gitExec = func(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	// Create the git command bound to the context.
	cmd := exec.CommandContext(ctx, "git", args...)
	// Redirect the command's output to the given writers.
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Run the command.
	return cmd.Run()
}

// runGit runs git, streaming stdout and stderr to the given writers while
// capturing stderr so failures can carry git's diagnostic text.
func runGit(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	// Capture stderr alongside the caller's writer.
	var errBuf bytes.Buffer
	// Run git and enrich any error with the captured diagnostic.
	if err := execGit(ctx, stdout, io.MultiWriter(stderr, &errBuf), args...); err != nil {
		return gitErr(err, errBuf.String())
	}
	// Return no error to indicate success.
	return nil
}

// gitErr enriches a git command error with the command's stderr output, which
// carries git's diagnostic text (e.g. "not a git repository"). The original
// error is wrapped, so errors.Is and errors.As still reach it.
func gitErr(err error, stderr string) error {
	// If git wrote no diagnostic, return the error unchanged.
	if stderr == "" {
		return err
	}
	// Otherwise, append the diagnostic text to the error.
	return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
}

// StagedDiff returns the staged diff, truncated to maxLen bytes.
// Returns an error if nothing is staged.
func stagedDiff(ctx context.Context, maxLen int) (string, error) {
	// Capture the diff from git using the flags --staged and -W. stderr is
	// discarded: runGit attaches any diagnostic text to the returned error.
	var outDiff bytes.Buffer
	if err := runGit(ctx, &outDiff, io.Discard, "diff", "--staged", "-W"); err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "diff", Fn: "git", Err: err})
	}
	// If the diff is empty, return an empty string and an error.
	if outDiff.Len() == 0 {
		return "", tserr.Empty("git staged changes")
	}
	// Truncate the diff to maxLen bytes.
	diff := outDiff.String()
	if len(diff) > maxLen {
		diff = diff[:maxLen] + "\n... (truncated)"
	}
	// Return the diff and no error to indicate success.
	return diff, nil
}

// Commit creates a git commit with the given message.
func commit(ctx context.Context, msg string) error {
	// Create a new git commit with the given message, streaming output to
	// os.Stdout and os.Stderr.
	if err := runGit(ctx, os.Stdout, os.Stderr, "commit", "-m", msg); err != nil {
		// If an error occurs, return an error.
		return tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: err})
	}
	// Return no error to indicate success.
	return nil
}
