// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"context" // Import the context package for cancellation-aware testing.
	"errors"  // Import the errors package for sentinel error creation.
	"fmt"     // Import the fmt package for formatted output.
	"io"      // Import the io package for input/output operations.
	"os"      // Import the os package for file and environment operations.
	"slices"  // Import the slices package for slice comparison.
	"strings" // Import the strings package for string containment checks.
	"testing" // Import the testing package for unit tests.

	"github.com/thorsphere/lpcli"      // Import the lpcli package for command-line interface testing.
	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client testing.
	"github.com/thorsphere/tscommit"   // Import the tscommit package under test.
	"github.com/thorsphere/tserr"      // Import the tserr package for error reporting.
)

// TestAbortOr tests that abortOr normalizes user aborts to tserr.Aborted and
// passes genuine failures through unchanged.
func TestAbortOr(t *testing.T) {
	// Create a cancelled context to simulate a user abort.
	cancelled, cancel := context.WithCancel(context.Background())
	// Cancel the context immediately.
	cancel()
	// The failure the caller observed.
	sentinel := errors.New("boom")
	// Test cases.
	cases := []struct {
		name string
		ctx  context.Context // The context for the run.
		err  error           // The error to normalize.
		want string          // Text the resulting error must contain.
	}{
		{"cancelled context maps to aborted", cancelled, sentinel, "commit aborted by user"},
		{"live context passes error through", context.Background(), sentinel, "boom"},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test.
			got := tscommit.AbortOr(tc.ctx, tc.err)
			// Compare the resulting error message.
			if !strings.Contains(got.Error(), tc.want) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "abortOr", Actual: got.Error(), Want: tc.want}))
			}
		})
	}
}

// fakeRunGit implements the git execution seam for whole-run tests: it
// records every invocation, serves a canned staged diff for diff calls,
// and replays canned failures per subcommand.
type fakeRunGit struct {
	args      [][]string // The recorded git argument lists, in call order.
	diffOut   string     // The canned staged diff served to diff calls.
	diffErr   error      // The canned failure returned by diff calls.
	commitErr error      // The canned failure returned by commit calls.
}

// run records the arguments and dispatches on the git subcommand.
func (f *fakeRunGit) run(_ context.Context, stdout, _ io.Writer, args ...string) error {
	// Check that the argument list is not empty; git always has a subcommand.
	if len(args) == 0 {
		// Reject empty argument lists.
		return tserr.Empty("git arguments")
	}
	// Record the argument list for later assertions.
	f.args = append(f.args, append([]string(nil), args...))
	// Dispatch on the subcommand, carried by the first argument.
	switch args[0] {
	case "diff":
		// Serve the canned staged diff.
		if _, err := io.WriteString(stdout, f.diffOut); err != nil {
			// Return the write failure.
			return tserr.Op(&tserr.OpArgs{Op: "write diff", Fn: "fakeRunGit", Err: err})
		}
		// Return the canned diff failure, if any.
		return f.diffErr
	case "commit":
		// Return the canned commit failure, if any.
		return f.commitErr
	}
	// Reject unexpected subcommands so new call sites fail loudly.
	return tserr.MethodNotAllowed(&tserr.MethodNotAllowedArgs{Method: args[0], Resource: "fakeRunGit"})

}

