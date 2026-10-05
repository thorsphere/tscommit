// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

import (
	"regexp"
	"strings"

	"github.com/thorsphere/tserr"
)

// headerRe matches a Conventional Commits header line with optional scope
// and optional breaking-change marker.
var headerRe = regexp.MustCompile(
	`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore)(\([a-z0-9._/-]+\))?!?: \S.*[^.]$`)

// validateMessage checks that msg conforms to the strict format rules.
// It returns a descriptive error suitable for feeding back to the model.
func validateMessage(msg string) error {
	// Split the message into header and body lines.
	header, body, hasBody := strings.Cut(msg, "\n")
	// Check that the header line matches the expected format.
	if !headerRe.MatchString(header) {
		return tserr.InvalidFormat(&tserr.InvalidFormatArgs{
			F:      "commit message header",
			Value:  header,
			Detail: "must match <type>[scope]!: <imperative description> with no trailing period",
		})
	}
	// Check that the header is not too long.
	if len(header) > 72 {
		return tserr.Lower(&tserr.LowerArgs{
			Var: "header length", Actual: int64(len(header)), HigherBound: 72,
		})
	}
	// Check that the body is not empty.
	if hasBody {
		// Trim the leading blank line from the body.
		rest := strings.TrimPrefix(body, "\n")
		// Check that the body is separated by exactly one blank line.
		if rest == body {
			return tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "commit message",
				Detail: "header and body must be separated by exactly one blank line",
			})
		}
	}
	// Return no error to indicate success.
	return nil
}

// hasBreakingChange reports whether msg declares a breaking change via the
// "!" header marker or a "BREAKING CHANGE:" footer.
func hasBreakingChange(msg string) bool {
	// Split the message into header and body lines.
	header, _, _ := strings.Cut(msg, "\n")
	// Check if the header contains the breaking change marker.
	return strings.Contains(header, "!") || strings.Contains(msg, "BREAKING CHANGE:")
}
