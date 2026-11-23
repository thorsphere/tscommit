// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package cli_test

// Imports for testing.
import (
	"bytes"   // Import the bytes package for capturing prompter output.
	"context" // Import the context package for context handling.
	"errors"  // Import the errors package for creating a fake error.
	"fmt"     // Import the fmt package for formatting quoted values in failures.
	"strings" // Import the strings package for string manipulation.
	"testing" // Import the testing package for table-driven tests.

	"github.com/thorsphere/lpcli"                 // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tscommit"              // Import the tscommit package for Message values.
	"github.com/thorsphere/tscommit/internal/cli" // Import the cli package under test.
	"github.com/thorsphere/tscommit/internal/git" // Import the internal git package for truncation suffix constants.
	"github.com/thorsphere/tserr"                 // Import the tserr package for error reporting.
)

// TestPromptUserSelects tests that promptUser maps each typed key and alias
// to the expected choice value, falls back to the default on Enter, and
// re-prompts on unknown input.
func TestPromptUserSelects(t *testing.T) {
	// Test cases.
	cases := []struct {
		name      string
		input     string // The canned stdin for the prompter.
		want      string // The expected choice value.
		wantRetry bool   // Whether the output must show a retry message.
	}{
		{"default key", "y\n", cli.ChoiceAccept, false},
		{"default alias", "yes\n", cli.ChoiceAccept, false},
		{"enter selects default", "\n", cli.ChoiceAccept, false},
		{"edit key", "e\n", cli.ChoiceEdit, false},
		{"edit alias", "edit\n", cli.ChoiceEdit, false},
		{"regenerate key", "r\n", cli.ChoiceRegenerate, false},
		{"regenerate alias", "regen\n", cli.ChoiceRegenerate, false},
		{"cancel key", "n\n", cli.ChoiceCancel, false},
		{"cancel alias", "q\n", cli.ChoiceCancel, false},
		// Matching is case-insensitive.
		{"uppercase key", "E\n", cli.ChoiceEdit, false},
		// Unknown input is rejected and the prompt is asked again.
		{"unknown key retries", "x\ny\n", cli.ChoiceAccept, true},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the prompter with canned input and captured output.
			p := cli.NewPrompter()
			// Capture the prompter output.
			var buf bytes.Buffer
			// Route the prompter output into the buffer. If that fails, fail
			// the test.
			if err := p.SetOut(&buf); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetOut", Err: err}))
				return
			}
			// Route the canned input into the prompter. If that fails, fail
			// the test.
			if err := p.SetIn(strings.NewReader(tc.input)); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetIn", Err: err}))
				return
			}
			// Call the function under test.
			res := tscommit.Message{
				Text:        "feat(x): xyz",
				Confidence:  0.92,
				Model:       "google/gemini-4-hydrogen",
				TotalTokens: 320,
				Cost:        0.0025,
			}
			// Invoke promptUser with AI metadata display enabled.
			got, err := cli.PromptUser(context.Background(), p, cli.PromptParams{
				Message:   res,
				Effort:    "high",
				DiffChars: 1420,
				ShowAI:    true,
			})
			// If prompting failed, fail the test.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// If the returned choice differs from the expectation, fail the test.
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptUser", Actual: got, Want: tc.want}))
			}
			// The generation summary line must appear before the candidate message.
			wantSummary := "Diff: 1420 chars | Model: google/gemini-4-hydrogen (high) | Tokens: 320 | Cost: $0.0025"
			// Fail when the summary line is missing from the output.
			if !strings.Contains(buf.String(), wantSummary) {
				t.Errorf("output %q does not show the summary line %q", buf.String(), wantSummary)
			}
			// The candidate message must be visible to the user after the summary line.
			if si, mi := strings.Index(buf.String(), wantSummary), strings.Index(buf.String(), "feat(x): xyz"); mi < 0 || si > mi {
				t.Errorf("output %q does not show summary before candidate message", buf.String())
			}
			// The confidence score must be visible to the user.
			if !strings.Contains(buf.String(), "Confidence: 92%") {
				t.Errorf("output %q does not show the confidence score", buf.String())
			}
			// If the retry expectation does not match the output, fail the test.
			if retry := strings.Contains(buf.String(), "Unknown option"); retry != tc.wantRetry {
				t.Errorf("retry message present = %v, want %v", retry, tc.wantRetry)
			}
		})
	}
}

