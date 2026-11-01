// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"context" // Import the context package for context handling.
	"errors"  // Import the errors package for error handling.
	"fmt"     // Import the fmt package for formatting.
	"io"      // Import the io package for I/O.
	"slices"  // Import the slices package for slice equality.
	"strings" // Import the strings package for string handling.
	"testing" // Import the testing package for testing.

	"github.com/thorsphere/tscommit" // Import the tscommit package for testing.
	"github.com/thorsphere/tserr"    // Import the tserr package for error handling.
)

// fakeGit implements the git execution seam without a git binary.
type fakeGit struct {
	args   [][]string // recorded git argument lists
	out    string     // fake stdout output
	errOut string     // fake stderr output
	err    error      // fake command error
}

// run records the args and replays the canned stdout/stderr/error.
func (f *fakeGit) run(_ context.Context, stdout, stderr io.Writer, args ...string) error {
	// Record the args.
	f.args = append(f.args, append([]string(nil), args...))
	// If there is canned stdout, write it.
	if f.out != "" {
		// Write the canned stdout.
		if _, err := io.WriteString(stdout, f.out); err != nil {
			// Return the error, if any.
			return tserr.Op(&tserr.OpArgs{Op: "WriteString", Fn: "fakeGit", Err: err})
		}
	}
	// If there is canned stderr, write it.
	if f.errOut != "" {
		// Write the canned stderr.
		if _, err := io.WriteString(stderr, f.errOut); err != nil {
			// Return the error, if any.
			return tserr.Op(&tserr.OpArgs{Op: "WriteString", Fn: "fakeGit", Err: err})
		}
	}
	// Return the error, if any.
	return f.err
}

// useFakeGit swaps the git seam for the duration of the test.
// Tests using it must not call t.Parallel.
func useFakeGit(t *testing.T, f *fakeGit) {
	// Mark the test function as test helper function.
	t.Helper()
	// Swap the git seam for the duration of the test.
	// Register the returned function as a test cleanup function to
	// undo the swap.
	t.Cleanup(tscommit.SetExecGit(f.run))
}

// wantGitArgs fails the test unless git was invoked exactly once with want.
func wantGitArgs(t *testing.T, got [][]string, want []string) {
	// Mark the test function as test helper function.
	t.Helper()
	// If git was not invoked exactly once with the expected args, fail the test.
	if len(got) != 1 || !slices.Equal(got[0], want) {
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "git args", Actual: fmt.Sprintf("%v", got), Want: fmt.Sprintf("%v", want)}))
	}
}

// TestStagedDiff tests stagedDiff: pass-through, character truncation,
// maxLen validation, the empty-staged error, and command failure.
func TestStagedDiff(t *testing.T) {
	// Define a long string.
	long := strings.Repeat("x", 100)
	// Define the test cases.
	cases := []struct {
		name           string
		out            string
		errOut         string
		err            error
		maxLen         int
		want           string
		wantErr        bool
		wantErrText    string
		wantNoGitCalls bool // When set, git must not be invoked at all.
	}{
		{"passthrough", "diff --git a/x b/x\n", "", nil, tscommit.MaxDiffChars, "diff --git a/x b/x\n", false, "", false},
		{"exact fit", "12345", "", nil, 5, "12345", false, "", false},
		{"truncated", long, "", nil, 40, long[:40] + "\n... (truncated)", false, "", false},
		// Truncation counts characters: 3 runes fit exactly in 3.
		{"exact fit multibyte", "ééé", "", nil, 3, "ééé", false, "", false},
		// Truncation cuts at the character boundary, never mid-rune.
		{"truncated multibyte", "ééé", "", nil, 2, "éé" + "\n... (truncated)", false, "", false},
		// A negative maxLen is rejected before git runs.
		{"negative maxLen", "", "", nil, -1, "", true, "maxLen", true},
		// A zero maxLen truncates to just the marker; the diff is non-empty.
		{"zero maxLen", "x", "", nil, 0, "\n... (truncated)", false, "", false},
		{"empty staged", "", "", nil, tscommit.MaxDiffChars, "", true, "", false},
		{"command failure", "", "", errors.New("exit status 128"), tscommit.MaxDiffChars, "", true, "", false},
		// git's diagnostic text must survive into the wrapped error.
		{"failure with stderr", "", "fatal: not a git repository\n", errors.New("exit status 128"), tscommit.MaxDiffChars, "", true, "fatal: not a git repository", false},
	}
	// Run the tests.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake git.
			fake := &fakeGit{out: tc.out, errOut: tc.errOut, err: tc.err}
			// Swap the git seam for the duration of the test.
			useFakeGit(t, fake)
			// Run the staged diff.
			got, err := tscommit.StagedDiff(context.Background(), tc.maxLen)
			// if the error state doesn't match, the test fails
			if (err != nil) != tc.wantErr {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "StagedDiff error", Actual: fmt.Sprintf("%v", err != nil), Want: fmt.Sprintf("%v", tc.wantErr)}))
			}
			// if the diff differs from the expectation, the test fails
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "StagedDiff", Actual: got, Want: tc.want}))
			}
			// if the invocation expectation is wrong, the test fails
			if tc.wantNoGitCalls {
				// The guard must reject maxLen without invoking git.
				if len(fake.args) != 0 {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "git args", Actual: fmt.Sprintf("%v", fake.args), Want: "no calls"}))
				}
			} else {
				// if the git invocation is wrong, the test fails
				wantArgs := []string{"diff", "--staged", "-W"}
				wantGitArgs(t, fake.args, wantArgs)
			}
			// if the diagnostic text is missing from the error, the test fails
			if tc.wantErrText != "" && err != nil && !strings.Contains(err.Error(), tc.wantErrText) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "StagedDiff error text", Actual: err.Error(), Want: tc.wantErrText}))
			}
		})
	}
}

// TestCommit tests commit: correct git invocation and error wrapping.
func TestCommit(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name    string
		msg     string
		err     error
		wantErr bool
	}{
		{"success", "feat: xyz", nil, false},
		{"nothing to commit", "feat: xyz", errors.New("exit status 1"), true},
	}
	// Run the tests.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake git.
			fake := &fakeGit{err: tc.err}
			// Swap the git seam for the duration of the test.
			useFakeGit(t, fake)
			// Run the commit.
			err := tscommit.Commit(context.Background(), tc.msg)
			// if the error state doesn't match, the test fails
			if (err != nil) != tc.wantErr {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Commit error", Actual: fmt.Sprintf("%v", err != nil), Want: fmt.Sprintf("%v", tc.wantErr)}))
			}
			// if the git invocation is wrong, the test fails
			wantArgs := []string{"commit", "-m", tc.msg}
			wantGitArgs(t, fake.args, wantArgs)
		})
	}
}
