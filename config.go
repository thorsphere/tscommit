// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

import (
	"time"

	"github.com/thorsphere/tsaiclient"
)

// Defaults for provider request settings.
const (
	defaultModel               = "tencent/hy4-preview" // The default model identifier.
	defaultMaxCompletionTokens = 1000                  // The default cap on generated tokens.
	defaultTemperature         = 0.3                   // The default sampling temperature.
	defaultReasoningEffort     = "medium"              // The default reasoning effort level.
)

// newClient returns the AI client used for commit message generation. It
// returns an error if the provider configuration is invalid, including a
// missing OPENROUTER_API_KEY in the environment.
func newClient() (*tsaiclient.Client, error) {
	// Build the model configuration with the package defaults.
	cfg, err := tsaiclient.NewConfig(defaultModel)
	// Return the error if the model configuration is invalid.
	if err != nil {
		return nil, err
	}
	// Apply the generation defaults used for commit messages.
	cfg = cfg.WithMaxCompletionTokens(defaultMaxCompletionTokens).
		WithTemperature(defaultTemperature).
		WithReasoningEffort(defaultReasoningEffort).
		WithTimeout(60 * time.Second)
	// Build the OpenRouter provider with app attribution headers.
	p := tsaiclient.NewOpenRouter().
		WithReferer("https://github.com/thorsphere/tscommit").
		WithTitle("tscommit").
		WithCategories([]string{"cli-agent", "cloud-agent"})
	// Return the client; construction validates cfg and p, including the key.
	return tsaiclient.NewClient(cfg, p)
}