// TestPromptUserWarns tests that promptUser shows the candidate message,
// prints the breaking-change warning after the message and before the
// prompt when msg carries a breaking-change marker, and stays silent
// otherwise.
func TestPromptUserWarns(t *testing.T) {
	// Test cases.
	cases := []struct {
		name string
		msg  string
		want string // The expected warning text; empty when no warning is expected.
	}{
		{"bang marker warns", "feat!: drop the v1 api", cli.BreakingWarning},
		{"footer warns", "feat: rework storage\n\nBREAKING CHANGE: the layout changed", cli.BreakingWarning},
		{"plain message silent", "feat: add the thing", ""},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the prompter with canned input and captured output.
			p := cli.NewPrompter()
			// Capture the prompter output.
			var buf bytes.Buffer
			// Route the prompter output into the buffer. If that fails, fail
			// the test.
			if err := p.SetOut(&buf); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetOut", Err: err}))
				return
			}
			// Route the canned input into the prompter. If that fails, fail
			// the test.
			if err := p.SetIn(strings.NewReader("y\n")); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetIn", Err: err}))
				return
			}
			// Call the function under test.
			_, err := cli.PromptUser(context.Background(), p, cli.PromptParams{
				Message:   tscommit.Message{Text: tc.msg, Confidence: 0.85, Model: "m1"},
				Effort:    "high",
				DiffChars: 42,
				ShowAI:    true,
			})
			// If prompting failed, fail the test.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// Check the message and warning against the captured output.
			got := buf.String()
			// The candidate message must always be shown.
			if !strings.Contains(got, tc.msg) {
				t.Error(tserr.NotFound(fmt.Sprintf("candidate message %q in promptUser output %q", tc.msg, got)))
			}
			// If a warning is expected, it must appear after the message and
			// before the prompt.
			if tc.want != "" {
				// Locate the warning and the message in the output.
				wi, mi := strings.Index(got, tc.want), strings.Index(got, tc.msg)
				// The warning must be present.
				if wi < 0 {
					t.Error(tserr.NotFound(fmt.Sprintf("breaking-change warning in promptUser output %q", got)))
				}
				// The warning must come after the message.
				if wi >= 0 && wi < mi {
					t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
						F:      "promptUser output",
						Value:  got,
						Detail: "breaking-change warning must appear after the candidate message",
					}))
				}
				// The warning must come before the prompt.
				if pi := strings.Index(got, "Apply this commit?"); pi >= 0 && wi > pi {
					t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
						F:      "promptUser output",
						Value:  got,
						Detail: "breaking-change warning must appear before the prompt",
					}))
				}
			}
			// If no warning is expected, the output must not contain one.
			if tc.want == "" && strings.Contains(got, "BREAKING CHANGE detected") {
				t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "promptUser output",
					Value:  got,
					Detail: "unexpected breaking-change warning for a message without a breaking-change marker",
				}))
			}
		})
	}
}