// useStdio swaps the process standard streams for the duration of the
// test: os.Stdin replays input while os.Stdout and os.Stderr are captured
// to a temporary file, because newPrompter binds the process streams
// internally. It returns a function that reads the captured output. Tests
// using it must not call t.Parallel.
func useStdio(t *testing.T, input string) func() string {
	// Mark the test function as test helper function.
	t.Helper()
	// Create a temporary file carrying the canned prompter input.
	in, err := os.CreateTemp(t.TempDir(), "stdin")
	// If the input file cannot be created, fail the test.
	if err != nil {
		t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "CreateTemp", Err: err}))
		// Return a no-op capture function.
		return func() string { return "" }
	}
	// Write the canned input and rewind the file so it can be read again.
	if _, err := io.WriteString(in, input); err != nil {
		t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "WriteString", Err: err}))
	}
	// Rewind the input file to the start.
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "Seek", Err: err}))
	}
	// Create a temporary file capturing program and prompt output.
	out, err := os.CreateTemp(t.TempDir(), "out")
	// If the output file cannot be created, fail the test.
	if err != nil {
		t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "CreateTemp", Err: err}))
		// Return a no-op capture function.
		return func() string { return "" }
	}
	// Save the current process streams.
	prevIn, prevOut, prevErr := os.Stdin, os.Stdout, os.Stderr
	// Install the canned input and the capture file on the process streams.
	os.Stdin, os.Stdout, os.Stderr = in, out, out
	// Restore the process streams and close the files at test end.
	t.Cleanup(func() {
		// Restore the saved process streams.
		os.Stdin, os.Stdout, os.Stderr = prevIn, prevOut, prevErr
		// Close the temporary files; errors are irrelevant at cleanup.
		in.Close()
		out.Close()
	})
	// Return a function reading the captured output.
	return func() string {
		// Rewind the capture file to the start.
		if _, err := out.Seek(0, io.SeekStart); err != nil {
			t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "Seek", Err: err}))
			return ""
		}
		// Read the captured output.
		b, err := io.ReadAll(out)
		// If the capture cannot be read, fail the test.
		if err != nil {
			t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "ReadAll", Err: err}))
			return ""
		}
		// Return the captured output.
		return string(b)
	}
}

// sessionIDFrom extracts the session ID from a request rendering recorded
// by fakeChat (fmt.Sprintf("%+v", req)); the sessionID field is the last
// field before the closing brace of the struct.
func sessionIDFrom(t *testing.T, rendered string) string {
	// Mark the test function as test helper function.
	t.Helper()
	// Locate the sessionID field in the rendering.
	i := strings.LastIndex(rendered, "sessionID:")
	// If the field is missing, fail the test.
	if i < 0 {
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "request rendering", Actual: rendered, Want: "a rendering carrying a sessionID field"}))
		return ""
	}
	// Extract the field value up to the closing brace of the struct.
	value, _, _ := strings.Cut(rendered[i+len("sessionID:"):], "}")
	// Return the extracted session ID.
	return value
}

// wantGitCalls fails the test unless git was invoked exactly with want,
// one argument list per call, in order.
func wantGitCalls(t *testing.T, got, want [][]string) {
	// Mark the test function as test helper function.
	t.Helper()
	// If the call counts differ, fail the test.
	if len(got) != len(want) {
		t.Errorf("git calls = %v, want %v", got, want)
		return
	}
	// Compare every recorded call against the expectation.
	for i := range want {
		// If one call differs, fail the test.
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("git args = %v, want %v", got, want)
			return
		}
	}
}

