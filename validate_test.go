// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	// Import the fmt package for formatting.
	"strings" // Import the strings package for string handling.
	"testing" // Import the testing package for testing.

	"github.com/thorsphere/tscommit" // Import the tscommit package for testing.
	"github.com/thorsphere/tserr"    // Import the tserr package for error handling.
)

// header72 is exactly 72 characters: "feat(x): " + 63 "a"s.
var header72 = "feat(x): " + strings.Repeat("a", 63)

// TestValidateMessageValid tests the ValidateMessage function with valid messages
// It checks that the function returns nil for each message in the slice
func TestValidateMessageValid(t *testing.T) {
	msgs := []string{
		"feat(x): xyz",     // minimum-3-char boundary description
		"feat(x): xyz\n",   // git-style trailing newline
		"feat(x): xyz\n\n", // stray extra trailing newline
		"fix(x): correct off-by-one in diff truncate",  // plain header
		"feat(git): add staged-diff truncation",        // scoped header
		"feat(api)!: remove legacy config keys",        // scoped breaking marker
		"refactor(a-b/c.d): tidy scope charset",        // full scope charset
		header72,                                       // header at the length limit
		"feat(x): " + strings.Repeat("é", 63),          // 72 characters — length counts characters
		"feat(x): add thing\n\nExplain WHAT and WHY.",  // body after one blank line
		"feat(x): add thing\n\nBody line.\n\nRefs #42", // body plus footer
		"feat(x): add thing\n\nBody.\n\nBREAKING CHANGE: config keys renamed",
		"feat(x): ééé", // 3 runes — min rule counts characters, not bytes
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
		{"missing space after colon", "feat(x):x"},
		{"no description", "feat(x): "},
		{"unknown type", "wip(x): x"},
		{"uppercase type", "FEAT(x): x"},
		{"trailing period", "feat(x): xyz."},
		{"trailing space", "feat(x): add thing "},
		{"trailing space short", "feat(x): x "},
		{"uppercase scope", "feat(Upper): x"},
		{"scope with space", "feat(with space): x"},
		{"no blank line", "feat(x): x\nbody"},
		{"three blank lines", "feat(x): xyz\n\n\n\nbody"},          // more than one blank line between header and body
		{"single-char description", "feat(x): x"},                  // min-3-char rule
		{"two-char description", "feat(x): ab"},                    // min-3-char rule
		{"trailing space", "feat(x): add thing "},                  // [^.\s]$ rule
		{"trailing tab", "feat(x): add thing\t"},                   // \s covers tabs too
		{"missing blank line", "feat(x): add thing\nbody"},         // missing blank line between header and body
		{"two blank lines", "feat(x): add thing\n\n\nbody"},        // more than one blank line between header and body
		{"three blank lines", "feat(x): add thing\n\n\n\nbody"},    // same second branch, extra blank line
		{"two-rune multibyte description", "feat(x): éé"},          // 2 runes — min-3 rule
		{"header too long", "feat(x): " + strings.Repeat("a", 64)}, // 73 characters
		// 73 characters (137 bytes) — rejected because length counts characters.
		{"character-length limit", "feat(x): " + strings.Repeat("é", 64)}, // 73 characters
		{"missing scope", "feat: xyz"},                                    // missing required scope
		{"empty scope", "feat(): xyz"},                                    // the charset class requires ≥1 char
	}
	// for each test case in the slice, call the function
	for _, tc := range cases {
		// if validateMessage returns nil, the test fails
		if err := tscommit.ValidateMessage(tc.msg); err == nil {
			t.Error(tserr.NilFailed("validateMessage"))
		}
	}
}
