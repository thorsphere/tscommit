// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"regexp"  // Import the regexp package for regular expressions.
	"strings" // Import the strings package for string manipulation.

	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

// headerRe matches a Conventional Commits header line with optional scope
// and optional breaking-change marker.
var headerRe = regexp.MustCompile(
	`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore)(\([a-z0-9._/-]+\))?!?: \S(.*[^.\s])?$`)

// breakingFooterRe matches a "BREAKING CHANGE:" or "BREAKING-CHANGE:" footer
// line (anchored to the start of a line).
var breakingFooterRe = regexp.MustCompile(`(?m)^BREAKING[- ]CHANGE:`)

// validateMessage checks that msg conforms to the strict format rules and
// returns a descriptive error suitable for feeding back to the model.
// Trailing newlines are ignored (git convention), so a header-only message
// is not mistaken for a missing blank-line separator.
func validateMessage(msg string) error {
	// Ignore trailing newlines (git convention) so a header-only message
	// is not mistaken for a missing blank-line separator.
	msg = strings.TrimRight(msg, "\n")
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
	// Check that the description is at least 3 characters long.
	if len(header)-strings.Index(header, ": ")-2 < 3 {
		return tserr.InvalidFormat(&tserr.InvalidFormatArgs{
			F:      "commit message header",
			Value:  header,
			Detail: "description must be at least 3 characters",
		})
	}
	// Check that the header is not too long. Count bytes, not runes:
	// git and commitlint measure header length in bytes, so we do too.
	if len(header) > 72 {
		return tserr.Lower(&tserr.LowerArgs{
			Var: "header length", Actual: int64(len(header)), HigherBound: 72,
		})
	}
	// Check that the body is separated from the header.
	if hasBody {
		// Trim the leading blank line from the body.
		rest := strings.TrimPrefix(body, "\n")
		// Check that the header and body are separated by exactly one blank line.
		switch {
		case rest == body:
			// No blank line between header and body.
			return tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "commit message",
				Detail: "missing blank line between header and body",
			})
		case strings.HasPrefix(rest, "\n"):
			// More than one blank line between header and body.
			return tserr.InvalidFormat(&tserr.InvalidFormatArgs{
				F:      "commit message",
				Detail: "more than one blank line between header and body",
			})
		}
	}
	// Return no error to indicate success.
	return nil
}

// hasBreakingChange reports whether msg declares a breaking change via the
// "!" header marker or a "BREAKING CHANGE:" footer.
// It ignores trailing newlines (git convention), matching validateMessage.
func hasBreakingChange(msg string) bool {
	// Ignore trailing newlines (git convention), matching validateMessage.
	msg = strings.TrimRight(msg, "\n")
	// Split the message into header and footer lines.
	header, _, _ := strings.Cut(msg, "\n")
	// Check for the breaking-change marker: a "!" immediately before the
	// ": " separator (e.g. "feat!: x" or "feat(api)!: x"). This ignores
	// exclamation marks elsewhere in the description.
	if i := strings.Index(header, ": "); strings.HasSuffix(header[:max(i, 0)], "!") {
		return true
	}
	// Otherwise check for a "BREAKING CHANGE:" footer line.
	return breakingFooterRe.MatchString(msg)
}
