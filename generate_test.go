// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Import packages
import (
	"context" // Import the context package for cancellation and deadlines.
	"errors"  // Import the errors package for sentinel error creation.
	"fmt"     // Import the fmt package for formatting values in failure diagnostics.
	"strings" // Import the strings package for substring assertions.
	"testing" // Import the testing package for table-driven unit tests.

	"github.com/thorsphere/tscommit" // Import the tscommit package under test.
	"github.com/thorsphere/tserr"    // Import the tserr package for error reporting.
)

// TestNewGenerator tests NewGenerator across valid and invalid provider/API-key
// environment configurations. It uses t.Setenv and must not call t.Parallel.
func TestNewGenerator(t *testing.T) {
	// Define the environment configuration scenarios.
	cases := []struct {
		name        string // The scenario name.
		key         string // The OPENROUTER_API_KEY value installed for the test.
		wantErr     bool   // Whether construction must fail.
		wantErrText string // Substring the returned error must contain.
	}{
		{
			name: "valid API key builds generator",
			key:  "sk-or-v1-test-key",
		},
		{
			name:        "missing API key fails",
			key:         "",
			wantErr:     true,
			wantErrText: "new client",
		},
		{
			name:        "whitespace API key fails",
			key:         "   \t\n",
			wantErr:     true,
			wantErrText: "new client",
		},
		{
			name:        "control character in API key fails",
			key:         "bad\nkey",
			wantErr:     true,
			wantErrText: "new client",
		},
	}
	// Run each scenario against NewGenerator.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario API key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Construct the generator under test.
			gen, err := tscommit.NewGenerator()
			// Check the error expectation.
			if tc.wantErr {
				// Fail if an expected error is missing.
				if err == nil {
					t.Error(tserr.NilFailed("NewGenerator"))
				} else if tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
					// Fail if the diagnostic substring is missing.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "NewGenerator error text", Actual: err.Error(), Want: tc.wantErrText}))
				}
				// Ensure no non-nil generator leaked alongside an error.
				if gen != nil {
					t.Errorf("NewGenerator = %v, want nil on error", gen)
				}
				return
			}
			// Fail on any unexpected error.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewGenerator", Err: err}))
				return
			}
			// Fail if the returned generator is nil on success.
			if gen == nil {
				t.Error(tserr.NilFailed("NewGenerator"))
			}
		})
	}
}