// TestPromptUserErrors tests promptUser with an
// error when prompting fails, so callers abort instead of committing.
func TestPromptUserErrors(t *testing.T) {
	// Create a context that is already cancelled.
	cancelled, cancel := context.WithCancel(context.Background())
	// Cancel the context immediately.
	cancel()
	// Test cases.
	cases := []struct {
		name  string
		ctx   context.Context // The context for the prompt.
		input string          // The canned stdin for the prompter.
	}{
		{"cancelled context", cancelled, "y\n"},
		{"input exhausted", context.Background(), ""},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the prompter with canned input and captured output.
			p := cli.NewPrompter()
			// Capture the prompter output.
			var buf bytes.Buffer
			// Route the prompter output into the buffer. If that fails, fail
			// the test.
			if err := p.SetOut(&buf); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetOut", Err: err}))
				return
			}
			// Route the canned input into the prompter. If that fails, fail
			// the test.
			if err := p.SetIn(strings.NewReader(tc.input)); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetIn", Err: err}))
				return
			}
			// Call the function under test.
			got, err := cli.PromptUser(tc.ctx, p, cli.PromptParams{
				Message:   tscommit.Message{Text: "feat(x): xyz", Confidence: 0.9},
				Effort:    "high",
				DiffChars: 42,
				ShowAI:    true,
			})
			// If no error was returned, fail the test.
			if err == nil {
				t.Error(tserr.NilFailed("promptUser"))
			}
			// The failure must report an empty choice.
			if got != "" {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptUser", Actual: got, Want: ""}))
			}
		})
	}
}

// TestPromptUserConfidence tests that promptUser renders the summary header
// before the message and the confidence line after the message and before any
// warning or prompt when showAI is true, and omits both AI blocks when showAI
// is false (e.g. after user edit).
func TestPromptUserConfidence(t *testing.T) {
	// Test cases.
	cases := []struct {
		name        string  // The scenario name.
		confidence  float64 // The confidence score supplied to promptUser.
		effort      string  // The reasoning effort supplied to promptUser.
		truncated   bool    // Whether the staged diff was truncated.
		showAI      bool    // Whether AI summary and confidence display is enabled.
		wantSummary string  // The expected rendered summary header; empty when omitted.
		wantLine    string  // The expected rendered confidence line; empty when omitted.
	}{
		{name: "shown as rounded percentage", confidence: 0.924, effort: "high", showAI: true, wantSummary: "Diff: 500 chars | Model: m1 (high) | Tokens: 80 | Cost: $0.001", wantLine: "Confidence: 92%"},
		{name: "shown with truncated diff", confidence: 0.5, effort: "high", truncated: true, showAI: true, wantSummary: "Diff: 500 (Truncated) chars | Model: m1 (high) | Tokens: 80 | Cost: $0.001", wantLine: "Confidence: 50%"},
		{name: "shown at zero boundary", confidence: 0, effort: "high", showAI: true, wantSummary: "Diff: 500 chars | Model: m1 (high) | Tokens: 80 | Cost: $0.001", wantLine: "Confidence: 0%"},
		{name: "shown at max boundary", confidence: 1.0, effort: "high", showAI: true, wantSummary: "Diff: 500 chars | Model: m1 (high) | Tokens: 80 | Cost: $0.001", wantLine: "Confidence: 100%"},
		{name: "empty effort falls back to n/a", confidence: 0.5, effort: "", showAI: true, wantSummary: "Diff: 500 chars | Model: m1 (n/a) | Tokens: 80 | Cost: $0.001", wantLine: "Confidence: 50%"},
		{name: "omitted when disabled", confidence: 0.92, effort: "high", truncated: true, showAI: false, wantSummary: "", wantLine: ""},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the prompter with canned input and captured output.
			p := cli.NewPrompter()
			// Capture the prompter output.
			var buf bytes.Buffer
			// Route the prompter output into the buffer.
			if err := p.SetOut(&buf); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetOut", Err: err}))
				return
			}
			// Route canned input into the prompter.
			if err := p.SetIn(strings.NewReader("y\n")); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetIn", Err: err}))
				return
			}
			// Call promptUser with a breaking message to verify layout order.
			msg := "feat(api)!: drop v1"
			// Build the candidate Message value for the call.
			res := tscommit.Message{
				Text:        msg,
				Confidence:  tc.confidence,
				Model:       "m1",
				TotalTokens: 80,
				Cost:        0.001,
			}
			// Execute promptUser under test.
			if _, err := cli.PromptUser(context.Background(), p, cli.PromptParams{
				Message:   res,
				Effort:    tc.effort,
				DiffChars: 500,
				Truncated: tc.truncated,
				ShowAI:    tc.showAI,
			}); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// Inspect the captured output.
			got := buf.String()
			// When omitted, neither the summary header nor the Confidence line may appear.
			if tc.wantLine == "" {
				// Fail if a summary line or confidence line leaked into the output.
				if strings.Contains(got, "Diff:") || strings.Contains(got, "Confidence:") {
					t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
						F:      "promptUser output",
						Value:  got,
						Detail: "unexpected AI summary or confidence line when showAI is false",
					}))
				}
				return
			}
			// Locate summary, message, confidence line, and warning in the output.
			si := strings.Index(got, tc.wantSummary)
			mi := strings.Index(got, msg)
			ci := strings.Index(got, tc.wantLine)
			wi := strings.Index(got, cli.BreakingWarning)
			// Fail when the expected summary line is missing.
			if si < 0 {
				t.Error(tserr.NotFound(fmt.Sprintf("summary line %q in promptUser output %q", tc.wantSummary, got)))
				return
			}
			// Summary must appear before the commit message.
			if si > mi {
				t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "promptUser output",
					Value:  got,
					Detail: "summary line must appear before the candidate message",
				}))
			}
			// Fail when the expected confidence line is missing.
			if ci < 0 {
				t.Error(tserr.NotFound(fmt.Sprintf("confidence line %q in promptUser output %q", tc.wantLine, got)))
				return
			}
			// Confidence must appear after the message and before the warning.
			if ci < mi || (wi >= 0 && ci > wi) {
				t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
					F:      "promptUser output",
					Value:  got,
					Detail: "confidence line must appear after the message and before the breaking-change warning",
				}))
			}
		})
	}
}

