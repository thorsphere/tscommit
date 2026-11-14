// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

// Package main is the entry point for the tscommit CLI helper, an
// interactive command-line tool that generates conventional git commit
// messages from the staged diff via a configured AI provider and creates
// the commit after user review. It reads its configuration, including the
// OPENROUTER_API_KEY required to call the AI provider, exclusively from
// environment variables, binds no port, and persists no local state.
package main

// Import packages
import (
	"context"   // Import the context package for context handling.
	"os"        // Import the os package for environment variable access and exit handling.
	"os/signal" // Import the os/signal package for signal handling.
	"syscall"   // Import the syscall package for signal constants.

	"github.com/thorsphere/tscommit/internal/cli" // Import the internal cli package for the tscommit workflow.
	"github.com/thorsphere/tserr"                 // Import the tserr package for error reporting.
	"github.com/thorsphere/tslog"                 // Import the tslog package for logging.
)

// main is the entry point for the tscommit command-line tool.
// It sets up logging, handles signals, and runs the tscommit workflow.
func main() {
	// Retrieve the OPENROUTER_API_KEY from the environment variables.
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	// Check if the OPENROUTER_API_KEY environment variable is set; if not, log an error and exit.
	if apiKey == "" {
		// Log that the API key is not set.
		tslog.Error(tserr.NotSet("OPENROUTER_API_KEY"))
		// Exit with a non-zero status code to indicate failure.
		os.Exit(1)
	}
	// Set the logging level to Info for the tscommit workflow.
	tslog.SetLevel(tslog.InfoLevel)
	// Set up a context that is canceled on interrupt signals (Ctrl+C) or termination signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Ensure that the context is canceled when the program exits.
	defer stop()
	// Run the tscommit workflow with context cancellation support.
	if err := cli.RunContext(ctx); err != nil {
		// Log the error and exit with a non-zero status code.
		tslog.Error(err)
		// Exit with a non-zero status code to indicate failure.
		os.Exit(1)
	}
}