// TestGeneratorGenerate tests Generator.Generate across valid outputs, retry
// recovery, generation errors, AI output validation failure, sticky session
// reuse across multiple calls, and defensive receiver/argument guards. It
// swaps the AI seam and must not call t.Parallel.
func TestGeneratorGenerate(t *testing.T) {
	// The sentinel error simulated by the provider seam.
	providerErr := errors.New("upstream unavailable")
	// Define the test cases.
	cases := []struct {
		name        string             // The scenario name.
		nilGen      bool               // When true, invoke Generate on a nil *Generator.
		zeroGen     bool               // When true, invoke Generate on a zero-value Generator{}.
		nilCtx      bool               // When true, pass a nil context.
		diffs       []string           // The diffs passed to sequential Generate calls on one Generator.
		script      []chatTurn         // The scripted provider turns across all calls.
		want        []tscommit.Message // The expected Message values per call, in order.
		wantErr     bool               // Whether the final call must return an error.
		wantErrText string             // Substring the returned error must contain.
		wantCalls   int                // The expected total number of provider calls.
		wantSticky  bool               // Whether all provider calls must share one non-empty session ID.
	}{
		{
			name:      "single call returns message and confidence",
			diffs:     []string{"diff --git a/a.go b/a.go\n"},
			script:    []chatTurn{okTurn("feat(core): add generator", 0.91)},
			want:      []tscommit.Message{{Text: "feat(core): add generator", Confidence: 0.91}},
			wantCalls: 1,
		},
		{
			name:   "multiple calls reuse sticky session ID across diffs",
			diffs:  []string{"diff --git a/a.go b/a.go\n", "diff --git a/b.go b/b.go\n"},
			script: []chatTurn{okTurn("feat(core): add generator", 0.85), okTurn("fix(core): handle empty diff", 0.93)},
			want: []tscommit.Message{
				{Text: "feat(core): add generator", Confidence: 0.85},
				{Text: "fix(core): handle empty diff", Confidence: 0.93},
			},
			wantCalls:  2,
			wantSticky: true,
		},
		{
			name:  "validation failure retries and recovers with same session ID",
			diffs: []string{"diff --git a/a.go b/a.go\n"},
			script: []chatTurn{
				okTurn("invalid header without scope", 0.4),
				okTurn("feat(core): add generator", 0.89),
			},
			want:       []tscommit.Message{{Text: "feat(core): add generator", Confidence: 0.89}},
			wantCalls:  2,
			wantSticky: true,
		},
		{
			name:  "validation failure on all attempts returns error and zero Message",
			diffs: []string{"diff --git a/a.go b/a.go\n"},
			script: []chatTurn{
				okTurn("wip: missing scope", 0.5),
				okTurn("feat: still missing scope", 0.5),
			},
			wantErr:     true,
			wantErrText: "generate message",
			wantCalls:   2,
			wantSticky:  true,
		},
		{
			name:        "provider generation error fails without retry",
			diffs:       []string{"diff --git a/a.go b/a.go\n"},
			script:      []chatTurn{{err: providerErr}},
			wantErr:     true,
			wantErrText: "upstream unavailable",
			wantCalls:   1,
		},
		{
			name:        "nil receiver returns error without calling provider",
			nilGen:      true,
			diffs:       []string{"diff --git a/a.go b/a.go\n"},
			wantErr:     true,
			wantErrText: "nil",
			wantCalls:   0,
		},
		{
			name:        "zero-value Generator returns error without calling provider",
			zeroGen:     true,
			diffs:       []string{"diff --git a/a.go b/a.go\n"},
			wantErr:     true,
			wantErrText: "AI client",
			wantCalls:   0,
		},
		{
			name:        "nil context returns error without calling provider",
			nilCtx:      true,
			diffs:       []string{"diff --git a/a.go b/a.go\n"},
			wantErr:     true,
			wantErrText: "context",
			wantCalls:   0,
		},
		{
			name:        "empty diff returns error without calling provider",
			diffs:       []string{""},
			wantErr:     true,
			wantErrText: "diff",
			wantCalls:   0,
		},
		{
			name:        "whitespace-only diff returns error without calling provider",
			diffs:       []string{" \t\n "},
			wantErr:     true,
			wantErrText: "diff",
			wantCalls:   0,
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Install a valid API key for constructing the generator.
			t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test-key")
			// Swap the AI completion seam with the scripted turns.
			fake := &fakeChat{script: tc.script}
			// Install the fake AI completion seam for the subtest.
			useFakeChat(t, fake)
			// Swap git and editor seams to fail loudly if Generate ever invokes them.
			git := &fakeRunGit{}
			// Install the fake git execution seam for the subtest.
			t.Cleanup(tscommit.SetExecGit(git.run))
			// Prepare the generator instance according to the scenario.
			var gen *tscommit.Generator
			switch {
			case tc.nilGen:
				// Leave gen nil to exercise the nil-receiver guard.
			case tc.zeroGen:
				// Use an uninitialized Generator literal to exercise the zero-value guard.
				gen = &tscommit.Generator{}
			default:
				// Construct a valid generator via NewGenerator.
				built, err := tscommit.NewGenerator()
				// Fail if setup construction unexpectedly fails.
				if err != nil {
					t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewGenerator", Err: err}))
					return
				}
				// Assign the constructed generator for the test calls.
				gen = built
			}
			// Prepare the context for the call.
			var ctx context.Context = context.Background()
			// Override ctx with nil when testing the nil-context guard.
			if tc.nilCtx {
				// Set a typed nil context to test the nil-context guard.
				ctx = nil
			}
			// Execute each diff against the same generator instance.
			var gotMsgs []tscommit.Message
			// Record the latest error returned by Generate.
			var lastErr error
			// Iterate through every scripted diff on the single Generator.
			for _, d := range tc.diffs {
				// Invoke Generate for the current diff.
				msg, err := gen.Generate(ctx, d)
				// Record the error from this invocation.
				lastErr = err
				// Stop on error and verify the zero-value Message contract.
				if err != nil {
					// Fail if a non-zero Message leaked on error.
					if msg != (tscommit.Message{}) {
						t.Errorf("Generate message on error = %+v, want zero Message{}", msg)
					}
					// Stop calling Generate after the expected failure.
					break
				}
				// Collect the returned Message for comparison.
				gotMsgs = append(gotMsgs, msg)
			}
			// Check error presence and text.
			if tc.wantErr {
				// Fail if the expected error was not returned.
				if lastErr == nil {
					t.Error(tserr.NilFailed("Generator.Generate"))
				} else if tc.wantErrText != "" && !strings.Contains(lastErr.Error(), tc.wantErrText) {
					// Fail if the error message lacks the expected diagnostic text.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Generate error text", Actual: lastErr.Error(), Want: tc.wantErrText}))
				}
			} else {
				// Fail on any unexpected error.
				if lastErr != nil {
					t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "Generator.Generate", Err: lastErr}))
				}
				// Compare the collected composite Message slice against expectations.
				if len(gotMsgs) != len(tc.want) {
					t.Errorf("Generate results = %+v, want %+v", gotMsgs, tc.want)
				} else {
					// Compare each returned Message value against the expected one.
					for i := range tc.want {
						// Fail if any field of the composite Message differs.
						if gotMsgs[i] != tc.want[i] {
							t.Errorf("Generate[%d] = %+v, want %+v", i, gotMsgs[i], tc.want[i])
						}
					}
				}
			}
			// Verify the provider call count.
			if fake.calls != tc.wantCalls {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "completeChat calls", Actual: fmt.Sprintf("%d", fake.calls), Want: fmt.Sprintf("%d", tc.wantCalls)}))
			}
			// Verify that Generate never touched git.
			if len(git.args) != 0 {
				t.Errorf("git calls = %v, want none", git.args)
			}
			// Verify sticky session ID reuse across all recorded requests.
			if tc.wantSticky && len(fake.reqs) > 0 {
				// Extract the session ID from the first recorded request.
				first := sessionIDFrom(t, fake.reqs[0])
				// Fail if the minted session ID is empty.
				if first == "" {
					t.Error(tserr.Empty("Generator sessionID"))
				}
				// Compare all subsequent requests against the first session ID.
				for _, req := range fake.reqs[1:] {
					// Fail if any request carried a different session ID.
					if id := sessionIDFrom(t, req); id != first {
						t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "session ID", Actual: id, Want: first}))
					}
				}
			}
		})
	}
}
