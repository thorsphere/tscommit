// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

// Package srv implements the Cloud Run service for authenticated Conventional
// Commits message generation from unified diffs.
package srv

// Import packages
import (
	"context"  // Import the context package for request cancellation and lifecycle coordination.
	"net/http" // Import the net/http package for routing, handlers, and status codes.
	"strings"  // Import the strings package for whitespace validation.
	"time"     // Import the time package for the generation timeout buffer.

	"github.com/thorsphere/lpsrv"    // Import the lpsrv package for HTTP middleware and Cloud Run lifecycle management.
	"github.com/thorsphere/tscommit" // Import the tscommit package for non-interactive commit message generation.
	"github.com/thorsphere/tserr"    // Import the tserr package for structured error construction.
)

// CommitServer serves unauthenticated health probes and bearer-authenticated
// commit message generation. The zero value is not usable; use NewCommitServer.
type CommitServer struct {
	gen *tscommit.Generator // The generator used to produce commit messages from diffs.
	mux *http.ServeMux      // The request multiplexer for the service endpoints.
}

// retrieveRequest holds the JSON payload for commit message generation.
type retrieveRequest struct {
	Diff string `json:"diff"` // The unified git diff to summarize.
}

// Service endpoints, request limits, and generation time budget.
const (
	generateEndpoint = "/commit/generate"                            // The endpoint for generating commit messages from diffs.
	healthEndpoint   = "/commit/healthz"                             // The liveness and readiness probe endpoint.
	maxBodyBytes     = 128 << 10                                     // The maximum request body size in bytes (128 KiB).
	generateTimeout  = tscommit.MaxGenerateDuration + 15*time.Second // The request deadline with grace above the generation budget.
)

// decodeDiff returns the original diff from the bounded JSON request body, or
// an HTTP error status and error if the payload is invalid or the diff is blank.
func decodeDiff(w http.ResponseWriter, r *http.Request) (string, int, error) {
	// Decode a single JSON payload within the request size limit.
	req, status, err := lpsrv.DecodeJSON[retrieveRequest](w, r, maxBodyBytes)
	// Preserve the codec's status and error on failure.
	if err != nil {
		// Return no diff for an invalid payload.
		return "", status, err
	}
	// Reject empty or whitespace-only diffs.
	if strings.TrimSpace(req.Diff) == "" {
		// Report the missing diff as a client error.
		return "", http.StatusBadRequest, tserr.Empty("diff")
	}
	// Preserve whitespace in the original diff.
	return req.Diff, 0, nil
}

// NewCommitServer returns a CommitServer secured with the bearer token tok and
// backed by an environment-configured Generator, or an error if tok is invalid
// or the Generator cannot be constructed.
func NewCommitServer(tok string) (*CommitServer, error) {
	// Validate the configured bearer token before constructing the generator.
	if err := lpsrv.ValidateBearerToken(tok); err != nil {
		// Return the token validation error.
		return nil, err
	}
	// Build the generator and validate its provider configuration.
	gen, err := tscommit.NewGenerator(tscommit.NewConfig())
	// Return the generator construction failure to the caller.
	if err != nil {
		// Do not expose a partially initialized server.
		return nil, err
	}
	// Construct the server with its generator and router.
	s := &CommitServer{gen: gen, mux: http.NewServeMux()}
	// Register the unauthenticated health probe.
	s.mux.Handle(healthEndpoint, lpsrv.HealthHandler())
	// Require authentication before handling commit generation requests.
	s.mux.Handle(generateEndpoint, lpsrv.RequireBearerAuth(tok, http.HandlerFunc(s.retrieveHandler)))
	// Return the configured server.
	return s, nil
}

// ServeHTTP dispatches r to the registered endpoint and writes its response to w.
func (s *CommitServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Delegate routing to the multiplexer.
	s.mux.ServeHTTP(w, r)
}

// generateMessage returns a generated message for diff. Tests swap this seam
// to avoid contacting the provider.
var generateMessage = func(ctx context.Context, gen *tscommit.Generator, diff string) (tscommit.Message, error) {
	// Delegate to the generator.
	return gen.Generate(ctx, diff)
}

// runServer serves handler with opts until ctx is canceled or the server fails.
// Tests swap this seam to avoid opening a network listener.
var runServer = lpsrv.RunContext

// Run starts the environment-configured CommitServer with a background context.
func Run() error {
	// Delegate to RunContext with a background context.
	return RunContext(context.Background())
}

// RunContext serves the CommitServer on the Cloud Run PORT and drains in-flight
// requests when ctx is canceled. It returns an error if ctx is nil or service
// configuration, startup, or shutdown fails.
func RunContext(ctx context.Context) error {
	// Reject a nil context before reading configuration.
	if ctx == nil {
		// Return an explicit missing-context error.
		return tserr.Empty("context")
	}
	// Read the bearer token protecting the generation endpoint.
	tok, err := lpsrv.RequireEnv("API_TOKEN")
	// Fail startup if the token is missing or blank.
	if err != nil {
		// Return the environment configuration error.
		return err
	}
	// Construct the HTTP handler from the service configuration.
	handler, err := NewCommitServer(tok)
	// Fail before opening a listener if the handler cannot be constructed.
	if err != nil {
		// Add context to the handler construction error.
		return tserr.Op(&tserr.OpArgs{Op: "new commit server", Fn: "CommitServer", Err: err})
	}
	// Keep the service's generation budget; the toolkit supplies the socket and drain defaults.
	return runServer(ctx, handler, lpsrv.WithName("CommitServer"), lpsrv.WithRequestTimeout(generateTimeout))
}

// retrieveHandler writes a generated commit message to w as JSON for a valid
// POST request in r, or writes an HTTP error response.
func (s *CommitServer) retrieveHandler(w http.ResponseWriter, r *http.Request) {
	// Allow only POST requests for generation.
	if !lpsrv.RequireMethod(w, r, "CommitServer", http.MethodPost) {
		// Stop processing the disallowed request.
		return
	}
	// Decode and validate the bounded request body.
	diff, status, err := decodeDiff(w, r)
	// Return the appropriate HTTP error for an invalid request.
	if err != nil {
		// Write the validation error with its HTTP status.
		lpsrv.WriteError(w, status, err)
		// Stop processing the invalid request.
		return
	}
	// Pass along the request context, including the deadline set by the HTTP server.
	res, err := generateMessage(r.Context(), s.gen, diff)
	// Respond with a sanitized error if generation fails.
	if err != nil {
		// Log the failure server-side and send only the status text to the client.
		lpsrv.WriteError(w, http.StatusInternalServerError, err)
		// Stop processing after writing the error response.
		return
	}
	// Encode the generated message and write the JSON response.
	lpsrv.WriteJSON(w, r, http.StatusOK, res)
}