// TestRunContext tests the full workflow end to end: client construction,
// staged diff reading, message generation, the interactive accept / edit /
// regenerate / cancel loop, and commit creation. The git, AI and editor
// seams are swapped and the process streams carry canned input and a
// capture file, so no external binary, network call, or terminal is
// required. It must not use t.Parallel.
func TestRunContext(t *testing.T) {
	// The API key installed for scenarios that must build a client.
	const testKey = "test-key-42"
	// The canned staged diff served to the workflow.
	const diff = "diff --git a/login.go b/login.go\n"
	// The message the fake provider produces on its first turn.
	const firstMsg = "feat: add login"
	// The message the fake editor produces after the user edits.
	const editedMsg = "feat: add login form"
	// The message the fake provider produces on regeneration.
	const secondMsg = "feat: add the login form"
	// The message carrying a breaking-change marker.
	const breakingMsg = "feat!: drop the v1 api"
	// The sentinel failure of the provider.
	genErr := errors.New("provider boom")
	// Test cases.
	cases := []struct {
		name         string     // The scenario name.
		key          string     // The OPENROUTER_API_KEY value installed for the run.
		input        string     // The canned prompter input (keystrokes, newline-terminated).
		diffOut      string     // The canned staged diff served by git.
		diffErr      error      // The canned staged diff failure.
		chat         []chatTurn // The scripted provider turns.
		editOut      string     // The content the fake editor returns.
		editErr      error      // The error the fake editor returns.
		commitErr    error      // The canned commit failure.
		cancelled    bool       // Whether the context is cancelled before the run.
		wantErr      bool       // Whether an error is expected.
		wantErrText  string     // Text the error must contain.
		wantGit      [][]string // The expected git argument lists, in call order.
		wantCalls    int        // The expected number of provider calls.
		wantEditText string     // The text the editor must receive; empty when the editor must not run.
		wantSticky   bool       // Whether all provider calls must share one session ID.
		wantWarn     bool       // Whether the breaking-change warning must be printed.
	}{
		{
			name:      "accept commits the generated message",
			key:       testKey,
			input:     "y\n",
			diffOut:   diff,
			chat:      []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			wantGit:   [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", firstMsg}},
			wantCalls: 1,
		},
		{
			name:      "enter accepts the default choice",
			key:       testKey,
			input:     "\n",
			diffOut:   diff,
			chat:      []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			wantGit:   [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", firstMsg}},
			wantCalls: 1,
		},
		{
			name:        "cancel aborts without committing",
			key:         testKey,
			input:       "n\n",
			diffOut:     diff,
			chat:        []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			wantErr:     true,
			wantErrText: "commit aborted by user",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   1,
		},
		{
			name:         "edit then accept commits the edited message",
			key:          testKey,
			input:        "e\ny\n",
			diffOut:      diff,
			chat:         []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			editOut:      editedMsg,
			wantEditText: firstMsg,
			wantGit:      [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", editedMsg}},
			wantCalls:    1,
		},
		{
			name:       "regenerate then accept commits the fresh message",
			key:        testKey,
			input:      "r\ny\n",
			diffOut:    diff,
			chat:       []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}, {res: tsaiclient.Result{Content: secondMsg}}},
			wantGit:    [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", secondMsg}},
			wantCalls:  2,
			wantSticky: true,
		},
		{
			name:      "breaking change warns before accept",
			key:       testKey,
			input:     "y\n",
			diffOut:   diff,
			chat:      []chatTurn{{res: tsaiclient.Result{Content: breakingMsg}}},
			wantGit:   [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", breakingMsg}},
			wantCalls: 1,
			wantWarn:  true,
		},
		{
			name:         "editor failure aborts the run",
			key:          testKey,
			input:        "e\n",
			diffOut:      diff,
			chat:         []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			editErr:      errFake, // The editor failure defined in prompt_user_test.go.
			wantEditText: firstMsg,
			wantErr:      true,
			wantErrText:  "editor failed",
			wantGit:      [][]string{{"diff", "--staged", "-W"}},
			wantCalls:    1,
		},
		{
			name:         "empty edit aborts the run",
			key:          testKey,
			input:        "e\n",
			diffOut:      diff,
			chat:         []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			editOut:      " \t\n",
			wantEditText: firstMsg,
			wantErr:      true,
			wantErrText:  "commit message after edit",
			wantGit:      [][]string{{"diff", "--staged", "-W"}},
			wantCalls:    1,
		},
		{
			name:        "generation failure surfaces before prompting",
			key:         testKey,
			diffOut:     diff,
			chat:        []chatTurn{{err: genErr}},
			wantErr:     true,
			wantErrText: "provider boom",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   1,
		},
		{
			name:        "regeneration failure aborts the run",
			key:         testKey,
			input:       "r\n",
			diffOut:     diff,
			chat:        []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}, {err: genErr}},
			wantErr:     true,
			wantErrText: "provider boom",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   2,
		},
		{
			name:        "commit failure surfaces",
			key:         testKey,
			input:       "y\n",
			diffOut:     diff,
			chat:        []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			commitErr:   errors.New("exit status 1"),
			wantErr:     true,
			wantErrText: "commit git failed",
			wantGit:     [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", firstMsg}},
			wantCalls:   1,
		},
		{
			name:        "staged diff failure surfaces",
			key:         testKey,
			diffErr:     errors.New("exit status 128"),
			wantErr:     true,
			wantErrText: "diff git failed",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   0,
		},
		{
			name:        "nothing staged surfaces",
			key:         testKey,
			diffOut:     "", // An empty diff is rejected by stagedDiff.
			wantErr:     true,
			wantErrText: "git staged changes",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   0,
		},
		{
			name:        "missing API key fails before any work",
			key:         "",
			wantErr:     true,
			wantErrText: "new client",
			wantGit:     nil, // Git must not be invoked at all.
			wantCalls:   0,
		},
		{
			name:        "cancelled context aborts at the prompt",
			key:         testKey,
			input:       "y\n",
			diffOut:     diff,
			chat:        []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			cancelled:   true,
			wantErr:     true,
			wantErrText: "commit aborted by user",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   1,
		},
		{
			name:        "exhausted input aborts at the prompt",
			key:         testKey,
			input:       "",
			diffOut:     diff,
			chat:        []chatTurn{{res: tsaiclient.Result{Content: firstMsg}}},
			wantErr:     true,
			wantErrText: "tscommit aborted by user", // The raw prompter abort passes through abortOr.
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
			wantCalls:   1,
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario API key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Create the fake git with the scenario's diff and failures.
			git := &fakeRunGit{diffOut: tc.diffOut, diffErr: tc.diffErr, commitErr: tc.commitErr}
			// Swap the git seam for the duration of the test.
			t.Cleanup(tscommit.SetExecGit(git.run))
			// Create the fake provider with the scripted turns.
			fake := &fakeChat{script: tc.chat}
			// Swap the AI seam for the duration of the test.
			useFakeChat(t, fake)
			// Track the editor invocations and the text the editor receives.
			editorText, editorCalls := "", 0
			// Swap the editor seam for the duration of the test.
			t.Cleanup(tscommit.SetEditMessage(func(_ context.Context, _ *lpcli.Prompter, initialText string) (string, error) {
				// Record the editor invocation and the received text.
				editorCalls++
				editorText = initialText
				// Return the scenario's canned editor result.
				return tc.editOut, tc.editErr
			}))
			// Replace the process streams with canned input and a capture file.
			captured := useStdio(t, tc.input)
			// Start with a background context.
			ctx := context.Background()
			// Cancel the context up front when the scenario requires it.
			if tc.cancelled {
				// Create a cancelled context to simulate a user abort.
				cancelled, cancel := context.WithCancel(ctx)
				// Cancel the context immediately.
				cancel()
				// Use the cancelled context for the run.
				ctx = cancelled
			}
			// Run the workflow under test.
			err := tscommit.RunContext(ctx)
			// Check the error expectation.
			if tc.wantErr {
				// An error is expected: fail if it is missing.
				if err == nil {
					t.Error(tserr.NilFailed("RunContext"))
				} else if tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
					// The diagnostic text must be present in the error.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "RunContext error text", Actual: err.Error(), Want: tc.wantErrText}))
				}
			} else if err != nil {
				// No error is expected: fail on any error.
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "RunContext", Err: err}))
			}
			// Check the git invocation sequence.
			wantGitCalls(t, git.args, tc.wantGit)
			// Check the provider call count.
			if fake.calls != tc.wantCalls {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "completeChat calls", Actual: fmt.Sprintf("%v", fake.calls), Want: fmt.Sprintf("%v", tc.wantCalls)}))
			}
			// Check the editor contract.
			if tc.wantEditText != "" {
				// The editor must have run exactly once.
				if editorCalls != 1 {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editMessage calls", Actual: fmt.Sprintf("%v", editorCalls), Want: "1"}))
				}
				// The editor must receive the pending message.
				if editorText != tc.wantEditText {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editMessage initialText", Actual: editorText, Want: tc.wantEditText}))
				}
			} else if editorCalls != 0 {
				// The editor must not run when no edit is expected.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editMessage calls", Actual: fmt.Sprintf("%v", editorCalls), Want: "0"}))
			}
			// Check that regenerations reuse one sticky routing key.
			if tc.wantSticky {
				// Extract the session ID of the first recorded request.
				first := sessionIDFrom(t, fake.reqs[0])
				// Every request must carry the same session ID.
				for _, req := range fake.reqs[1:] {
					// If a request carries a different session ID, fail the test.
					if id := sessionIDFrom(t, req); id != first {
						t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "session ID", Actual: id, Want: first}))
					}
				}
			}
			// Check the breaking-change warning against the captured output.
			if warn := strings.Contains(captured(), tscommit.BreakingWarning); warn != tc.wantWarn {
				t.Errorf("breaking-change warning present = %v, want %v", warn, tc.wantWarn)
			}
		})
	}
}

