// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Imports for testing.
import (
	"fmt"     // Import the fmt package for formatting quoted values in failures.
	"testing" // Import the testing package for unit tests.

	"github.com/thorsphere/tscommit" // Import the tscommit package under test.
	"github.com/thorsphere/tserr"    // Import the tserr package for error reporting.
)

// TestNewClient verifies client construction across API key scenarios.
func TestNewClient(t *testing.T) {
	// Define the API key scenarios and their expected outcome.
	tests := []struct {
		name    string // The scenario name.
		key     string // The OPENROUTER_API_KEY value to install.
		wantErr bool   // Whether construction must fail.
	}{
		{"valid key", "sk-or-v1-test-key", false}, // A well-formed key builds the client.
		{"missing key", "", true},                 // An unset key fails validation.
		{"whitespace key", "   ", true},           // A blank key trims to empty and fails.
		{"invalid header key", "bad\nkey", true},  // A control character fails validation.
	}
	// Run each scenario against newClient.
	for _, tc := range tests {
		// Run one scenario in its own subtest.
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Build the AI client.
			client, err := tscommit.NewClient()
			// Report an error-presence mismatch.
			if (err != nil) != tc.wantErr {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{
					Var:    "newClient error presence",
					Actual: fmt.Sprintf("%v", err != nil),
					Want:   fmt.Sprintf("%v", tc.wantErr),
				}))
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
