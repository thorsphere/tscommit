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
	"github.com/thorsphere/tscommit/internal/cli" // Import the cli package under test.
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
			got, err := cli.PromptUser(context.Background(), p, "feat(x): xyz", 0.92, true)
			// If prompting failed, fail the test.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// If the returned choice differs from the expectation, fail the test.
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptUser", Actual: got, Want: tc.want}))
			}
			// The candidate message must be visible to the user.
			if !strings.Contains(buf.String(), "feat(x): xyz") {
				t.Errorf("output %q does not show the candidate message", buf.String())
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
			_, err := cli.PromptUser(context.Background(), p, tc.msg, 0.85, true)
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
			got, err := cli.PromptUser(tc.ctx, p, "feat(x): xyz", 0.9, true)
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

// TestPromptUserConfidence tests that promptUser renders the confidence line
// after the message and before any warning or prompt when showConfidence is
// true, and omits it when showConfidence is false (e.g. after user edit).
func TestPromptUserConfidence(t *testing.T) {
	// Test cases.
	cases := []struct {
		name           string  // The scenario name.
		confidence     float64 // The confidence score supplied to promptUser.
		showConfidence bool    // Whether confidence display is enabled.
		wantLine       string  // The expected rendered confidence line; empty when omitted.
	}{
		{"shown as rounded percentage", 0.924, true, "Confidence: 92%"},
		{"shown at zero boundary", 0, true, "Confidence: 0%"},
		{"shown at max boundary", 1.0, true, "Confidence: 100%"},
		{"omitted when disabled", 0.92, false, ""},
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
			// Execute promptUser under test.
			if _, err := cli.PromptUser(context.Background(), p, msg, tc.confidence, tc.showConfidence); err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// Inspect the captured output.
			got := buf.String()
			// When omitted, no Confidence line may appear.
			if tc.wantLine == "" {
				// Fail if a confidence line leaked into the output.
				if strings.Contains(got, "Confidence:") {
					t.Error(tserr.InvalidFormat(&tserr.InvalidFormatArgs{
						F:      "promptUser output",
						Value:  got,
						Detail: "unexpected confidence line when showConfidence is false",
					}))
				}
				return
			}
			// Locate message, confidence line, and warning in the output.
			mi := strings.Index(got, msg)
			ci := strings.Index(got, tc.wantLine)
			wi := strings.Index(got, cli.BreakingWarning)
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
