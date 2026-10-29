// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"context"     // Import the context package for context handling.
	"crypto/rand" // Import the rand package for cryptographic randomness.
	"errors"      // Import the errors package for error construction.
	"strings"     // Import the strings package for string manipulation.

	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
	"github.com/thorsphere/tserr"      // Import the tserr package for error handling.
)

// maxGenAttempts caps provider calls per message: 1 initial + retries with feedback.
const maxGenAttempts = 2

// completeChat sends req to client and returns the normalized result. It is a
// package-level seam so tests can swap out the external AI call.
var completeChat = func(ctx context.Context, client *tsaiclient.Client, req *tsaiclient.Request) (tsaiclient.Result, error) {
	// Complete the request exactly once; retries are the caller's policy.
	return client.Complete(ctx, req)
}

// genMessage calls the configured AI client to produce a commit message for
// diff and returns it. It attempts to generate a message up to maxGenAttempts
// times, feeding validation and truncation failures back to the model between
// attempts. The sessionID is attached to every request so retries and repeated
// invocations for the same diff share one provider endpoint and prompt cache.
// It returns an error if all attempts fail.
func genMessage(ctx context.Context, client *tsaiclient.Client, diff string, sessionID string) (string, error) {
	// Build prompt
	prompt := buildPrompt(diff)
	// Attempt generation up to maxGenAttempts times.
	var lastErr error
	for attempt := 1; attempt <= maxGenAttempts; attempt++ {
		// Build the completion request for the current prompt.
		req, err := tsaiclient.NewRequest(prompt)
		// Return the error if building the request fails.
		if err != nil {
			return "", tserr.Op(&tserr.OpArgs{Op: "new request", Fn: "AI Client", Err: err})
		}
		// Attach the sticky routing key shared by every attempt and regeneration.
		req = req.WithSessionID(sessionID)
		// Call the provider to produce a commit message for diff.
		res, err := completeChat(ctx, client, req)
		// Return the error if the provider call fails.
		if err != nil {
			return "", err
		}
		// Clean wrapper formatting the model may have added around the message.
		content := cleanMessage(res.Content)
		// Return an error when nothing remains after cleaning; not retryable.
		if content == "" {
			return "", tserr.Empty("AI client commit message")
		}
		// Validate the generated message.
		validateErr := validateMessage(content)
		// Accept the message if it validates and was not truncated.
		if validateErr == nil && !res.Truncated {
			return content, nil
		}
		// Build feedback from the validation failure and/or truncation.
		var feedback string
		// Report validation failure as a failure when the message is invalid.
		if validateErr != nil {
			feedback = "\n\nYour previous output was invalid: " + validateErr.Error()
		}
		// Explain truncation so the retry stays under the token limit.
		if res.Truncated {
			feedback += "\nYour previous output was cut off; be significantly more concise."
		}
		// Record the cause: prefer the validation error, fall back to truncation.
		lastErr = validateErr
		// Fall back to the truncation cause when the message itself validated.
		if lastErr == nil {
			lastErr = errors.New("output was truncated by the token limit")
		}
		// If attempts remain, ask the provider to fix its previous output.
		if attempt < maxGenAttempts {
			// Build corrective feedback for the retry prompt, including the
			// previous output and the generated message.
			prompt = prompt +
				feedback +
				"\nPrevious output:\n" + content +
				"\nRegenerate the commit message following STRICT FORMAT RULES exactly."
		}
	}
	// Return an error when all attempts produced invalid messages.
	return "", tserr.Op(&tserr.OpArgs{Op: "generate message", Fn: "AI client", Err: lastErr})
}

// cleanMessage removes markdown code fences, backticks, and extra wrapper formatting.
func cleanMessage(s string) string {
	// Trim leading and trailing whitespace
	s = strings.TrimSpace(s)
	// Strip outer markdown code block fences (e.g. ```commit ... ``` or ``` ...)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence and optional language tag up to newline
		if idx := strings.Index(s, "\n"); idx != -1 {
			// Remove opening fence and optional language tag up to newline
			s = s[idx+1:]
		} else {
			// Remove opening fence, if no newline
			s = strings.TrimPrefix(s, "```")
		}
		// Remove trailing fence
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	// Strip outer single or double quotes / backticks if wrapped
	s = strings.Trim(strings.TrimSpace(s), "`\"'")
	// Return trimmed string
	return strings.TrimSpace(s)
}

// newSessionID returns a fresh random identifier used as the sticky routing
// key for one generation session.
func newSessionID() string {
	// Return a cryptographically random text identifier.
	return rand.Text()
}
