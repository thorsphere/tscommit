// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Defaults for provider request settings.
const (
	defaultModel               = "tencent/hy4-preview" // The default model identifier.
	defaultMaxCompletionTokens = 1000                  // The default cap on generated tokens.
	defaultTemperature         = 0.3                   // The default sampling temperature.
	defaultReasoningEffort     = "medium"              // The default reasoning effort level.
)

// provider describes how to reach a chat completion API: endpoint, auth
// placement, provider-specific headers, and provider-specific request body
// fields. The zero value is not usable.
type provider struct {
	name          string            // The provider name used in error contexts.
	baseURL       string            // The chat completions endpoint URL.
	authHeader    string            // The auth header name, e.g. "Authorization".
	authValue     string            // The auth header prefix before the secret, e.g. "Bearer ".
	staticHeaders map[string]string // Provider-specific headers sent with every request.
	extraBody     map[string]any    // Provider-specific fields merged into the request body.
}

// config carries the settings for a commit message generation run.
// The zero value is not usable.
type config struct {
	apiKey              string   // The secret API key appended to authValue; never logged.
	model               string   // The model identifier passed to the provider.
	maxCompletionTokens int      // The maximum number of tokens the model may generate per call.
	temperature         float64  // The sampling temperature; lower values stabilize format.
	provider            provider // The chat completion API to call.
}

// openRouterProvider is the hard-coded default provider.
var openRouterProvider = provider{
	name:       "OpenRouter",
	baseURL:    "https://openrouter.ai/api/v1/chat/completions",
	authHeader: "Authorization",
	authValue:  "Bearer ",
	staticHeaders: map[string]string{
		"HTTP-Referer":            "https://github.com/thorsphere/tscommit",
		"X-OpenRouter-Title":      "tscommit",
		"X-OpenRouter-Categories": "cli-agent,cloud-agent",
	},
	extraBody: map[string]any{
		"reasoning_effort": defaultReasoningEffort,
	},
}

// DefaultConfig returns the configuration used when no overrides are applied.
func DefaultConfig() *config {
	return &config{
		model:               defaultModel,
		maxCompletionTokens: defaultMaxCompletionTokens,
		temperature:         defaultTemperature,
		provider:            openRouterProvider,
	}
}

func (cfg *config) withAPIKey(apiKey string) *config {
	cfg.apiKey = apiKey
	return cfg
}

func (cfg *config) withModel(model string) *config {
	cfg.model = model
	return cfg
}

// withMaxCompletionTokens sets the generation token cap and returns cfg.
func (cfg *config) withMaxCompletionTokens(maxCompletionTokens int) *config {
	cfg.maxCompletionTokens = maxCompletionTokens
	return cfg
}

// withTemperature sets the sampling temperature and returns cfg.
func (cfg *config) withTemperature(temperature float64) *config {
	cfg.temperature = temperature
	return cfg
}