// TestDiffInfo tests that diffInfo counts characters in runes and strips the
// git truncation suffix when present.
func TestDiffInfo(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name      string // The scenario name.
		diff      string // The staged diff input.
		wantChars int    // The expected character (rune) count.
		wantTrunc bool   // Whether truncation must be reported.
	}{
		{"plain ASCII diff", "diff --git a/x b/x\n", 19, false},
		{"multibyte runes counted as characters", "ééé", 3, false},
		{"truncated diff strips suffix", "12345" + git.TruncatedSuffix, 5, true},
		{"truncated multibyte diff strips suffix", "éé" + git.TruncatedSuffix, 2, true},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Measure the diff under test.
			gotChars, gotTrunc := cli.DiffInfo(tc.diff)
			// Fail if the character count differs from expectation.
			if gotChars != tc.wantChars {
				t.Errorf("diffInfo chars = %d, want %d", gotChars, tc.wantChars)
			}
			// Fail if the truncation flag differs from expectation.
			if gotTrunc != tc.wantTrunc {
				t.Errorf("diffInfo truncated = %v, want %v", gotTrunc, tc.wantTrunc)
			}
		})
	}
}

// TestFormatSummary tests that formatSummary renders diff character count,
// optional truncation indicator, model fallback, the reasoning effort in
// brackets after the model name, billed tokens, and non-scientific
// USD cost.
func TestFormatSummary(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name      string           // The scenario name.
		diffChars int              // The diff character count.
		truncated bool             // Whether the diff was truncated.
		effort    string           // The configured reasoning effort.
		res       tscommit.Message // The generation result metadata.
		want      string           // The expected summary line.
	}{
		{
			name:      "full metadata without truncation",
			diffChars: 1420,
			truncated: false,
			effort:    "high",
			res:       tscommit.Message{Model: "google/gemini-4-hydrogen", TotalTokens: 320, Cost: 0.0025},
			want:      "Diff: 1420 chars | Model: google/gemini-4-hydrogen (high) | Tokens: 320 | Cost: $0.0025",
		},
		{
			name:      "truncated diff adds marker",
			diffChars: 100000,
			truncated: true,
			effort:    "low",
			res:       tscommit.Message{Model: "openai/gpt-5", TotalTokens: 1500, Cost: 0.015},
			want:      "Diff: 100000 (Truncated) chars | Model: openai/gpt-5 (low) | Tokens: 1500 | Cost: $0.015",
		},
		{
			name:      "unreported metadata uses n/a and zero cost without scientific notation",
			diffChars: 42,
			truncated: false,
			effort:    "",
			res:       tscommit.Message{},
			want:      "Diff: 42 chars | Model: n/a (n/a) | Tokens: 0 | Cost: $0",
		},
		{
			name:      "empty effort falls back to n/a after the model",
			diffChars: 80,
			truncated: false,
			effort:    "",
			res:       tscommit.Message{Model: "m1", TotalTokens: 12, Cost: 0.000042},
			want:      "Diff: 80 chars | Model: m1 (n/a) | Tokens: 12 | Cost: $0.000042",
		},
		{
			name:      "sub-cent micro cost avoids scientific notation",
			diffChars: 80,
			truncated: false,
			effort:    "medium",
			res:       tscommit.Message{Model: "m1", TotalTokens: 12, Cost: 0.000042},
			want:      "Diff: 80 chars | Model: m1 (medium) | Tokens: 12 | Cost: $0.000042",
		},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Render the summary line under test.
			got := cli.FormatSummary(cli.PromptParams{
				Message:   tc.res,
				Effort:    tc.effort,
				DiffChars: tc.diffChars,
				Truncated: tc.truncated,
			})
			// Compare the rendered summary against the expected string.
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "formatSummary", Actual: got, Want: tc.want}))
			}
		})
	}
}

