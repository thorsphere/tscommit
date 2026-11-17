// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

// Package main is the entry point for the tscommit Cloud Run service, a
// non-interactive HTTP microservice that generates Conventional Commits
// messages from unified diffs via a configured AI provider. Configuration
// comes exclusively from environment variables, all state is per-request, and
// third-party credentials remain server-side.
package main

// Import packages
import (
	"context"   // Import the context package for graceful shutdown coordination.
	"os"        // Import the os package for exit handling.
	"os/signal" // Import the os/signal package for signal notification.
	"syscall"   // Import the syscall package for SIGTERM.

	"github.com/thorsphere/tscommit/internal/srv" // Import the internal/srv package for the CommitServer lifecycle.
	"github.com/thorsphere/tslog"                 // Import the tslog package for structured logging.
)

// main starts the tscommit Cloud Run service and blocks until shutdown. It
// exits non-zero if the server cannot start or shuts down unsuccessfully.
func main() {
	// Set the logging level to Info for the service lifecycle.
	tslog.SetLevel(tslog.InfoLevel)
	// Set up a context that is canceled on interrupt or termination signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Release signal notification resources when the program exits.
	defer stop()
	// Run the HTTP service until ctx is canceled or an error occurs.
	if err := srv.RunContext(ctx); err != nil {
		// Log the startup, listener, or shutdown failure.
		tslog.Error(err)
		// Exit non-zero because the service did not complete cleanly.
		os.Exit(1)
	}
}
