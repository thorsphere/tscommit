// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// Import packages
import (
	"strings" // Import the strings package for whitespace trimming.
	"time"    // Import the time package for timeout settings.

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
	defaultReasoningEffort     = "low"                      // The default reasoning effort level.
	maxGenAttempts             = 2                          // The maximum number of provider calls per message (1 initial + retries with feedback).
	minConfidence              = 0.0                        // The minimum allowed confidence score.
	maxConfidence              = 1.0                        // The maximum allowed confidence score.
)

// Config carries the AI provider settings used for commit message
// generation. Each instance provides the model identifier and the reasoning
// effort level forwarded to the AI provider. The zero value is not usable
// directly; construct instances with NewConfig.
type Config struct {
	model           string // The model identifier passed to the AI provider.
	reasoningEffort string // The reasoning effort level passed to the AI provider.
}

// NewConfig returns a Config populated with the package defaults: the
// default model identifier and the default reasoning effort level.
func NewConfig() *Config {
	// Initialize a configuration with the package defaults.
	return &Config{
		model:           defaultModel,
		reasoningEffort: defaultReasoningEffort,
	}
}

// WithModel sets the model identifier to model, trimmed of leading and
// trailing whitespace, and returns cfg. It is a no-op returning nil when cfg
// is nil.
func (cfg *Config) WithModel(model string) *Config {
	// Set the model identifier if cfg is not nil.
	if cfg != nil {
		cfg.model = strings.TrimSpace(model)
	}
	// Return the configuration.
	return cfg
}

// WithReasoningEffort sets the reasoning effort level to effort, trimmed of
// leading and trailing whitespace, and returns cfg. An empty effort clears
// the level so the provider default applies. It is a no-op returning nil
// when cfg is nil.
func (cfg *Config) WithReasoningEffort(effort string) *Config {
	// Set the reasoning effort level if cfg is not nil.
	if cfg != nil {
		cfg.reasoningEffort = strings.TrimSpace(effort)
	}
	// Return the configuration.
	return cfg
}

// Model returns the configured model identifier, or an empty string when cfg
// is nil.
func (cfg *Config) Model() string {
	// Return an empty string if cfg is nil.
	if cfg == nil {
		return ""
	}
	// Return the configured model identifier.
	return cfg.model
}

// ReasoningEffort returns the configured reasoning effort level, or an
// empty string when cfg is nil. An empty level means the provider default
// applies.
func (cfg *Config) ReasoningEffort() string {
	// Return an empty string if cfg is nil.
	if cfg == nil {
		return ""
	}
	// Return the configured reasoning effort level.
	return cfg.reasoningEffort
}

// newClient returns the AI client used for commit message generation,
// configured from cfg. It returns an error if cfg is nil or the provider
// configuration is invalid.
func newClient(cfg *Config) (*tsaiclient.Client, error) {
	// Return an error if the configuration is nil.
	if cfg == nil {
		return nil, tserr.Empty("config")
	}
	// Build the model configuration from the configured model identifier.
	aiCfg, err := tsaiclient.NewConfig(cfg.model)
	// Return the error if the model configuration is invalid.
	if err != nil {
		return nil, tserr.Op(&tserr.OpArgs{Op: "new config", Fn: "AI client", Err: err})
	}
	// Apply the generation defaults used for commit messages.
	aiCfg = aiCfg.WithMaxCompletionTokens(defaultMaxCompletionTokens).
		WithTemperature(defaultTemperature).
		WithReasoningEffort(cfg.reasoningEffort).
		WithTimeout(DefaultTimeout)
	// Build the OpenRouter provider with app attribution headers.
	p := tsaiclient.NewOpenRouter().
		WithReferer("https://github.com/thorsphere/tscommit").
		WithTitle("tscommit").
		WithCategories([]string{"cli-agent", "cloud-agent"})
	// Return the client; construction validates cfg and p, including the key.
	c, err := tsaiclient.NewClient(aiCfg, p)
	// Return the error if the client construction fails.
	if err != nil {
		return nil, tserr.Op(&tserr.OpArgs{Op: "new client", Fn: "AI client", Err: err})
	}
	// Return the constructed client and nil error to indicate success.
	return c, nil
}
