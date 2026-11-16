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
	"errors"    // Import the errors package for error inspection.
	"fmt"       // Import the fmt package for formatted diagnostics.
	"net/http"  // Import the net/http package for the server and listener.
	"os"        // Import the os package for environment variables and exit handling.
	"os/signal" // Import the os/signal package for signal notification.
	"strings"   // Import the strings package for whitespace trimming.
	"syscall"   // Import the syscall package for SIGTERM.
	"time"      // Import the time package for the shutdown deadline.

	"github.com/thorsphere/tscommit/internal/srv" // Import the internal/srv package for the CommitServer.
	"github.com/thorsphere/tserr"                 // Import the tserr package for contextual errors.
	"github.com/thorsphere/tslog"                 // Import the tslog package for structured logging.
)

// Defaults for Cloud Run server startup and shutdown.
const (
	srvPort           = "8080"                               // The fallback port when the PORT environment variable is unset.
	srvAddr           = ":"                                  // The host prefix that binds the server to all interfaces.
	readHeaderTimeout = 5 * time.Second                      // The maximum duration for reading request headers.
	readTimeout       = 15 * time.Second                     // The maximum duration for reading the entire request body.
	writeTimeout      = srv.GenerateTimeout + 15*time.Second // The socket backstop derived above the handler generation deadline.
	idleTimeout       = 60 * time.Second                     // The maximum duration to keep idle keep-alive connections open.
	shutdownTimeout   = 8 * time.Second                      // The deadline for draining in-flight requests after SIGTERM.
)

// main starts the tscommit Cloud Run service and blocks until shutdown. It
// exits non-zero if the server cannot start or shuts down unsuccessfully.
func main() {
	// Read and trim the listen port from the environment (Cloud Run injects PORT).
	port := strings.TrimSpace(os.Getenv("PORT"))
	// Fall back to the development port when PORT is unset or whitespace-only.
	if port == "" {
		// Warn that Cloud Run should supply PORT in production.
		tslog.Warn(fmt.Sprintf("PORT environment variable not set, defaulting to %s", srvPort))
		// Use the development port.
		port = srvPort
	}
	// Construct the listen address for all interfaces.
	serverAddr := srvAddr + port
	// Read the bearer token protecting the generate endpoint.
	token := os.Getenv("API_TOKEN")
	// Exit non-zero if the token is missing because the server cannot operate without it.
	if token == "" {
		// Log the missing token error.
		tslog.Error(tserr.Empty("API_TOKEN"))
		// Exit non-zero because the server cannot start without a token.
		os.Exit(1)
	}
	// Construct the HTTP server handler wrapping the generator.
	srv, err := srv.NewCommitServer(token)
	// Exit non-zero if the server cannot be constructed.
	if err != nil {
		// Log the server construction failure.
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "new commit server", Fn: serverAddr, Err: err}))
		// Exit non-zero because the server handler could not be built.
		os.Exit(1)
	}
	// Construct the HTTP server bound to the configured address.
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           srv,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	// Buffer a listener error so the server goroutine never blocks on send.
	errCh := make(chan error, 1)
	// Start the server in a separate goroutine.
	go func() {
		// Log the address on which the server is starting.
		tslog.Info(fmt.Sprintf("Starting CommitServer on %v", serverAddr))
		// Serve incoming requests and forward unexpected listener errors.
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Report the startup or listener error to the main goroutine.
			errCh <- err
		}
	}()
	// Set up a context that is canceled on interrupt or termination signals.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Wait until a shutdown signal arrives or the listener fails.
	select {
	case err := <-errCh:
		// Release signal notification resources before exiting.
		stop()
		// Log the listener failure.
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "ListenAndServe", Fn: serverAddr, Err: err}))
		// Exit non-zero because the server failed to start or serve.
		os.Exit(1)
	case <-sigCtx.Done():
		// Release signal notification resources once graceful shutdown begins.
		stop()
	}
	// Log that graceful shutdown has started after receiving a termination signal.
	tslog.Info("Shutting down server gracefully...")
	// Bound the graceful shutdown duration so draining requests cannot hang indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	// Attempt to gracefully shut down the server and drain active connections.
	if err := server.Shutdown(ctx); err != nil {
		// Release the shutdown context resources before exiting.
		cancel()
		// Log the graceful shutdown failure.
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "Shutdown", Fn: serverAddr, Err: err}))
		// Exit non-zero because the server did not shut down cleanly.
		os.Exit(1)
	}
	// Release the shutdown context resources after clean shutdown.
	cancel()
	// Log that the server has exited cleanly.
	tslog.Info("Server successfully exited")
}
