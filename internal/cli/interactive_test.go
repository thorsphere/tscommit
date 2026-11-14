// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package cli_test

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

	"github.com/thorsphere/lpcli"                 // Import the lpcli package for command-line interface testing.
	"github.com/thorsphere/tscommit"              // Import the tscommit package for message and generator types.
	"github.com/thorsphere/tscommit/internal/cli" // Import the cli package under test.
	"github.com/thorsphere/tserr"                 // Import the tserr package for error reporting.
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
			got := cli.AbortOr(tc.ctx, tc.err)
			// Compare the resulting error message.
			if !strings.Contains(got.Error(), tc.want) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "abortOr", Actual: got.Error(), Want: tc.want}))
			}
		})
	}
}

// fakeGit implements the git seams for whole-run tests: it records every
// invocation, serves a canned staged diff for diff calls, and replays canned
// failures per operation.
type fakeGit struct {
	maxLens   []int    // The recorded diff bounds, in call order.
	diffOut   string   // The canned staged diff served to diff calls.
	diffErr   error    // The canned failure returned by diff calls.
	commits   []string // The recorded commit messages, in call order.
	commitErr error    // The canned failure returned by commit calls.
}

// stagedDiff records the bound and serves the canned staged diff and failure.
func (f *fakeGit) stagedDiff(_ context.Context, maxLen int) (string, error) {
	// Record the diff bound for later assertions.
	f.maxLens = append(f.maxLens, maxLen)
	// Return the canned staged diff and failure.
	return f.diffOut, f.diffErr
}

