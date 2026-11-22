// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"fmt"     // Import the fmt package for formatting quoted values in failures.
	"strings" // Import the strings package for error substring checks.
	"testing" // Import the testing package for unit tests.

	"github.com/thorsphere/tscommit" // Import the tscommit package under test.
	"github.com/thorsphere/tserr"    // Import the tserr package for error reporting.
)

// TestConfig verifies NewConfig defaults, WithModel and WithReasoningEffort
// mutation with whitespace trimming, chaining identity, clearing the
// reasoning effort level, and nil-receiver safety.
func TestConfig(t *testing.T) {
	// Define the configuration scenarios.
	cases := []struct {
		name       string                                      // The scenario name.
		cfg        *tscommit.Config                            // The receiver; nil exercises nil-receiver safety.
		apply      func(cfg *tscommit.Config) *tscommit.Config // The mutation applied to cfg.
		wantModel  string                                      // The expected Model() result.
		wantEffort string                                      // The expected ReasoningEffort() result.
	}{
		{
			name:       "NewConfig installs package defaults",
			cfg:        tscommit.NewConfig(),
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg },
			wantModel:  "google/gemini-4-hydrogen",
			wantEffort: "low",
		},
		{
			name:       "WithModel trims surrounding whitespace",
			cfg:        tscommit.NewConfig(),
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg.WithModel("  openai/gpt-5  ") },
			wantModel:  "openai/gpt-5",
			wantEffort: "low",
		},
		{
			name:       "WithReasoningEffort trims surrounding whitespace",
			cfg:        tscommit.NewConfig(),
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg.WithReasoningEffort("  low  ") },
			wantModel:  "google/gemini-4-hydrogen",
			wantEffort: "low",
		},
		{
			name: "chained setters return the same pointer",
			cfg:  tscommit.NewConfig(),
			apply: func(cfg *tscommit.Config) *tscommit.Config {
				return cfg.WithModel("openai/gpt-5").WithReasoningEffort("medium")
			},
			// The chained result carries both updated values on one instance.
			wantModel:  "openai/gpt-5",
			wantEffort: "medium",
		},
		{
			name:       "WithReasoningEffort with empty string clears the level",
			cfg:        tscommit.NewConfig(),
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg.WithReasoningEffort("") },
			wantModel:  "google/gemini-4-hydrogen",
			wantEffort: "",
		},
		{
			name:       "WithReasoningEffort with whitespace clears the level",
			cfg:        tscommit.NewConfig(),
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg.WithReasoningEffort("   ") },
			wantModel:  "google/gemini-4-hydrogen",
			wantEffort: "",
		},
		{
			name:       "nil receiver WithModel returns nil",
			cfg:        nil,
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg.WithModel("x") },
			wantModel:  "",
			wantEffort: "",
		},
		{
			name:       "nil receiver WithReasoningEffort returns nil",
			cfg:        nil,
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg.WithReasoningEffort("low") },
			wantModel:  "",
			wantEffort: "",
		},
		{
			name:       "nil receiver accessors return empty strings",
			cfg:        nil,
			apply:      func(cfg *tscommit.Config) *tscommit.Config { return cfg },
			wantModel:  "",
			wantEffort: "",
		},
	}
	// Run each scenario against the Config API.
	for _, tc := range cases {
		// Run one scenario in its own subtest.
		t.Run(tc.name, func(t *testing.T) {
			// Apply the mutation under test.
			got := tc.apply(tc.cfg)
			// Verify the mutator returns the receiver for chaining.
			if got != tc.cfg {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "mutator result", Actual: fmt.Sprintf("%p", got), Want: fmt.Sprintf("%p", tc.cfg)}))
			}
			// Verify the model identifier.
			if model := got.Model(); model != tc.wantModel {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Config.Model", Actual: model, Want: tc.wantModel}))
			}
			// Verify the reasoning effort level.
			if effort := got.ReasoningEffort(); effort != tc.wantEffort {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Config.ReasoningEffort", Actual: effort, Want: tc.wantEffort}))
			}
		})
	}
}

// TestNewClient verifies client construction across configuration and API
// key scenarios. It uses t.Setenv and must not call t.Parallel.
func TestNewClient(t *testing.T) {
	// Define the configuration and API key scenarios and their expected outcome.
	tests := []struct {
		name        string           // The scenario name.
		cfg         *tscommit.Config // The configuration passed to NewClient.
		key         string           // The OPENROUTER_API_KEY value to install.
		wantErr     bool             // Whether construction must fail.
		wantErrText string           // Substring the returned error must contain.
	}{
		{"default config with valid key", tscommit.NewConfig(), "sk-or-v1-test-key", false, ""},
		{"custom model with low effort", tscommit.NewConfig().WithModel("openai/gpt-5").WithReasoningEffort("low"), "sk-or-v1-test-key", false, ""},
		{"custom model with medium effort", tscommit.NewConfig().WithModel("openai/gpt-5").WithReasoningEffort("medium"), "sk-or-v1-test-key", false, ""},
		{"custom model with empty effort", tscommit.NewConfig().WithModel("openai/gpt-5").WithReasoningEffort(""), "sk-or-v1-test-key", false, ""},
		{"nil config", nil, "sk-or-v1-test-key", true, "config"},
		{"whitespace model", tscommit.NewConfig().WithModel("   "), "sk-or-v1-test-key", true, "new config"},
		{"unsupported reasoning effort", tscommit.NewConfig().WithReasoningEffort("invalid"), "sk-or-v1-test-key", true, "new client"},
		{"missing key", tscommit.NewConfig(), "", true, "new client"},
		{"whitespace key", tscommit.NewConfig(), "   ", true, "new client"},
		{"invalid header key", tscommit.NewConfig(), "bad\nkey", true, "new client"},
	}
	// Run each scenario against newClient.
	for _, tc := range tests {
		// Run one scenario in its own subtest.
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Build the AI client.
			client, err := tscommit.NewClient(tc.cfg)
			// Report an error-presence mismatch.
			if (err != nil) != tc.wantErr {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{
					Var:    "newClient error presence",
					Actual: fmt.Sprintf("%v", err != nil),
					Want:   fmt.Sprintf("%v", tc.wantErr),
				}))
			}
			// Report a missing diagnostic substring on the failure path.
			if tc.wantErr && err != nil && tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "newClient error text", Actual: err.Error(), Want: tc.wantErrText}))
			}
			// Report a nil client on the success path.
			if !tc.wantErr && client == nil {
				t.Error(tserr.NilFailed("newClient"))
			}
		})
	}
}

// TestTimeoutBounds verifies that DefaultTimeout is positive and that
// MaxGenerateDuration is at least DefaultTimeout.
func TestTimeoutBounds(t *testing.T) {
	// Verify DefaultTimeout is strictly positive.
	if tscommit.DefaultTimeout <= 0 {
		t.Error(tserr.Higher(&tserr.HigherArgs{
			Var:        "DefaultTimeout",
			Actual:     int64(tscommit.DefaultTimeout),
			LowerBound: 1,
		}))
	}
	// Verify MaxGenerateDuration covers at least one full DefaultTimeout attempt.
	if tscommit.MaxGenerateDuration < tscommit.DefaultTimeout {
		t.Error(tserr.Higher(&tserr.HigherArgs{
			Var:        "MaxGenerateDuration",
			Actual:     int64(tscommit.MaxGenerateDuration),
			LowerBound: int64(tscommit.DefaultTimeout),
		}))
	}
}
