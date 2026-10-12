// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"bytes"         // Import the bytes package for byte slices.
	"context"       // Import the context package for context handling.
	"encoding/json" // Import the json package for JSON encoding/decoding.
	"errors"        // Import the errors package for error construction.
	"io"            // Import the io package for input/output.
	"net/http"      // Import the http package for HTTP requests.
	"strings"       // Import the strings package for string manipulation.
	"time"          // Import the time package for time handling.

	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

// defaultHTTPClient is the HTTP client used for provider chat requests,
// bounded by a 60-second timeout.
var defaultHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

// message is a single chat message exchanged with the provider.
type message struct {
	Role    string `json:"role"`    // The conversational role, e.g. "user" or "assistant".
	Content string `json:"content"` // The message text.
}

// requestPayload is the JSON body sent to the provider chat endpoint.
type requestPayload struct {
	Model               string    `json:"model"`                           // The model identifier for the request.
	Messages            []message `json:"messages"`                        // The conversation messages, oldest first.
	MaxCompletionTokens int       `json:"max_completion_tokens,omitempty"` // The maximum number of tokens the model may generate; omitted when zero.
	Temperature         float64   `json:"temperature,omitempty"`           // The sampling temperature; omitted when zero.
}

// responsePayload is the provider chat completion response body. The zero
// value means no completion was returned.
type responsePayload struct {
	Choices []struct {
		Message      message `json:"message"`       // The generated message for this choice.
		FinishReason string  `json:"finish_reason"` // Why generation stopped, e.g. "stop" or "length".
	} `json:"choices"` // The completion choices returned by the model.
}

// apiErrorResponse is the provider error body returned for non-2xx responses.
type apiErrorResponse struct {
	Error struct {
		Message string      `json:"message"` // The human-readable error message.
		Code    interface{} `json:"code"`    // The provider-specific error code.
	} `json:"error"` // The error details.
}

// maxGenAttempts caps provider calls per message: 1 initial + retries with feedback.
const maxGenAttempts = 2

// finishReasonLength is the provider finish reason indicating the generated
// message was truncated by the token limit.
const finishReasonLength = "length"

// finishReasonContentFilter is the provider finish reason indicating the
// response was blocked by the provider's content filter.
const finishReasonContentFilter = "content_filter"

// finishReasonError is the provider finish reason indicating generation
// failed on the provider side.
const finishReasonError = "error"

// genMessage calls the configured provider to produce a commit message for diff and returns it.
// It attempts to generate a message up to maxGenAttempts times.
// It returns an error if all attempts fail.
func genMessage(ctx context.Context, cfg *config, diff string) (string, error) {
	// Build prompt
	prompt := buildPrompt(diff)
	// Attempt generation up to maxGenAttempts times.
	var lastErr error
	for attempt := 1; attempt <= maxGenAttempts; attempt++ {
		// Call the provider to produce a commit message for diff.
		res, err := chatOnce(ctx, cfg, prompt)
		// Return the error if the provider call fails.
		if err != nil {
			return "", err
		}
		// Validate the generated message.
		validateErr := validateMessage(res.content)
		// Accept the message if it validates and was not truncated.
		if validateErr == nil && !res.truncated() {
			return res.content, nil
		}
		// Build feedback from the validation failure and/or truncation.
		var feedback string
		// Report validation failure as a failure when the message is invalid.
		if validateErr != nil {
			feedback = "\n\nYour previous output was invalid: " + validateErr.Error()
		}
		// Explain truncation so the retry stays under the token limit.
		if res.truncated() {
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
				"\nPrevious output:\n" + res.content +
				"\nRegenerate the commit message following STRICT FORMAT RULES exactly."
		}
	}
	// Return an error when all attempts produced invalid messages.
	return "", tserr.Op(&tserr.OpArgs{Op: "generate message", Fn: cfg.provider.name, Err: lastErr})
}

// chatResult holds the outcome of a single chat completion. The zero value
// means no message was produced and is returned alongside every error.
type chatResult struct {
	content      string // The cleaned commit message produced by the model.
	finishReason string // Why generation stopped, e.g. "stop" or "length".
}

// truncated reports whether the model output was cut off by the token limit.
func (r chatResult) truncated() bool {
	// Report truncation when the provider stopped generation due to length.
	return r.finishReason == finishReasonLength
}

