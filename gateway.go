// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"bytes"         // Import the bytes package for byte slices.
	"context"       // Import the context package for context handling.
	"encoding/json" // Import the json package for JSON encoding/decoding.
	"fmt"           // Import the fmt package for formatting.
	"io"            // Import the io package for input/output.
	"net/http"      // Import the http package for HTTP requests.
	"strings"       // Import the strings package for string manipulation.
	"time"          // Import the time package for time handling.

	"github.com/thorsphere/tserr" // Import the tserr package for error handling.
)

var defaultHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestPayload struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type responsePayload struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

type apiErrorResponse struct {
	Error struct {
		Message string      `json:"message"`
		Code    interface{} `json:"code"`
	} `json:"error"`
}

// genMessage calls OpenRouter to produce a commit message for diff.
func genMessage(ctx context.Context, cfg *config, diff string) (string, error) {
	prompt := buildPrompt(diff)

	content, err := chatOnce(ctx, cfg, prompt)
	if err != nil {
		return "", err
	}

	if err := validateMessage(content); err != nil {
		retryPrompt := prompt +
			"\n\nYour previous output was invalid: " + err.Error() +
			"\nRegenerate the commit message following STRICT FORMAT RULES exactly."
		content, err = chatOnce(ctx, cfg, retryPrompt)
		if err != nil {
			return "", err
		}
		if err := validateMessage(content); err != nil {
			return "", tserr.Op(&tserr.OpArgs{
				Op:  "validate message",
				Fn:  "OpenRouter",
				Err: err,
			})
		}
	}

	return content, nil
}

// chatOnce performs a single OpenRouter chat completion and returns the
// cleaned message content.
func chatOnce(ctx context.Context, cfg *config, prompt string) (string, error) {
	payload := requestPayload{
		Model:    cfg.model,
		Messages: []message{{Role: "user", Content: prompt}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "marshal payload", Fn: "OpenRouter", Err: err})
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "new http request", Fn: "OpenRouter", Err: err})
	}

	req.Header.Set("Authorization", "Bearer "+cfg.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/thorsphere/tscommit")
	req.Header.Set("X-OpenRouter-Title", "tscommit")
	req.Header.Set("X-OpenRouter-Categories", "cli-agent")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "send request", Fn: "OpenRouter", Err: err})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", tserr.Op(&tserr.OpArgs{Op: "read body", Fn: "OpenRouter", Err: err})
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var errRes apiErrorResponse
		if err := json.Unmarshal(respBody, &errRes); err == nil && errRes.Error.Message != "" {
			return "", tserr.Op(&tserr.OpArgs{
				Op:  fmt.Sprintf("status %d (%s)", resp.StatusCode, errRes.Error.Message),
				Fn:  "OpenRouter",
				Err: fmt.Errorf("API error: %s", errRes.Error.Message),
			})
		}
		return "", tserr.Op(&tserr.OpArgs{
			Op:  fmt.Sprintf("status %d", resp.StatusCode),
			Fn:  "OpenRouter",
			Err: fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody))),
		})
	}

	var res responsePayload
	if err := json.Unmarshal(respBody, &res); err != nil || len(res.Choices) == 0 {
		return "", tserr.Op(&tserr.OpArgs{Op: "unmarshal", Fn: "string(respBody)", Err: err})
	}

	if len(res.Choices) == 0 {
		return "", tserr.Empty("OpenRouter response")
	}

	content := cleanMessage(res.Choices[0].Message.Content)
	if content == "" {
		return "", tserr.Empty("OpenRouter commit message")
	}

	return content, nil
}

// cleanMessage removes markdown code fences, backticks, and extra wrapper formatting.
func cleanMessage(s string) string {
	s = strings.TrimSpace(s)

	// Strip outer markdown code block fences (e.g. ```commit ... ``` or ``` ...)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence and optional language tag up to newline
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		} else {
			s = strings.TrimPrefix(s, "```")
		}

		// Remove trailing fence
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}

	// Strip outer single or double quotes / backticks if wrapped
	s = strings.Trim(strings.TrimSpace(s), "`\"'")

	return strings.TrimSpace(s)
}
