// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"context" // Import the context package for context handling.
	"errors"  // Import the errors package for error handling.
	"fmt"     // Import the fmt package for formatting quoted values in failures.
	"strings" // Import the strings package for string handling.
	"testing" // Import the testing package for unit tests.

	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
	"github.com/thorsphere/tscommit"   // Import the tscommit package under test.
	"github.com/thorsphere/tserr"      // Import the tserr package for error reporting.
)

// chatTurn is one scripted provider response: a canned result and an error.
type chatTurn struct {
	res tsaiclient.Result // The canned result for this call.
	err error             // The canned error for this call.
}

// fakeChat implements the AI completion seam without a provider. It records
// every request and replays scripted turns in order.
type fakeChat struct {
	calls  int        // The number of times the seam was invoked.
	reqs   []string   // The recorded request renderings, one per call.
	script []chatTurn // The canned turns replayed in order.
}

// run records the request and replays the next scripted turn.
func (f *fakeChat) run(_ context.Context, _ *tsaiclient.Client, req *tsaiclient.Request) (tsaiclient.Result, error) {
	// Record the rendered request so prompts and session IDs can be asserted.
	f.reqs = append(f.reqs, fmt.Sprintf("%+v", req))
	// Count the invocation.
	f.calls++
	// Return an error when the script is exhausted; the call-count
	// assertion reports the over-invocation.
	if f.calls > len(f.script) {
		// Return an error for the unexpected extra call.
		return tsaiclient.Result{}, errors.New("fakeChat: script exhausted")
	}
	// Replay the next scripted turn.
	turn := f.script[f.calls-1]
	// Return the canned result and error.
	return turn.res, turn.err
}

// useFakeChat swaps the AI completion seam for the duration of the test.
// Tests using it must not call t.Parallel.
func useFakeChat(t *testing.T, f *fakeChat) {
	// Mark the test function as test helper function.
	t.Helper()
	// Swap the completion seam for the duration of the test and undo the
	// swap at test end.
	t.Cleanup(tscommit.SetCompleteChat(f.run))
}

// TestGenMessage tests genMessage: first-attempt acceptance, wrapper
// cleaning, retry with validation feedback, retry with truncation feedback,
// exhaustion, non-retryable empty content, and provider failure.
func TestGenMessage(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name        string
		script      []chatTurn // The scripted provider turns.
		want        string     // The expected message.
		wantErr     bool       // Whether an error is expected.
		wantErrText string     // Text the error must contain.
		wantCalls   int        // The expected number of provider calls.
	}{
		{
			name:      "valid on first attempt",
			script:    []chatTurn{{res: tsaiclient.Result{Content: "feat(login): add login"}}},
			want:      "feat(login): add login",
			wantCalls: 1,
		},
		{
			name:      "wrapper formatting cleaned",
			script:    []chatTurn{{res: tsaiclient.Result{Content: "\"feat(login): add login\""}}},
			want:      "feat(login): add login",
			wantCalls: 1,
		},
		{
			name: "retry after validation failure",
			script: []chatTurn{
				{res: tsaiclient.Result{Content: "wip(login): x"}},
				{res: tsaiclient.Result{Content: "feat(login): add login"}},
			},
			want:      "feat(login): add login",
			wantCalls: 2,
		},
		{
			name: "retry after truncation",
			script: []chatTurn{
				{res: tsaiclient.Result{Content: "feat(login): add login", Truncated: true}},
				{res: tsaiclient.Result{Content: "feat(login): add login"}},
			},
			want:      "feat(login): add login",
			wantCalls: 2,
		},
		{
			name: "invalid on both attempts",
			script: []chatTurn{
				{res: tsaiclient.Result{Content: "wip(login): x"}},
				{res: tsaiclient.Result{Content: "wip(login): y"}},
			},
			wantErr:     true,
			wantErrText: "generate message",
			wantCalls:   2,
		},
		{
			name: "truncated on both attempts",
			script: []chatTurn{
				{res: tsaiclient.Result{Content: "feat(login): add login", Truncated: true}},
				{res: tsaiclient.Result{Content: "feat(login): add login", Truncated: true}},
			},
			wantErr:     true,
			wantErrText: "output was truncated by the token limit",
			wantCalls:   2,
		},
		{
			name:        "empty after cleaning is not retried",
			script:      []chatTurn{{res: tsaiclient.Result{Content: "   "}}},
			wantErr:     true,
			wantErrText: "AI client commit message",
			wantCalls:   1,
		},
		{
			name:        "provider failure is not retried",
			script:      []chatTurn{{err: errors.New("boom")}},
			wantErr:     true,
			wantErrText: "boom",
			wantCalls:   1,
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake provider with the scripted turns.
			fake := &fakeChat{script: tc.script}
			// Swap the completion seam for the duration of the test.
			useFakeChat(t, fake)
			// Generate the message; the nil client is never reached because
			// the seam intercepts the only call site.
			got, err := tscommit.GenMessage(context.Background(), nil, "diff --git a/x b/x", "session-42")
			// If the error state doesn't match, fail the test.
			if (err != nil) != tc.wantErr {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "genMessage error", Actual: fmt.Sprintf("%v", err != nil), Want: fmt.Sprintf("%v", tc.wantErr)}))
			}
			// If the message differs from the expectation, fail the test.
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "genMessage", Actual: fmt.Sprintf("%q", got), Want: fmt.Sprintf("%q", tc.want)}))
			}
			// If the provider call count is wrong, fail the test.
			if fake.calls != tc.wantCalls {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "completeChat calls", Actual: fmt.Sprintf("%v", fake.calls), Want: fmt.Sprintf("%v", tc.wantCalls)}))
			}
			// If the diagnostic text is missing from the error, fail the test.
			if tc.wantErrText != "" && err != nil && !strings.Contains(err.Error(), tc.wantErrText) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "genMessage error text", Actual: err.Error(), Want: tc.wantErrText}))
			}
			// If no request carried the sticky session ID, fail the test.
			if len(fake.reqs) > 0 && !strings.Contains(fake.reqs[0], "session-42") {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "session ID", Actual: fake.reqs[0], Want: "request carrying session-42"}))
			}
		})
	}
}

