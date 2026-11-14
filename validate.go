// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"regexp"       // Import the regexp package for regular expressions.
	"strings"      // Import the strings package for string manipulation.
	"unicode/utf8" // Import the utf8 package to count runes in the description.

	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

// headerRe matches a Conventional Commits header line with a mandatory
// scope and an optional breaking-change marker.
var headerRe = regexp.MustCompile(
	`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore)(\([a-z0-9._/-]+\))!?: \S(.*[^.\s])?$`)

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
			Detail: "must match <type>(<scope>)[!]: <imperative description> with no trailing period",
		})
	}
	// Extract the description: everything after the ": " separator. The
	// separator is guaranteed to exist because the header matched headerRe.
	desc := header[strings.Index(header, ": ")+2:]
	// Check that the description is at least 3 characters long. Count runes,
	// not bytes, so multibyte text is measured as a reader counts it.
	if utf8.RuneCountInString(desc) < 3 {
		return tserr.InvalidFormat(&tserr.InvalidFormatArgs{
			F:      "commit message header",
			Value:  header,
			Detail: "description must be at least 3 characters",
		})
	}
	// Check that the header is not too long. Count characters, not bytes,
	// matching the git/commitlint convention of measuring display length.
	if n := utf8.RuneCountInString(header); n > 72 {
		return tserr.Lower(&tserr.LowerArgs{
			Var: "header length", Actual: int64(n), HigherBound: 72,
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