// TestRun tests that Run executes the workflow with a background context and
// propagates the outcome unchanged: an accepted message commits, and a
// failure surfaces as an error. The workflow internals are covered by
// TestRunContext; this pins the delegation contract. It must not use
// t.Parallel.
func TestRun(t *testing.T) {
	// The canned staged diff served to the workflow.
	const diff = "diff --git a/login.go b/login.go\n"
	// The message the fake provider produces.
	const msg = "feat: add login"
	// Test cases.
	cases := []struct {
		name        string     // The scenario name.
		key         string     // The OPENROUTER_API_KEY value installed for the run.
		input       string     // The canned prompter input (keystrokes, newline-terminated).
		diffOut     string     // The canned staged diff served by git.
		chat        []chatTurn // The scripted provider turns.
		wantErr     bool       // Whether an error is expected.
		wantErrText string     // Text the error must contain.
		wantGit     [][]string // The expected git argument lists, in call order.
	}{
		{
			name:    "accept commits the generated message",
			key:     "sk-or-v1-test-key",
			input:   "y\n",
			diffOut: diff,
			chat:    []chatTurn{{res: tsaiclient.Result{Content: msg}}},
			wantGit: [][]string{{"diff", "--staged", "-W"}, {"commit", "-m", msg}},
		},
		{
			name:        "missing API key fails before any work",
			key:         "",
			wantErr:     true,
			wantErrText: "new client",
			wantGit:     nil, // Git must not be invoked at all.
		},
		{
			name:        "cancel aborts without committing",
			key:         "sk-or-v1-test-key",
			input:       "n\n",
			diffOut:     diff,
			chat:        []chatTurn{{res: tsaiclient.Result{Content: msg}}},
			wantErr:     true,
			wantErrText: "commit aborted by user",
			wantGit:     [][]string{{"diff", "--staged", "-W"}},
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario API key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Create the fake git with the scenario's diff.
			git := &fakeRunGit{diffOut: tc.diffOut}
			// Swap the git seam for the duration of the test.
			t.Cleanup(tscommit.SetExecGit(git.run))
			// Create the fake provider with the scripted turns.
			fake := &fakeChat{script: tc.chat}
			// Swap the AI seam for the duration of the test.
			useFakeChat(t, fake)
			// Replace the process streams with canned input and a capture file.
			useStdio(t, tc.input)
			// Run the workflow under test.
			err := tscommit.Run()
			// Check the error expectation.
			if tc.wantErr {
				// An error is expected: fail if it is missing.
				if err == nil {
					t.Error(tserr.NilFailed("Run"))
				} else if tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
					// The diagnostic text must be present in the error.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Run error text", Actual: err.Error(), Want: tc.wantErrText}))
				}
			} else if err != nil {
				// No error is expected: fail on any error.
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "Run", Err: err}))
			}
			// Check the git invocation sequence.
			wantGitCalls(t, git.args, tc.wantGit)
		})
	}
}