// TestNewPrompter tests that newPrompter builds a prompter carrying the CLI
// tool name shown to the user in prompts and error messages.
func TestNewPrompter(t *testing.T) {
	// Create the prompter.
	p := cli.NewPrompter()
	// If the prompter is nil, fail the test.
	if p == nil {
		t.Error(tserr.NilFailed("newPrompter"))
		return
	}
	// If the prompter name is not the CLI name, fail the test.
	if p.Name != "tscommit" {
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "prompter name", Actual: p.Name, Want: "tscommit"}))
	}
}

// TestWarnBreakingChange tests that warnBreakingChange prints the warning
// only when msg carries a Conventional Commits breaking-change marker.
func TestWarnBreakingChange(t *testing.T) {
	// Test cases.
	cases := []struct {
		name string
		msg  string
		want string // The expected prompter output; empty when no warning.
	}{
		{"bang marker", "feat!: drop the v1 api", cli.BreakingWarning},
		{"bang marker with scope", "feat(api)!: remove the legacy field", cli.BreakingWarning},
		{"breaking change footer", "feat: rework storage\n\nBREAKING CHANGE: the layout changed", cli.BreakingWarning},
		{"breaking-change footer", "feat: rework storage\n\nBREAKING-CHANGE: the layout changed", cli.BreakingWarning},
		{"plain message silent", "feat: add the thing", ""},
		// An exclamation mark in the description is not a marker.
		{"exclamation in description silent", "feat: wow! neat", ""},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a prompter whose output is captured in a buffer.
			p := lpcli.NewPrompter("tscommit")
			// Capture the prompter output.
			var buf bytes.Buffer
			// Route the prompter output into the buffer. If that fails, fail
			// the test.
			if err := p.SetOut(&buf); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "SetOut", Err: err}))
				return
			}
			// Call the function under test.
			cli.WarnBreakingChange(p, tc.msg)
			// If the output differs from the expectation, fail the test.
			if got := buf.String(); got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "warnBreakingChange output", Actual: got, Want: tc.want}))
			}
		})
	}
}

// errFake simulates an editor failure for the editMessage seam.
var errFake = errors.New("editor failed") // The fake editor failure.

