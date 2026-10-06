// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"fmt"
	"strings"
	"testing"

	"github.com/thorsphere/tscommit"
	"github.com/thorsphere/tserr"
)

// header72 is exactly 72 bytes: "feat: " + 66 "a"s.
var header72 = "feat: " + strings.Repeat("a", 66)

// TestValidateMessageValid tests the ValidateMessage function with valid messages
// It checks that the function returns nil for each message in the slice
func TestValidateMessageValid(t *testing.T) {
	msgs := []string{
		"feat: xyz",     // single-character description
		"feat: xyz\n",   // git-style trailing newline
		"feat: xyz\n\n", // stray extra trailing newline
		"fix: correct off-by-one in diff truncate",  // plain header
		"feat(git): add staged-diff truncation",     // scoped header
		"feat(api)!: remove legacy config keys",     // scoped breaking marker
		"chore!: drop Go 1.20 support",              // unscoped breaking marker
		"refactor(a-b/c.d): tidy scope charset",     // full scope charset
		header72,                                    // header at the length limit
		"feat: " + strings.Repeat("é", 33),          // 72 bytes — length counts bytes
		"feat: add thing\n\nExplain WHAT and WHY.",  // body after one blank line
		"feat: add thing\n\nBody line.\n\nRefs #42", // body plus footer
		"feat: add thing\n\nBody.\n\nBREAKING CHANGE: config keys renamed",
	}
	// for each message in the slice, call the function
	for _, msg := range msgs {
		// if validateMessage returns an error, the test fails
		if err := tscommit.ValidateMessage(msg); err != nil {
			t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "validateMessage", Err: err}))
		}
	}
}

// TestValidateMessageInvalid tests the ValidateMessage function with invalid messages
// The test fails if the function returns nil for any message in the slice
func TestValidateMessageInvalid(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"empty", ""},
		{"missing space after colon", "feat:x"},
		{"no description", "feat: "},
		{"unknown type", "wip: x"},
		{"uppercase type", "FEAT: x"},
		{"trailing period", "feat: xyz."},
		{"trailing space", "feat: add thing "},
		{"trailing space short", "feat: x "},
		{"uppercase scope", "feat(Upper): x"},
		{"scope with space", "feat(with space): x"},
		{"missing blank line", "feat: x\nbody"},
		{"header too long", "feat: " + strings.Repeat("a", 67)}, // 73 bytes
		{"no blank line", "feat: x\nbody"},
		{"two blank lines", "feat: x\n\n\nbody"},
		{"three blank lines", "feat: x\n\n\n\nbody"},
		{"single-char description", "feat: x"}, // min-3-char rule
		{"two-char description", "feat: ab"},   // min-3-char rule
		{"trailing space", "feat: add thing "}, // [^.\s]$ rule
		{"trailing tab", "feat: add thing\t"},  // \s covers tabs too
		// 73 bytes but only 68 runes — must be rejected because length is bytes.
		{"byte-length limit", "feat: " + strings.Repeat("é", 67)}, // 6 + 134 = 140 bytes
	}
	// for each test case in the slice, call the function
	for _, tc := range cases {
		// if validateMessage returns nil, the test fails
		if err := tscommit.ValidateMessage(tc.msg); err == nil {
			t.Error(tserr.NilFailed("validateMessage"))
		}
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
		{"plain header marker", "chore!: drop Go 1.20 support", true},
		{"footer", "feat: x\n\nBREAKING CHANGE: config keys renamed", true},
		{"no marker", "fix: handle empty diff", false},
		{"exclamation in description", "feat: support wow!", false},
		{"footer text mid-prose", "feat: x\n\nWe avoided BREAKING CHANGE: here", false},
		{"hyphen footer", "feat: x\n\nBREAKING-CHANGE: renamed", true},
		{"empty footer", "feat: x\n\nBREAKING CHANGE:", true},
		{"marker in description", "feat: document the !: marker", false},
	}
	// for each test case in the slice, call the function
	for _, tc := range cases {
		// if HasBreakingChange returns the expected value, the test passes
		if got := tscommit.HasBreakingChange(tc.msg); got != tc.want {
			t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "HasBreakingChange", Actual: fmt.Sprintf("%v", got), Want: fmt.Sprintf("%v", tc.want)}))
		}
	}
}