// TestCleanMessage tests that cleanMessage strips wrapper formatting the
// model may add around a commit message — markdown code fences and a single
// matching pair of outer quotes or backticks — while leaving legitimate
// punctuation and multibyte content untouched.
func TestCleanMessage(t *testing.T) {
	// Test cases.
	cases := []struct {
		name string
		in   string // The raw model output.
		want string // The expected cleaned message.
	}{
		{"bare text", "feat(login): add login", "feat(login): add login"},
		{"empty", "", ""},
		{"whitespace only", "  \n\t ", ""},
		{"fenced block", "```\nfeat(login): add login\n```", "feat(login): add login"},
		{"fenced block with language tag", "```commit\nfeat(login): add login\n```", "feat(login): add login"},
		{"fence without newline", "```feat(login): add login```", "feat(login): add login"},
		{"unterminated fence", "```\nfeat(login): add login", "feat(login): add login"},
		{"fence with outer whitespace", "  ```\nfeat(login): add login\n```  ", "feat(login): add login"},
		{"fenced then quoted", "```\n\"feat(login): add login\"\n```", "feat(login): add login"},
		{"double-quoted wrapper", "\"feat(login): add login\"", "feat(login): add login"},
		{"single-quoted wrapper", "'feat(login): add login'", "feat(login): add login"},
		{"backtick wrapper", "`feat(login): add login`", "feat(login): add login"},
		{"wrapper trims inner whitespace", "\" feat(login): add login \"", "feat(login): add login"},
		{"trailing apostrophe preserved", "fix(login): handle trailing '", "fix(login): handle trailing '"},
		{"trailing quote preserved", `docs(login): fix typo in "README"`, `docs(login): fix typo in "README"`},
		{"mismatched wrappers preserved", "\"feat(login): add login'", "\"feat(login): add login'"},
		{"lone quote preserved", "'", "'"},
		{"lone quote pair unwraps", "''", ""},
		{"same-letter ends preserved", "t(login): t", "t(login): t"},
		{"multibyte content untouched", "feat(login): ééé", "feat(login): ééé"},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test.
			got := tscommit.CleanMessage(tc.in)
			// If the cleaned message is not the expected one, fail the test.
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "cleanMessage", Actual: fmt.Sprintf("%q", got), Want: fmt.Sprintf("%q", tc.want)}))
			}
		})
	}
}

// TestNewSessionID verifies the session ID contract: every call yields a
// non-empty identifier and distinct calls yield distinct identifiers.
func TestNewSessionID(t *testing.T) {
	// Generate a small batch of session IDs.
	const n = 100
	seen := make(map[string]bool, n)
	for range n {
		// Generate one session ID.
		id := tscommit.NewSessionID()
		// Check that the identifier is not empty.
		if id == "" {
			t.Error(tserr.Empty("NewSessionID"))
			return
		}
		// Check that the identifier has not been seen before.
		if seen[id] {
			t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "NewSessionID", Actual: fmt.Sprintf("%q", id), Want: "a unique identifier"}))
			return
		}
		// Record the identifier.
		seen[id] = true
	}
}