// TestEditInEditor tests that editInEditor returns the editor's content
// unchanged and that editor failures and empty edits are rejected. It swaps
// the editMessage seam so no real editor runs; it must not use t.Parallel.
func TestEditInEditor(t *testing.T) {
	// The initial text the fake editor must receive.
	const initial = "feat(x): xyz"
	// Test cases.
	cases := []struct {
		name    string // The name of the test case.
		edited  string // The content the fake editor returns.
		editErr error  // The error the fake editor returns, if any.
		want    string // The expected returned message.
		wantErr bool   // Whether an error is expected.
	}{
		{"edit returned unchanged", "feat(x): xyz\n\nBody text.", nil, "feat(x): xyz\n\nBody text.", false},
		{"editor error propagates", "", errFake, "", true},
		{"empty edit rejected", "", nil, "", true},
		{"whitespace-only edit rejected", " \t\n", nil, "", true},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Swap the editor seam and restore it after the test.
			t.Cleanup(cli.SetEditMessage(func(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
				// The initial text must reach the editor verbatim.
				if initialText != initial {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "initialText", Actual: initialText, Want: initial}))
				}
				// Return the case's canned result.
				return tc.edited, tc.editErr
			}))
			// Call the function under test.
			got, err := cli.EditInEditor(context.Background(), lpcli.NewPrompter("tscommit"), initial)
			// Check the error expectation and the returned message.
			if tc.wantErr {
				// An error is expected: fail if it is missing or a value leaked.
				if err == nil {
					t.Error(tserr.NilFailed("editInEditor"))
				}
				if got != "" {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editInEditor", Actual: fmt.Sprintf("%q", got), Want: `""`}))
				}
				return
			}
			// No error expected: fail on any error or wrong message.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "editInEditor", Err: err}))
			}
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "editInEditor", Actual: got, Want: tc.want}))
			}
		})
	}
}

// TestCheckEditedValid tests that a non-empty edit is returned unchanged.
func TestCheckEditedValid(t *testing.T) {
	// Test cases.
	cases := []struct {
		name   string
		edited string
	}{
		{"simple message", "feat(x): xyz"},
		{"trailing newline", "feat(x): xyz\n"},
		{"single character", "x"},
		// The edit is authoritative: surrounding whitespace is preserved,
		// never trimmed or rewritten.
		{"surrounding whitespace preserved", " feat(x): xyz "},
		// Deliberately NOT a valid Conventional Commits message — user edits
		// are not re-validated (design decision).
		{"invalid format passes through", "just some text"},
		// A multi-line header-plus-body edit is the common real-world editor
		// output and is returned unchanged.
		{"multi-line with body", "feat: add thing\n\nBody text."},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test.
			got, err := cli.CheckEdited(tc.edited)
			// If the function returns an error, fail the test.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "checkEdited", Err: err}))
				return
			}
			// If the returned message differs from the input, fail the test.
			if got != tc.edited {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "checkEdited", Actual: got, Want: tc.edited}))
			}
		})
	}
}

// TestCheckEditedInvalid tests that an edit which is empty after trimming
// is rejected and returns an empty string.
func TestCheckEditedInvalid(t *testing.T) {
	// Test cases.
	cases := []struct {
		name   string
		edited string
	}{
		{"empty", ""},
		{"spaces only", "   "},
		{"tabs and newlines only", "\t\n \n"},
		// strings.TrimSpace trims Unicode whitespace (unicode.IsSpace),
		// including U+00A0 NO-BREAK SPACE, so this is empty after trimming.
		{"non-breaking space only", "\u00a0"},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test.
			got, err := cli.CheckEdited(tc.edited)
			// If the function returns no error, fail the test.
			if err == nil {
				t.Error(tserr.NilFailed("checkEdited"))
			}
			// If the function returns a non-empty string, fail the test.
			if got != "" {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "checkEdited", Actual: fmt.Sprintf("%q", got), Want: `""`}))
			}
		})
	}
}
