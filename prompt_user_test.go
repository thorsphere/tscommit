// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"bytes"   // Import the bytes package for capturing prompter output.
	"context" // Import the context package for context handling.
	"errors"  // Import the errors package for creating a fake error.
	"fmt"     // Import the fmt package for formatting quoted values in failures.
	"strings" // Import the strings package for string manipulation.
	"testing" // Import the testing package for table-driven tests.

	"github.com/thorsphere/lpcli"    // Import the lpcli package for interactive prompting.
	"github.com/thorsphere/tscommit" // Import the tscommit package under test.
	"github.com/thorsphere/tserr"    // Import the tserr package for error reporting.
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
		{"default key", "y\n", tscommit.ChoiceAccept, false},
		{"default alias", "yes\n", tscommit.ChoiceAccept, false},
		{"enter selects default", "\n", tscommit.ChoiceAccept, false},
		{"edit key", "e\n", tscommit.ChoiceEdit, false},
		{"edit alias", "edit\n", tscommit.ChoiceEdit, false},
		{"regenerate key", "r\n", tscommit.ChoiceRegenerate, false},
		{"regenerate alias", "regen\n", tscommit.ChoiceRegenerate, false},
		{"cancel key", "n\n", tscommit.ChoiceCancel, false},
		{"cancel alias", "q\n", tscommit.ChoiceCancel, false},
		// Matching is case-insensitive.
		{"uppercase key", "E\n", tscommit.ChoiceEdit, false},
		// Unknown input is rejected and the prompt is asked again.
		{"unknown key retries", "x\ny\n", tscommit.ChoiceAccept, true},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the prompter with canned input and captured output.
			p := tscommit.NewPrompter()
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
			got, err := tscommit.PromptUser(context.Background(), p, "feat: xyz")
			// If prompting failed, fail the test.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// If the returned choice differs from the expectation, fail the test.
			if got != tc.want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptUser", Actual: got, Want: tc.want}))
			}
			// If the retry expectation does not match the output, fail the test.
			if retry := strings.Contains(buf.String(), "Unknown option"); retry != tc.wantRetry {
				t.Errorf("retry message present = %v, want %v", retry, tc.wantRetry)
			}
		})
	}
}

// TestPromptUserWarns tests that promptUser prints the breaking-change
// warning before the prompt when msg carries a breaking-change marker, and
// stays silent otherwise.
func TestPromptUserWarns(t *testing.T) {
	// Test cases.
	cases := []struct {
		name string
		msg  string
		want string // The expected output prefix; empty when no warning.
	}{
		{"bang marker warns", "feat!: drop the v1 api", tscommit.BreakingWarning},
		{"footer warns", "feat: rework storage\n\nBREAKING CHANGE: the layout changed", tscommit.BreakingWarning},
		{"plain message silent", "feat: add the thing", ""},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the prompter with canned input and captured output.
			p := tscommit.NewPrompter()
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
			_, err := tscommit.PromptUser(context.Background(), p, tc.msg)
			// If prompting failed, fail the test.
			if err != nil {
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "promptUser", Err: err}))
				return
			}
			// Check the warning against the captured output.
			got := buf.String()
			// If a warning is expected, it must lead the output.
			if tc.want != "" && !strings.HasPrefix(got, tc.want) {
				t.Errorf("output %q does not start with the breaking-change warning", got)
			}
			// If no warning is expected, the output must not contain one.
			if tc.want == "" && strings.Contains(got, "BREAKING CHANGE detected") {
				t.Errorf("output %q contains an unexpected breaking-change warning", got)
			}
		})
	}
}

// TestPromptUserErrors tests that promptUser returns choiceCancel with an
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
			p := tscommit.NewPrompter()
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
			got, err := tscommit.PromptUser(tc.ctx, p, "feat: xyz")
			// If no error was returned, fail the test.
			if err == nil {
				t.Error(tserr.NilFailed("promptUser"))
			}
			// The failure must report the cancel choice, never accept.
			if got != tscommit.ChoiceCancel {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "promptUser", Actual: got, Want: tscommit.ChoiceCancel}))
			}
		})
	}
}

// TestNewPrompter tests that newPrompter builds a prompter carrying the CLI
// tool name shown to the user in prompts and error messages.
func TestNewPrompter(t *testing.T) {
	// Create the prompter.
	p := tscommit.NewPrompter()
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
		{"bang marker", "feat!: drop the v1 api", tscommit.BreakingWarning},
		{"bang marker with scope", "feat(api)!: remove the legacy field", tscommit.BreakingWarning},
		{"breaking change footer", "feat: rework storage\n\nBREAKING CHANGE: the layout changed", tscommit.BreakingWarning},
		{"breaking-change footer", "feat: rework storage\n\nBREAKING-CHANGE: the layout changed", tscommit.BreakingWarning},
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
			tscommit.WarnBreakingChange(p, tc.msg)
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
	const initial = "feat: xyz"
	// Test cases.
	cases := []struct {
		name    string // The name of the test case.
		edited  string // The content the fake editor returns.
		editErr error  // The error the fake editor returns, if any.
		want    string // The expected returned message.
		wantErr bool   // Whether an error is expected.
	}{
		{"edit returned unchanged", "feat: xyz\n\nBody text.", nil, "feat: xyz\n\nBody text.", false},
		{"editor error propagates", "", errFake, "", true},
		{"empty edit rejected", "", nil, "", true},
		{"whitespace-only edit rejected", " \t\n", nil, "", true},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Swap the editor seam and restore it after the test.
			t.Cleanup(tscommit.SetEditMessage(func(ctx context.Context, p *lpcli.Prompter, initialText string) (string, error) {
				// The initial text must reach the editor verbatim.
				if initialText != initial {
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "initialText", Actual: initialText, Want: initial}))
				}
				// Return the case's canned result.
				return tc.edited, tc.editErr
			}))
			// Call the function under test.
			got, err := tscommit.EditInEditor(context.Background(), lpcli.NewPrompter("tscommit"), initial)
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
		{"simple message", "feat: xyz"},
		{"trailing newline", "feat: xyz\n"},
		{"single character", "x"},
		// The edit is authoritative: surrounding whitespace is preserved,
		// never trimmed or rewritten.
		{"surrounding whitespace preserved", " feat: xyz "},
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
			got, err := tscommit.CheckEdited(tc.edited)
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
			got, err := tscommit.CheckEdited(tc.edited)
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
