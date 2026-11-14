// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"time" // Import the time package for timeout settings.

	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
	"github.com/thorsphere/tserr"      // Import the tserr package for error handling.
)

// Defaults for provider and request settings.
const (
	// DefaultTimeout is the per-attempt AI provider request timeout.
	DefaultTimeout = 2 * time.Minute
	// MaxGenerateDuration is the worst-case wall-clock budget for a single
	// Generate call across all retry attempts.
	MaxGenerateDuration        = maxGenAttempts * DefaultTimeout
	defaultModel               = "google/gemini-4-hydrogen" // The default model identifier.
	defaultMaxCompletionTokens = 16000                      // The default cap on generated tokens.
	defaultTemperature         = 0.3                        // The default sampling temperature.
	defaultReasoningEffort     = "high"                     // The default reasoning effort level.
	maxGenAttempts             = 2                          // The maximum number of provider calls per message (1 initial + retries with feedback).
	minConfidence              = 0.0                        // The minimum allowed confidence score.
	maxConfidence              = 1.0                        // The maximum allowed confidence score.
)

// newClient returns the AI client used for commit message generation. It
// returns an error if the provider configuration is invalid.
func newClient() (*tsaiclient.Client, error) {
	// Build the model configuration with the package defaults.
	cfg, err := tsaiclient.NewConfig(defaultModel)
	// Return the error if the model configuration is invalid.
	if err != nil {
		return nil, tserr.Op(&tserr.OpArgs{Op: "new config", Fn: "AI client", Err: err})
	}
	// Apply the generation defaults used for commit messages.
	cfg = cfg.WithMaxCompletionTokens(defaultMaxCompletionTokens).
		WithTemperature(defaultTemperature).
		WithReasoningEffort(defaultReasoningEffort).
		WithTimeout(DefaultTimeout)
	// Build the OpenRouter provider with app attribution headers.
	p := tsaiclient.NewOpenRouter().
		WithReferer("https://github.com/thorsphere/tscommit").
		WithTitle("tscommit").
		WithCategories([]string{"cli-agent", "cloud-agent"})
	// Return the client; construction validates cfg and p, including the key.
	c, err := tsaiclient.NewClient(cfg, p)
	// Return the error if the client construction fails.
	if err != nil {
		return nil, tserr.Op(&tserr.OpArgs{Op: "new client", Fn: "AI client", Err: err})
	}
	// Return the constructed client and nil error to indicate success.
	return c, nil
}
