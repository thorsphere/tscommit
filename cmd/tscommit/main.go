// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/thorsphere/tscommit"
	"github.com/thorsphere/tserr"
	"github.com/thorsphere/tslog"
)

func main() {
	// Retrieve the OPENROUTER_API_KEY from the environment variables.
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	// Check if the OPENROUTER_API_KEY environment variable is set; if not, log an error and exit.
	if apiKey == "" {
		tslog.Error(tserr.NotSet("OPENROUTER_API_KEY"))
		os.Exit(1)
	}

	// Set the logging level to Info for the tscommit workflow.
	tslog.SetLevel(tslog.InfoLevel)

	// Set up a context that is canceled on interrupt signals (Ctrl+C) or termination signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Ensure that the context is canceled when the program exits.
	defer stop()

	// Run the tscommit workflow with context cancellation support.
	if err := tscommit.RunContext(ctx); err != nil {
		// Log the error and exit with a non-zero status code.
		tslog.Error(err)
		// Exit with a non-zero status code to indicate failure.
		os.Exit(1)
	}
}