// commit records the message and returns the canned commit failure.
func (f *fakeGit) commit(_ context.Context, msg string) error {
	// Record the commit message for later assertions.
	f.commits = append(f.commits, msg)
	// Return the canned commit failure, if any.
	return f.commitErr
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

// genTurn is one scripted generation outcome: a canned message and error.
type genTurn struct {
	msg tscommit.Message // The canned message for this call.
	err error            // The canned error for this call.
}

// okGen returns a scripted genTurn carrying text and confidence.
func okGen(text string, confidence float64) genTurn {
	// Wrap the text and confidence in a canned message.
	return genTurn{msg: tscommit.Message{Text: text, Confidence: confidence}}
}

// fakeGenerate implements the generateMessage seam without a provider. It
// records every diff and replays scripted turns in order.
type fakeGenerate struct {
	calls  int       // The number of times the seam was invoked.
	diffs  []string  // The recorded diffs, one per call.
	script []genTurn // The canned turns replayed in order.
}

// run records the diff and replays the next scripted turn.
func (f *fakeGenerate) run(_ context.Context, _ *tscommit.Generator, diff string) (tscommit.Message, error) {
	// Record the diff so the generation input can be asserted.
	f.diffs = append(f.diffs, diff)
	// Count the invocation.
	f.calls++
	// Return an error when the script is exhausted; the call-count
	// assertion reports the over-invocation.
	if f.calls > len(f.script) {
		// Return an error for the unexpected extra call.
		return tscommit.Message{}, errors.New("fakeGenerate: script exhausted")
	}
	// Replay the next scripted turn.
	turn := f.script[f.calls-1]
	// Return the canned message and error.
	return turn.msg, turn.err
}

// wantGitCalls fails the test unless the git seams were invoked as wanted:
// wantDiff staged diff reads bounded to cli.MaxDiffChars and wantCommits
// committed messages, in order.
func wantGitCalls(t *testing.T, f *fakeGit, wantDiff int, wantCommits []string) {
	// Mark the test function as test helper function.
	t.Helper()
	// If the staged diff read count differs, fail the test.
	if len(f.maxLens) != wantDiff {
		t.Errorf("stagedDiff calls = %v, want %v", len(f.maxLens), wantDiff)
	}
	// Every staged diff read must use the CLI diff bound.
	for _, maxLen := range f.maxLens {
		// If a read used a different bound, fail the test.
		if maxLen != cli.MaxDiffChars {
			t.Errorf("stagedDiff maxLen = %v, want %v", maxLen, cli.MaxDiffChars)
		}
	}
	// If the committed messages differ from the expectation, fail the test.
	if !slices.Equal(f.commits, wantCommits) {
		t.Errorf("commits = %v, want %v", f.commits, wantCommits)
	}
}

// TestRunContext tests the full workflow end to end: client construction,
// staged diff reading, message generation, the interactive accept / edit /
// regenerate / cancel loop, and commit creation. The git, generation and
// editor seams are swapped and the process streams carry canned input and a
// capture file, so no external binary, network call, or terminal is
// required. It must not use t.Parallel.
func TestRunContext(t *testing.T) {
	// The API key installed for scenarios that must build a client.
	const testKey = "test-key-42"
	// The canned staged diff served to the workflow.
	const diff = "diff --git a/login.go b/login.go\n"
	// The message the fake provider produces on its first turn.
	const firstMsg = "feat(login): add login"
	// The message the fake editor produces after the user edits.
	const editedMsg = "feat: add login form"
	// The message the fake provider produces on regeneration.
	const secondMsg = "feat(login): add the login form"
	// The message carrying a breaking-change marker.
	const breakingMsg = "feat(login)!: drop the v1 api"
	// The sentinel failure of the provider.
	genErr := errors.New("provider boom")
	// Test cases.
	cases := []struct {
		name         string    // The scenario name.
		key          string    // The OPENROUTER_API_KEY value installed for the run.
		input        string    // The canned prompter input (keystrokes, newline-terminated).
		diffOut      string    // The canned staged diff served by the git diff seam.
		diffErr      error     // The canned staged diff failure.
		script       []genTurn // The scripted generation turns.
		editOut      string    // The content the fake editor returns.
		editErr      error     // The error the fake editor returns.
		commitErr    error     // The canned commit failure.
		choices      []string  // Optional canned choice values returned by the selectChoice seam.
		cancelled    bool      // Whether the context is cancelled before the run.
		wantErr      bool      // Whether an error is expected.
		wantErrText  string    // Text the error must contain.
		wantDiff     int       // The expected number of staged diff reads.
		wantCommits  []string  // The expected committed messages, in order.
		wantCalls    int       // The expected number of generation calls.
		wantEditText string    // The text the editor must receive; empty when the editor must not run.
		wantWarn     bool      // Whether the breaking-change warning must be printed.
		wantConfCnt  int       // The expected number of "Confidence:" lines; -1 skips the check.
		wantOutText  string    // Optional text that must appear in the captured output.
	}{
		{
			name:        "accept commits the generated message",
			key:         testKey,
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			wantDiff:    1,
			wantCommits: []string{firstMsg},
			wantCalls:   1,
			wantConfCnt: 1,
			input:       "y\n",
		},
		{
			name:  "truncated staged diff is passed to generation unchanged",
			key:   testKey,
			input: "y\n",
			// The canned staged diff as git.StagedDiff returns it when oversized.
			diffOut:     strings.Repeat("d", cli.MaxDiffChars) + "\n... (truncated)",
			script:      []genTurn{okGen(firstMsg, 0.62)},
			wantDiff:    1,
			wantCommits: []string{firstMsg},
			wantCalls:   1,
			wantConfCnt: 1,
		},
		{
			name:        "enter accepts the default choice",
			key:         testKey,
			input:       "\n",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			wantDiff:    1,
			wantCommits: []string{firstMsg},
			wantCalls:   1,
			wantConfCnt: 1,
		},
		{
			name:        "cancel aborts without committing",
			key:         testKey,
			input:       "n\n",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			wantErr:     true,
			wantErrText: "commit aborted by user",
			wantDiff:    1,
			wantCalls:   1,
			wantConfCnt: 1,
		},
		{
			name:         "edit then accept commits the edited message",
			key:          testKey,
			input:        "e\ny\n",
			diffOut:      diff,
			script:       []genTurn{okGen(firstMsg, 0.91)},
			editOut:      editedMsg,
			wantEditText: firstMsg,
			wantDiff:     1,
			wantCommits:  []string{editedMsg},
			wantCalls:    1,
			wantConfCnt:  1, // Shown before edit, omitted after edit.
		},
		{
			name:        "regenerate then accept commits the fresh message",
			key:         testKey,
			input:       "r\ny\n",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.75), okGen(secondMsg, 0.94)},
			wantDiff:    1,
			wantCommits: []string{secondMsg},
			wantCalls:   2,
			wantConfCnt: 2, // Shown on both AI-generated turns.
		},
		{
			name:        "breaking change warns before accept",
			key:         testKey,
			input:       "y\n",
			diffOut:     diff,
			script:      []genTurn{okGen(breakingMsg, 0.88)},
			wantDiff:    1,
			wantCommits: []string{breakingMsg},
			wantCalls:   1,
			wantWarn:    true,
			wantConfCnt: 1,
		},
		{
			name:         "editor failure aborts the run",
			key:          testKey,
			input:        "e\n",
			diffOut:      diff,
			script:       []genTurn{okGen(firstMsg, 0.91)},
			editErr:      errFake, // The editor failure defined in prompt_user_test.go.
			wantEditText: firstMsg,
			wantErr:      true,
			wantErrText:  "editor failed",
			wantDiff:     1,
			wantCalls:    1,
			wantConfCnt:  1,
		},
		{
			name:         "empty edit aborts the run",
			key:          testKey,
			input:        "e\n",
			diffOut:      diff,
			script:       []genTurn{okGen(firstMsg, 0.91)},
			editOut:      " \t\n",
			wantEditText: firstMsg,
			wantErr:      true,
			wantErrText:  "commit message after edit",
			wantDiff:     1,
			wantCalls:    1,
			wantConfCnt:  1,
		},
		{
			name:        "generation failure surfaces before prompting",
			key:         testKey,
			diffOut:     diff,
			script:      []genTurn{{err: genErr}},
			wantErr:     true,
			wantErrText: "provider boom",
			wantDiff:    1,
			wantCalls:   1,
			wantConfCnt: 0,
		},
		{
			name:        "regeneration failure aborts the run",
			key:         testKey,
			input:       "r\n",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91), {err: genErr}},
			wantErr:     true,
			wantErrText: "provider boom",
			wantDiff:    1,
			wantCalls:   2,
			wantConfCnt: 1,
		},
		{
			name:        "commit failure surfaces",
			key:         testKey,
			input:       "y\n",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			commitErr:   tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: errors.New("exit status 1")}), // The wrapped commit failure as git.Commit returns it.
			wantErr:     true,
			wantErrText: "commit git failed",
			wantDiff:    1,
			wantCommits: []string{firstMsg},
			wantCalls:   1,
			wantConfCnt: 1,
		},
		{
			name:        "staged diff failure surfaces",
			key:         testKey,
			diffErr:     tserr.Op(&tserr.OpArgs{Op: "diff", Fn: "git", Err: errors.New("exit status 128")}), // The wrapped diff failure as git.StagedDiff returns it.
			wantErr:     true,
			wantErrText: "diff git failed",
			wantDiff:    1,
			wantCalls:   0,
			wantConfCnt: 0,
		},
		{
			name:        "nothing staged surfaces",
			key:         testKey,
			diffErr:     tserr.Empty("git staged changes"), // The empty-diff failure as git.StagedDiff returns it.
			wantErr:     true,
			wantErrText: "git staged changes",
			wantDiff:    1,
			wantCalls:   0,
			wantConfCnt: 0,
		},
		{
			name:        "missing API key fails before any work",
			key:         "",
			wantErr:     true,
			wantErrText: "new client",
			wantDiff:    0, // Git must not be invoked at all.
			wantCalls:   0,
			wantConfCnt: 0,
		},
		{
			name:        "cancelled context aborts at the prompt",
			key:         testKey,
			input:       "y\n",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			cancelled:   true,
			wantErr:     true,
			wantErrText: "commit aborted by user",
			wantDiff:    1,
			wantCalls:   1,
			wantConfCnt: 1,
		},
		{
			name:        "exhausted input aborts at the prompt",
			key:         testKey,
			input:       "",
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			wantErr:     true,
			wantErrText: "tscommit aborted by user", // The raw prompter abort passes through abortOr.
			wantDiff:    1,
			wantCalls:   1,
			wantConfCnt: 1,
		},
		{
			name:        "unhandled choice warns and re-prompts before accepting",
			key:         testKey,
			diffOut:     diff,
			script:      []genTurn{okGen(firstMsg, 0.91)},
			choices:     []string{"surprise", cli.ChoiceAccept},
			wantDiff:    1,
			wantCommits: []string{firstMsg},
			wantCalls:   1,
			wantConfCnt: 2,
			wantOutText: `Ignoring unhandled option "surprise"; please choose again.`,
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario API key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Create the fake git with the scenario's diff and failures.
			git := &fakeGit{diffOut: tc.diffOut, diffErr: tc.diffErr, commitErr: tc.commitErr}
			// Swap the git diff seam for the duration of the test.
			t.Cleanup(cli.SetStagedDiff(git.stagedDiff))
			// Swap the git commit seam for the duration of the test.
			t.Cleanup(cli.SetCommit(git.commit))
			// Create the fake generator with the scripted turns.
			fake := &fakeGenerate{script: tc.script}
			// Swap the generation seam for the duration of the test.
			t.Cleanup(cli.SetGenerateMessage(fake.run))
			// Swap the selection seam when the scenario specifies explicit choice values.
			if len(tc.choices) > 0 {
				// Track the current index in the scripted choice sequence.
				choiceIdx := 0
				// Swap selectChoice for the duration of the subtest.
				t.Cleanup(cli.SetSelectChoice(func(_ context.Context, _ *lpcli.Prompter, _ lpcli.SelectOptions) (string, error) {
					// Return an error if the scripted choices are exhausted.
					if choiceIdx >= len(tc.choices) {
						return "", errors.New("selectChoice script exhausted")
					}
					// Advance and return the next scripted choice.
					c := tc.choices[choiceIdx]
					// Increment the scripted choice index.
					choiceIdx++
					// Return the scripted choice and nil error.
					return c, nil
				}))
			}
			// Track the editor invocations and the text the editor receives.
			editorText, editorCalls := "", 0
			// Swap the editor seam for the duration of the test.
			t.Cleanup(cli.SetEditMessage(func(_ context.Context, _ *lpcli.Prompter, initialText string) (string, error) {
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
			err := cli.RunContext(ctx)
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
			// Check the git seam invocation contract.
			wantGitCalls(t, git, tc.wantDiff, tc.wantCommits)
			// Check the generation call count.
			if fake.calls != tc.wantCalls {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "generateMessage calls", Actual: fmt.Sprintf("%v", fake.calls), Want: fmt.Sprintf("%v", tc.wantCalls)}))
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
			// Read the captured output once for layout checks.
			out := captured()
			// Check the breaking-change warning against the captured output.
			if warn := strings.Contains(out, cli.BreakingWarning); warn != tc.wantWarn {
				t.Errorf("breaking-change warning present = %v, want %v", warn, tc.wantWarn)
			}
			// Check how many times the AI confidence line was displayed.
			if gotConfCnt := strings.Count(out, "Confidence:"); tc.wantConfCnt >= 0 && gotConfCnt != tc.wantConfCnt {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Confidence line count", Actual: fmt.Sprintf("%d", gotConfCnt), Want: fmt.Sprintf("%d", tc.wantConfCnt)}))
			}
			// Check for any required output substring.
			if tc.wantOutText != "" && !strings.Contains(out, tc.wantOutText) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "captured output", Actual: out, Want: tc.wantOutText}))
			}
			// Verify every generation call received the canned staged diff unchanged.
			for _, d := range fake.diffs {
				// If the generation input differs from the canned staged diff, fail the test.
				if d != tc.diffOut {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "generation input", Actual: fmt.Sprintf("%q", d), Want: fmt.Sprintf("%q", tc.diffOut)}))
				}
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
	// The message the fake generator produces.
	const msg = "feat(x): add login"
	// Test cases.
	cases := []struct {
		name        string    // The scenario name.
		key         string    // The OPENROUTER_API_KEY value installed for the run.
		input       string    // The canned prompter input (keystrokes, newline-terminated).
		diffOut     string    // The canned staged diff served by the git diff seam.
		script      []genTurn // The scripted generation turns.
		wantErr     bool      // Whether an error is expected.
		wantErrText string    // Text the error must contain.
		wantDiff    int       // The expected number of staged diff reads.
		wantCommits []string  // The expected committed messages, in order.
	}{
		{
			name:        "accept commits the generated message",
			key:         "sk-or-v1-test-key",
			input:       "y\n",
			diffOut:     diff,
			script:      []genTurn{okGen(msg, 0.9)},
			wantDiff:    1,
			wantCommits: []string{msg},
		},
		{
			name:        "missing API key fails before any work",
			key:         "",
			wantErr:     true,
			wantErrText: "new client",
			wantDiff:    0, // Git must not be invoked at all.
		},
		{
			name:        "cancel aborts without committing",
			key:         "sk-or-v1-test-key",
			input:       "n\n",
			diffOut:     diff,
			script:      []genTurn{okGen(msg, 0.9)},
			wantErr:     true,
			wantErrText: "commit aborted by user",
			wantDiff:    1,
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario API key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Create the fake git with the scenario's diff.
			git := &fakeGit{diffOut: tc.diffOut}
			// Swap the git diff seam for the duration of the test.
			t.Cleanup(cli.SetStagedDiff(git.stagedDiff))
			// Swap the git commit seam for the duration of the test.
			t.Cleanup(cli.SetCommit(git.commit))
			// Create the fake generator with the scripted turns.
			fake := &fakeGenerate{script: tc.script}
			// Swap the generation seam for the duration of the test.
			t.Cleanup(cli.SetGenerateMessage(fake.run))
			// Replace the process streams with canned input and a capture file.
			useStdio(t, tc.input)
			// Run the workflow under test.
			err := cli.Run()
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
			// Check the git seam invocation contract.
			wantGitCalls(t, git, tc.wantDiff, tc.wantCommits)
		})
	}
}

// TestHasBreakingChange tests the HasBreakingChange function with valid messages
// It checks that the function returns the expected value for each message in the slice
func TestHasBreakingChange(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"scoped header marker", "feat(api)!: remove legacy config keys", true},
		{"plain header marker", "chore(x)!: drop Go 1.20 support", true},
		{"footer", "feat(x): x\n\nBREAKING CHANGE: config keys renamed", true},
		{"no marker", "fix(x): handle empty diff", false},
		{"exclamation in description", "feat(x): support wow!", false},
		{"footer text mid-prose", "feat(x): x\n\nWe avoided BREAKING CHANGE: here", false},
		{"hyphen footer", "feat(x): x\n\nBREAKING-CHANGE: renamed", true},
		{"empty footer", "feat(x): x\n\nBREAKING CHANGE:", true},
		{"marker in description", "feat(x): document the !: marker", false},
	}
	// for each test case in the slice, call the function
	for _, tc := range cases {
		// if HasBreakingChange returns the expected value, the test passes
		if got := cli.HasBreakingChange(tc.msg); got != tc.want {
			t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "HasBreakingChange", Actual: fmt.Sprintf("%v", got), Want: fmt.Sprintf("%v", tc.want)}))
		}
	}
}
