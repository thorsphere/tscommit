// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"strings" // Import the strings package for checking template contents.
	"testing" // Import the testing package for table-driven tests.

	"github.com/thorsphere/tscommit" // Import the tscommit package under test.
	"github.com/thorsphere/tserr"    // Import the tserr package for error reporting.
)

// TestBuildPrompt tests that buildPrompt appends the diff verbatim after the
// prompt template, without trimming or truncation.
func TestBuildPrompt(t *testing.T) {
	// Test cases.
	cases := []struct {
		name string
		diff string
	}{
		{"empty diff", ""},
		{"simple diff", "diff --git a/x.go b/x.go\n"},
		{"diff with trailing newline", "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n"},
		// The diff is appended verbatim: whitespace and unicode survive.
		{"diff with unicode and spaces", "  +fmt.Println(\"héllo\")  \n"},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test.
			got := tscommit.BuildPrompt(tc.diff)
			// The prompt must be the template followed by the diff verbatim.
			if want := tscommit.PromptTemplate + tc.diff; got != want {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "buildPrompt", Actual: got, Want: want}))
			}
		})
	}
}

// TestPromptTemplateContract tests that the prompt template states the same
// format rules the validator enforces. The commit format is deliberately
// specified in both prompt_ai.go and validate.go (AGENTS.md); this test makes
// the coupled facts fail loudly when one side changes alone.
func TestPromptTemplateContract(t *testing.T) {
	// The coupled facts: each must appear in the prompt template.
	cases := []struct {
		name string
		want string
	}{
		{"type list", "feat, fix, docs, style, refactor, perf, test, build, ci, chore"},
		{"header length limit", "Max 72 characters"},
		{"no trailing period", "no trailing period"},
		{"blank line rule", "exactly one empty line"},
		{"breaking marker", `"!" after type/scope`},
		{"breaking footer spelling", "BREAKING CHANGE: "},
		{"description minimum", "short description"},
		{"scope rule", "<type>(<scope>)[!]: <short description>"},
		{"single JSON object rule", "ONLY a single JSON object"},
		{"envelope findings key", `"findings":`},
		{"envelope confidence key", `"confidence":`},
		{"confidence scale", "from 0.0 to 1.0"},
		{"confidence rubric high tier", "0.85-1.00"},
		{"confidence rubric medium tier", "0.65-0.84"},
		{"confidence rubric low tier", "0.40-0.64"},
		{"confidence rubric lowest tier", "0.00-0.39"},
	}
	// Run the test cases.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// If the template no longer states the rule, fail the test.
			if !strings.Contains(tscommit.PromptTemplate, tc.want) {
				t.Error(tserr.NotFound("prompt template rule " + tc.want))
			}
		})
	}
}