// chatOnce performs a single chat completion and returns the cleaned result.
// It returns the zero result and an error if the call fails or the response is
// unusable.
func chatOnce(ctx context.Context, cfg *config, prompt string) (chatResult, error) {
	// Build payload
	payload := requestPayload{
		Model:               cfg.model,
		Messages:            []message{{Role: "user", Content: prompt}},
		MaxCompletionTokens: cfg.maxCompletionTokens,
		Temperature:         cfg.temperature,
	}
	// Marshal payload
	body, err := json.Marshal(payload)
	// Return the zero result and an error if marshalling payload fails.
	if err != nil {
		return chatResult{}, tserr.Op(&tserr.OpArgs{Op: "marshal payload", Fn: cfg.provider.name, Err: err})
	}
	// Merge provider-specific body fields into the payload.
	body, err = mergeExtraBody(body, cfg.provider.extraBody)
	// Return the zero result and an error if merging extra body fields fails.
	if err != nil {
		return chatResult{}, tserr.Op(&tserr.OpArgs{Op: "merge extra body", Fn: cfg.provider.name, Err: err})
	}
	// Create request with body
	req, err := http.NewRequestWithContext(ctx, "POST", cfg.provider.baseURL, bytes.NewBuffer(body))
	// Return an empty string and an error if creating request fails.
	if err != nil {
		return chatResult{}, tserr.Op(&tserr.OpArgs{Op: "new http request", Fn: cfg.provider.name, Err: err})
	}
	// Set headers
	// Authorization header
	req.Header.Set(cfg.provider.authHeader, cfg.provider.authValue+cfg.apiKey)
	// Content-Type header
	req.Header.Set("Content-Type", "application/json")
	// Static headers
	for k, v := range cfg.provider.staticHeaders {
		req.Header.Set(k, v)
	}
	// Send request
	resp, err := defaultHTTPClient.Do(req)
	// Return an empty string and an error if sending request fails.
	if err != nil {
		return chatResult{}, tserr.Op(&tserr.OpArgs{Op: "send request", Fn: cfg.provider.name, Err: err})
	}
	// Close response body after reading
	defer resp.Body.Close()
	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	// Return an empty string and an error if reading response body fails.
	if err != nil {
		return chatResult{}, tserr.Op(&tserr.OpArgs{Op: "read body", Fn: cfg.provider.name, Err: err})
	}
	// Return error if status code is not in the expected range.
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Unmarshal error response
		var errRes apiErrorResponse
		if err := json.Unmarshal(respBody, &errRes); err == nil && errRes.Error.Message != "" {
			// Return error if error message is not empty and use it as the error.
			return chatResult{}, tserr.NotAvailable(&tserr.NotAvailableArgs{
				S:   cfg.provider.name,
				Err: errors.New(errRes.Error.Message),
			})
		}
		// If error message is empty or unmarshalling fails, return status not matching error.
		return chatResult{}, tserr.StatusNotMatching(&tserr.StatusNotMatchingArgs{
			Expected: http.StatusOK, Actual: resp.StatusCode,
		})
	}
	// Unmarshal response body
	var res responsePayload
	if err := json.Unmarshal(respBody, &res); err != nil {
		// Return error if unmarshalling fails and an error.
		return chatResult{}, tserr.Op(&tserr.OpArgs{Op: "unmarshal", Fn: "string(respBody)", Err: err})
	}
	// Return empty string if no choices and an error.
	if len(res.Choices) == 0 {
		return chatResult{}, tserr.Empty(cfg.provider.name + " response choices")
	}
	// Reject responses the provider flagged as unusable; these are not
	// retryable, so fail before the caller spends another attempt.
	switch res.Choices[0].FinishReason {
	case finishReasonContentFilter:
		return chatResult{}, tserr.Op(&tserr.OpArgs{
			Op:  "generate message",
			Fn:  cfg.provider.name,
			Err: errors.New("response blocked by content filter"),
		})
	case finishReasonError:
		return chatResult{}, tserr.Op(&tserr.OpArgs{
			Op:  "generate message",
			Fn:  cfg.provider.name,
			Err: errors.New("provider reported a generation error"),
		})
	}
	// Clean message
	content := cleanMessage(res.Choices[0].Message.Content)
	// Return the zero result and an error if the message is empty after cleaning.
	if content == "" {
		return chatResult{}, tserr.Empty(cfg.provider.name + " commit message")
	}
	// Return the cleaned result.
	return chatResult{content: content, finishReason: res.Choices[0].FinishReason}, nil
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

// mergeExtraBody overlays the provider-specific fields in extra onto the base
// JSON object and returns the merged JSON. Extra values win over base fields
// with the same key. It returns base unchanged when extra is empty.
func mergeExtraBody(base []byte, extra map[string]any) ([]byte, error) {
	// Return base unchanged if no extra fields are configured.
	if len(extra) == 0 {
		return base, nil
	}
	// Decode the base payload into a generic object.
	var obj map[string]any
	// Return an error if the base payload is not a JSON object.
	if err := json.Unmarshal(base, &obj); err != nil {
		return nil, err
	}
	// Overlay each extra field, letting the provider-specific value win.
	for k, v := range extra {
		obj[k] = v
	}
	// Marshal the merged object.
	return json.Marshal(obj)
}
