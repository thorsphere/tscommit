// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"context" // Import the context package for context handling.
	"errors"  // Import the errors package for error handling.
	"fmt"     // Import the fmt package for formatting quoted values in failures.
	"math"    // Import the math package for NaN and infinity test values.
	"strings" // Import the strings package for string handling.
	"testing" // Import the testing package for unit tests.

	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
	"github.com/thorsphere/tscommit"   // Import the tscommit package under test.
	"github.com/thorsphere/tserr"      // Import the tserr package for error reporting.
)

// TestGenMessage tests genMessage: first-attempt acceptance, wrapper
// cleaning, retry with validation feedback, retry with truncation feedback,
// exhaustion, non-retryable empty content, envelope contract failures, and
// provider/decode failure.
func TestGenMessage(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name        string           // The scenario name.
		script      []chatTurn       // The scripted provider turns.
		want        tscommit.Message // The expected message with completion metadata.
		wantErr     bool             // Whether an error is expected.
		wantErrText string           // Text the error must contain.
		wantCalls   int              // The expected number of provider calls.
	}{
		{
			name:      "valid on first attempt",
			script:    []chatTurn{okTurn("feat(login): add login", 0.92)},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 0.92},
			wantCalls: 1,
		},
		{
			name:      "wrapper formatting cleaned",
			script:    []chatTurn{okTurn("\"feat(login): add login\"", 0.85)},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 0.85},
			wantCalls: 1,
		},
		{
			name:      "unreported metadata stays zero",
			script:    []chatTurn{okTurnMeta("feat(login): add login", 0.75, "", 0, 0)},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 0.75},
			wantCalls: 1,
		},
		{
			name: "first-attempt metadata propagated",
			script: []chatTurn{{res: tsaiclient.JSONResult{
				Envelope: tsaiclient.Envelope{Findings: []string{"feat(login): add login"}, Confidence: 0.9},
				Model:    "google/gemini-4-hydrogen",
				Usage:    tsaiclient.Usage{PromptTokens: 3, CompletionTokens: 5, ReasoningTokens: 2, TotalTokens: 120, Cost: 0.125},
			}}},
			want: tscommit.Message{
				Text:        "feat(login): add login",
				Confidence:  0.9,
				Model:       "google/gemini-4-hydrogen",
				TotalTokens: 120,
				Cost:        0.125,
			},
			wantCalls: 1,
		},
		{
			name:      "zero confidence boundary accepted",
			script:    []chatTurn{okTurn("feat(login): add login", 0)},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 0},
			wantCalls: 1,
		},
		{
			name:      "max confidence boundary accepted",
			script:    []chatTurn{okTurn("feat(login): add login", 1.0)},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 1.0},
			wantCalls: 1,
		},
		{
			name: "retry after validation failure",
			script: []chatTurn{
				okTurn("wip(login): x", 0.4),
				okTurn("feat(login): add login", 0.95),
			},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 0.95},
			wantCalls: 2,
		},
		{
			name: "retry accumulates metadata across validation failure",
			script: []chatTurn{
				okTurnMeta("wip(login): x", 0.4, "google/gemini-4-hydrogen", 120, 0.125),
				okTurnMeta("feat(login): add login", 0.95, "google/gemini-4-hydrogen-001", 150, 0.25),
			},
			want: tscommit.Message{
				Text:        "feat(login): add login",
				Confidence:  0.95,
				Model:       "google/gemini-4-hydrogen-001",
				TotalTokens: 270,
				Cost:        0.375,
			},
			wantCalls: 2,
		},
		{
			name: "retry after truncation",
			script: []chatTurn{
				{res: tsaiclient.JSONResult{
					Envelope:  tsaiclient.Envelope{Findings: []string{"feat(login): add login"}, Confidence: 0.8},
					Truncated: true,
				}},
				okTurn("feat(login): add login", 0.9),
			},
			want:      tscommit.Message{Text: "feat(login): add login", Confidence: 0.9},
			wantCalls: 2,
		},
		{
			name: "retry accumulates metadata across truncation",
			script: []chatTurn{
				{res: tsaiclient.JSONResult{
					Envelope:  tsaiclient.Envelope{Findings: []string{"feat(login): add login"}, Confidence: 0.8},
					Model:     "m1",
					Truncated: true,
					Usage:     tsaiclient.Usage{TotalTokens: 100, Cost: 0.125},
				}},
				okTurnMeta("feat(login): add login", 0.9, "m1", 200, 0.25),
			},
			want: tscommit.Message{
				Text:        "feat(login): add login",
				Confidence:  0.9,
				Model:       "m1",
				TotalTokens: 300,
				Cost:        0.375,
			},
			wantCalls: 2,
		},
		{
			name: "model falls back to latest non-empty across retry",
			script: []chatTurn{
				okTurnMeta("wip(login): x", 0.4, "m1", 10, 0.125),
				okTurnMeta("feat(login): add login", 0.95, "", 20, 0.25),
			},
			want: tscommit.Message{
				Text:        "feat(login): add login",
				Confidence:  0.95,
				Model:       "m1",
				TotalTokens: 30,
				Cost:        0.375,
			},
			wantCalls: 2,
		},
		{
			name: "invalid on both attempts",
			script: []chatTurn{
				okTurnMeta("wip(login): x", 0.5, "m1", 10, 0.125),
				okTurnMeta("wip(login): y", 0.5, "m2", 20, 0.25),
			},
			wantErr:     true,
			wantErrText: "generate message",
			wantCalls:   2,
		},
		{
			name: "truncated on both attempts",
			script: []chatTurn{
				{res: tsaiclient.JSONResult{
					Envelope:  tsaiclient.Envelope{Findings: []string{"feat(login): add login"}, Confidence: 0.8},
					Model:     "m1",
					Truncated: true,
					Usage:     tsaiclient.Usage{TotalTokens: 10, Cost: 0.125},
				}},
				{res: tsaiclient.JSONResult{
					Envelope:  tsaiclient.Envelope{Findings: []string{"feat(login): add login"}, Confidence: 0.8},
					Model:     "m2",
					Truncated: true,
					Usage:     tsaiclient.Usage{TotalTokens: 20, Cost: 0.25},
				}},
			},
			wantErr:     true,
			wantErrText: "output was truncated by the token limit",
			wantCalls:   2,
		},
		{
			name:        "empty findings is not retried",
			script:      []chatTurn{{res: tsaiclient.JSONResult{Envelope: tsaiclient.Envelope{Confidence: 0.9}}}},
			wantErr:     true,
			wantErrText: "AI client commit message",
			wantCalls:   1,
		},
		{
			name: "multiple findings is not retried",
			script: []chatTurn{{res: tsaiclient.JSONResult{
				Envelope: tsaiclient.Envelope{Findings: []string{"feat(a): one", "feat(b): two"}, Confidence: 0.9},
			}}},
			wantErr:     true,
			wantErrText: "AI client findings count",
			wantCalls:   1,
		},
		{
			name:        "negative confidence is not retried",
			script:      []chatTurn{okTurn("feat(login): add login", -0.1)},
			wantErr:     true,
			wantErrText: "AI client confidence",
			wantCalls:   1,
		},
		{
			name:        "confidence above 1 is not retried",
			script:      []chatTurn{okTurn("feat(login): add login", 1.1)},
			wantErr:     true,
			wantErrText: "AI client confidence",
			wantCalls:   1,
		},
		{
			name:        "NaN confidence is not retried",
			script:      []chatTurn{okTurn("feat(login): add login", math.NaN())},
			wantErr:     true,
			wantErrText: "AI client confidence",
			wantCalls:   1,
		},
		{
			name:        "empty after cleaning is not retried",
			script:      []chatTurn{okTurn("   ", 0.9)},
			wantErr:     true,
			wantErrText: "AI client commit message",
			wantCalls:   1,
		},
		{
			name:        "provider or decode failure is not retried",
			script:      []chatTurn{{err: errors.New("decode envelope failed: invalid character")}},
			wantErr:     true,
			wantErrText: "decode envelope",
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
			// Verify the zero-value Message contract on error.
			if tc.wantErr && got != (tscommit.Message{}) {
				t.Errorf("genMessage message on error = %+v, want zero Message{}", got)
			}
			// If the message differs from the expectation, fail the test.
			if got != tc.want {
				t.Errorf("genMessage = %+v, want %+v", got, tc.want)
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
