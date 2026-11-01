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

// maxDiffChars is the maximum staged diff length in characters (runes)
// sent to the AI provider.
const maxDiffChars = 30000 // The staged diff size limit.

// execGit executes a real git command via os/exec. It is the one piece of
// git.go that unit tests never run: tests must not depend on the git
// binary, so they swap this seam via SetExecGit.
var execGit gitExec = func(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	// Create the git command bound to the context.
	cmd := exec.CommandContext(ctx, "git", args...)
	// Redirect the command's output to the given writers.
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Run the command and return any error.
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

// stagedDiff returns the staged diff, truncated to the first maxLen
// characters when it is longer, with a truncation marker appended.
// Truncation counts characters (runes), matching the display-length
// convention used by validateMessage. Returns an error if nothing is
// staged or if maxLen is negative.
func stagedDiff(ctx context.Context, maxLen int) (string, error) {
	// If maxLen is negative, return an error.
	if maxLen < 0 {
		return "", tserr.Higher(&tserr.HigherArgs{Var: "maxLen", Actual: int64(maxLen), LowerBound: 0})
	}
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
	// Truncate the diff to maxLen characters, counting runes rather than
	// bytes so multi-byte text is measured as a reader counts it and the
	// cut never splits a character.
	diff := outDiff.String()
	// If the diff exceeds maxLen characters, cut at the character boundary
	// and append the truncation marker.
	if r := []rune(diff); len(r) > maxLen {
		diff = string(r[:maxLen]) + "\n... (truncated)"
	}
	// Return the diff and no error to indicate success.
	return diff, nil
}

// commit creates a git commit with the message msg.
func commit(ctx context.Context, msg string) error {
	// Create a new git commit with the given message msg, streaming output to
	// os.Stdout and os.Stderr.
	if err := runGit(ctx, os.Stdout, os.Stderr, "commit", "-m", msg); err != nil {
		// If an error occurs, return an error.
		return tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: err})
	}
	// Return no error to indicate success.
	return nil
}
