// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"fmt"     // fmt
	"testing" // testing

	"github.com/thorsphere/tscommit" // tscommit
	"github.com/thorsphere/tserr"    // tserr
)

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
