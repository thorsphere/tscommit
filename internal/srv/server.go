// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

// Package srv implements the HTTP server handler and Cloud Run lifecycle for
// health probes and authenticated Conventional Commits message generation from
// unified diffs.
package srv

// Import packages
import (
	"context"       // Import the context package for request deadlines and cancellation.
	"crypto/subtle" // Import the crypto/subtle package for constant-time token comparison.
	"encoding/json" // Import the encoding/json package for JSON request decoding and response encoding.
	"errors"        // Import the errors package for error inspection and construction.
	"fmt"           // Import the fmt package for formatted log messages.
	"io"            // Import the io package for EOF checks and response body writes.
	"net/http"      // Import the net/http package for HTTP routing, middleware, and handlers.
	"os"            // Import the os package for reading environment variables.
	"strings"       // Import the strings package for whitespace validation.
	"time"          // Import the time package for timeout durations.

	"github.com/thorsphere/tscommit" // Import the tscommit package for non-interactive commit message generation.
	"github.com/thorsphere/tserr"    // Import the tserr package for structured error construction.
	"github.com/thorsphere/tslog"    // Import the tslog package for structured logging.
)

// CommitServer serves HTTP endpoints for unauthenticated health probes and
// bearer-authenticated Conventional Commits message generation. The zero value
// is not usable; construct instances with NewCommitServer.
type CommitServer struct {
	gen *tscommit.Generator // The generator used to produce commit messages from diffs.
	mux *http.ServeMux      // The HTTP request multiplexer routing incoming requests.
	tok string              // The bearer token required on authenticated endpoints.
}

// retrieveRequest holds the JSON request payload for commit message generation.
type retrieveRequest struct {
	Diff string `json:"diff"` // The unified git diff to summarize.
}

// Server endpoints, request limits, and Cloud Run HTTP lifecycle defaults.
const (
	generateEndpoint  = "/commit/generate"                            // The endpoint for generating commit messages from diffs.
	healthEndpoint    = "/commit/healthz"                             // The liveness and readiness probe endpoint.
	maxBodyBytes      = 128 << 10                                     // The maximum allowed HTTP request body size in bytes (128 KiB).
	srvPort           = "8080"                                        // The fallback port when the PORT environment variable is unset.
	srvAddr           = ":"                                           // The host prefix that binds the server to all interfaces.
	generateTimeout   = tscommit.MaxGenerateDuration + 15*time.Second // The per-request generation deadline with a grace buffer above MaxGenerateDuration.
	readHeaderTimeout = 5 * time.Second                               // The maximum duration for reading request headers.
	readTimeout       = 15 * time.Second                              // The maximum duration for reading the entire request body.
	writeTimeout      = generateTimeout + 15*time.Second              // The socket backstop derived above the handler generation deadline.
	idleTimeout       = 60 * time.Second                              // The maximum duration to keep idle keep-alive connections open.
	shutdownTimeout   = 8 * time.Second                               // The deadline for draining in-flight requests after cancellation.
)

// decodeDiff returns the validated diff from the JSON body of r, enforcing the
// request body size limit via w, or an HTTP error status code and error if the
// body exceeds maxBodyBytes, is invalid JSON, or carries an empty diff.
func decodeDiff(w http.ResponseWriter, r *http.Request) (string, int, error) {
	// Bound the request body size so oversized diffs are rejected without buffering.
	body := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	// Build a JSON decoder over the bounded body.
	dec := json.NewDecoder(body)
	// Reject unknown JSON fields so misspelled keys fail fast.
	dec.DisallowUnknownFields()
	// Prepare the target struct for the decoded JSON payload.
	var req retrieveRequest
	// Decode the JSON payload from the request body.
	if err := dec.Decode(&req); err != nil {
		// Prepare to match an oversized-body error from MaxBytesReader.
		var maxErr *http.MaxBytesError
		// Check whether the body exceeded maxBodyBytes.
		if errors.As(err, &maxErr) {
			// Reject oversized payloads with 413 Request Entity Too Large.
			return "", http.StatusRequestEntityTooLarge, tserr.TooLarge(&tserr.TooLargeArgs{
				F:        "request body",
				Actual:   maxErr.Limit + 1,
				MaxBytes: maxErr.Limit,
			})
		}
		// Reject malformed or empty JSON bodies with 400 Bad Request.
		return "", http.StatusBadRequest, tserr.InvalidJson(err)
	}
	// Check that no trailing data follows the single JSON object.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		// Prepare to match an oversized-body error from MaxBytesReader.
		var maxErr *http.MaxBytesError
		// Check whether trailing whitespace or data exceeded maxBodyBytes.
		if errors.As(err, &maxErr) {
			// Reject oversized payloads with 413 Request Entity Too Large.
			return "", http.StatusRequestEntityTooLarge, tserr.TooLarge(&tserr.TooLargeArgs{
				F:        "request body",
				Actual:   maxErr.Limit + 1,
				MaxBytes: maxErr.Limit,
			})
		}
		// Reject trailing tokens with 400 Bad Request.
		return "", http.StatusBadRequest, tserr.InvalidJson(errors.New("unexpected trailing data"))
	}
	// If the diff is empty or whitespace-only, return a 400 Bad Request error.
	if strings.TrimSpace(req.Diff) == "" {
		// Reject empty or whitespace-only diffs with 400 Bad Request.
		return "", http.StatusBadRequest, tserr.Empty("diff")
	}
	// Return the validated diff, zero status, and nil error to indicate success.
	return req.Diff, 0, nil
}

// NewCommitServer returns a CommitServer secured with the bearer token tok and
// backed by an environment-configured Generator, or an error if tok is empty
// or the Generator cannot be constructed.
func NewCommitServer(tok string) (*CommitServer, error) {
	// If tok is empty or whitespace-only, return an error before building dependencies.
	if strings.TrimSpace(tok) == "" {
		// Reject blank bearer tokens with an empty-token error.
		return nil, tserr.Empty("token")
	}
	// Build the non-interactive generator; construction validates the provider and API key.
	gen, err := tscommit.NewGenerator()
	// If the generator cannot be built, log and return the error.
	if err != nil {
		// Log the generator construction failure.
		tslog.Error(err)
		// Return nil server and the construction error.
		return nil, err
	}
	// Construct the CommitServer with the generator, multiplexer, and bearer token.
	s := &CommitServer{gen: gen, mux: http.NewServeMux(), tok: tok}
	// Register the unauthenticated health probe handler.
	s.mux.HandleFunc(healthEndpoint, s.healthHandler)
	// Register the authenticated commit message generation handler.
	s.mux.HandleFunc(generateEndpoint, s.requireAuth(s.retrieveHandler))
	// Return the configured server and nil error to indicate success.
	return s, nil
}

// healthHandler writes a 200 OK plain-text response to w for GET and HEAD
// health probes in r, and rejects all other HTTP methods with 405 Method Not
// Allowed.
func (s *CommitServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET and HEAD requests on the health probe endpoint.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		// Advertise GET and HEAD as the allowed methods per RFC 9110.
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		// Reject unsupported methods with 405 Method Not Allowed.
		http.Error(w, tserr.MethodNotAllowed(&tserr.MethodNotAllowedArgs{Method: r.Method, Resource: "health"}).Error(), http.StatusMethodNotAllowed)
		// Stop processing the disallowed request.
		return
	}
	// Set the plain-text content type for the health probe response.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Write the 200 OK status header.
	w.WriteHeader(http.StatusOK)
	// Omit the response body on HEAD requests per HTTP semantics.
	if r.Method == http.MethodHead {
		// Return without writing a response body for HEAD probes.
		return
	}
	// Write the plain-text health indicator body and log any write failure.
	if _, err := io.WriteString(w, "ok\n"); err != nil {
		// Log the response write error for diagnostics.
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "write health response", Fn: r.URL.Path, Err: err}))
	}
}

// requireAuth returns an http.HandlerFunc that invokes next when the incoming
// request carries s's bearer token in the Authorization header, and otherwise
// responds with 401 Unauthorized.
func (s *CommitServer) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	// Return a handler that enforces bearer token authentication before calling next.
	return func(w http.ResponseWriter, r *http.Request) {
		// Reject requests if the server carries an uninitialized or blank token.
		if strings.TrimSpace(s.tok) == "" {
			// Advertise the Bearer authentication scheme per RFC 6750.
			w.Header().Set("WWW-Authenticate", "Bearer")
			// Reject the request with 401 Unauthorized.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			// Stop processing the unauthenticated request.
			return
		}
		// Construct the expected Authorization header value from the configured token.
		expectedHeader := "Bearer " + s.tok
		// Read the Authorization header from the incoming request.
		got := r.Header.Get("Authorization")
		// Compare the header against the expected value in constant time to prevent timing side-channels.
		if subtle.ConstantTimeCompare([]byte(got), []byte(expectedHeader)) != 1 {
			// Advertise the Bearer authentication scheme per RFC 6750.
			w.Header().Set("WWW-Authenticate", "Bearer")
			// Reject the request with 401 Unauthorized.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			// Stop processing the unauthenticated request.
			return
		}
		// Forward the authenticated request to next.
		next(w, r)
	}
}

// ServeHTTP dispatches r to s's internal HTTP multiplexer and writes the
// response to w, satisfying http.Handler.
func (s *CommitServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route the request through the internal multiplexer.
	s.mux.ServeHTTP(w, r)
}

// generateMessage generates a commit message for diff with gen. It is the
// generation seam, swapped via SetGenerateMessage in export_test.go so unit
// tests script generation outcomes without a network call.
var generateMessage = func(ctx context.Context, gen *tscommit.Generator, diff string) (tscommit.Message, error) {
	// Delegate to the generator.
	return gen.Generate(ctx, diff)
}

// listenAndServe binds srv to its configured address and serves incoming HTTP
// requests. It is the listener seam, swapped via SetListenAndServe in
// export_test.go so unit tests never open a real network socket.
var listenAndServe = func(srv *http.Server) error {
	// Delegate to the standard library HTTP server listener.
	return srv.ListenAndServe()
}

// shutdownServer gracefully stops srv from accepting new connections and drains
// active requests within ctx. It is the shutdown seam, swapped via
// SetShutdownServer in export_test.go so unit tests script shutdown outcomes
// without a network socket.
var shutdownServer = func(ctx context.Context, srv *http.Server) error {
	// Delegate to the standard library HTTP server graceful shutdown.
	return srv.Shutdown(ctx)
}

// Run starts the environment-configured CommitServer and blocks until the
// listener fails with a background context.
func Run() error {
	// Delegate to RunContext with a background context.
	return RunContext(context.Background())
}

// RunContext reads PORT and API_TOKEN from the environment, constructs a
// CommitServer, serves HTTP requests on all interfaces, and gracefully drains
// in-flight work when ctx is canceled. It returns an error if ctx is nil,
// API_TOKEN is unset or blank, server construction fails, the listener fails,
// or graceful shutdown fails.
func RunContext(ctx context.Context) error {
	// Reject a nil context before starting any work.
	if ctx == nil {
		// Return an empty-context error when ctx is nil.
		return tserr.Empty("context")
	}
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
	// Fail fast if API_TOKEN is unset or whitespace-only.
	if strings.TrimSpace(token) == "" {
		// Return a not-set error when API_TOKEN is missing or blank.
		return tserr.NotSet("API_TOKEN")
	}
	// Construct the HTTP server handler wrapping the generator.
	handler, err := NewCommitServer(token)
	// Return the error if the server handler cannot be constructed.
	if err != nil {
		// Wrap the server construction failure with the target listen address.
		return tserr.Op(&tserr.OpArgs{Op: "new commit server", Fn: serverAddr, Err: err})
	}
	// Construct the HTTP server bound to the configured address and timeouts.
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	// Buffer a listener error so the server goroutine never blocks on send.
	errCh := make(chan error, 1)
	// Start the HTTP listener in a separate goroutine.
	go func() {
		// Log the address on which the server is starting.
		tslog.Info(fmt.Sprintf("Starting CommitServer on %v", serverAddr))
		// Serve incoming requests and forward unexpected listener errors.
		if err := listenAndServe(server); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Send the unexpected listener failure to RunContext.
			errCh <- err
		}
	}()
	// Wait until ctx is canceled or the listener fails unexpectedly.
	select {
	case err := <-errCh:
		// Return the wrapped listener error immediately.
		return tserr.Op(&tserr.OpArgs{Op: "ListenAndServe", Fn: serverAddr, Err: err})
	case <-ctx.Done():
	}
	// Log that graceful shutdown has started after context cancellation.
	tslog.Info("Shutting down server gracefully...")
	// Bound the graceful shutdown duration so draining requests cannot hang indefinitely.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	// Release the shutdown context resources when RunContext returns.
	defer cancel()
	// Attempt to gracefully shut down the server and drain active connections.
	if err := shutdownServer(shutdownCtx, server); err != nil {
		// Return the wrapped shutdown failure when draining does not complete cleanly.
		return tserr.Op(&tserr.OpArgs{Op: "Shutdown", Fn: serverAddr, Err: err})
	}
	// Log that the server has exited cleanly.
	tslog.Info("Server successfully exited")
	// Return no error to indicate a clean server lifecycle.
	return nil
}

// retrieveHandler handles POST requests in r carrying a JSON-encoded unified
// diff and writes the generated Conventional Commits tscommit.Message as JSON
// to w, or writes an HTTP error response if the method, payload, or generation
// fails.
func (s *CommitServer) retrieveHandler(w http.ResponseWriter, r *http.Request) {
	// If the generator is nil, reject the request with 500 Internal Server Error.
	if s.gen == nil {
		// Write a 500 Internal Server Error indicating an uninitialized generator.
		http.Error(w, tserr.Empty("Generator").Error(), http.StatusInternalServerError)
		// Stop processing on an uninitialized server.
		return
	}
	// Only allow POST requests for generating commit messages.
	if r.Method != http.MethodPost {
		// Advertise POST as the only allowed method per RFC 9110.
		w.Header().Set("Allow", http.MethodPost)
		// Reject unsupported methods with 405 Method Not Allowed.
		http.Error(w, tserr.MethodNotAllowed(&tserr.MethodNotAllowedArgs{Method: r.Method, Resource: "CommitServer"}).Error(), http.StatusMethodNotAllowed)
		// Stop processing the disallowed request.
		return
	}
	// Decode and validate the bounded JSON diff from the request body.
	diff, status, err := decodeDiff(w, r)
	// If request validation fails, respond with the corresponding client error status.
	if err != nil {
		// Write the validation error and client status code.
		http.Error(w, err.Error(), status)
		// Stop processing the invalid request.
		return
	}
	// Bound the total generation time so slow upstream calls cancel cleanly before WriteTimeout.
	ctx, cancel := context.WithTimeout(r.Context(), generateTimeout)
	// Release the timeout context resources when the handler returns.
	defer cancel()
	// Generate the commit message using the bounded request context.
	res, err := generateMessage(ctx, s.gen, diff)
	// If message generation fails, log the error and respond with 500 Internal Server Error.
	if err != nil {
		// Log the generation failure for server diagnostics.
		tslog.Error(err)
		// Write the 500 Internal Server Error response.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		// Stop processing after writing the error response.
		return
	}
	// Marshal the generated message before writing headers so encoding failures can still return 500.
	jsonBytes, err := json.Marshal(res)
	// If JSON encoding fails, log the error and respond with 500 Internal Server Error.
	if err != nil {
		// Log the JSON marshaling error for server diagnostics.
		tslog.Error(err)
		// Write the 500 Internal Server Error response before any 200 headers are sent.
		http.Error(w, tserr.Op(&tserr.OpArgs{Op: "json.Marshal", Fn: "result", Err: err}).Error(), http.StatusInternalServerError)
		// Stop processing after writing the error response.
		return
	}
	// Set the JSON content type header before writing the buffered payload.
	w.Header().Set("Content-Type", "application/json")
	// Write the JSON response body and log any write failure after headers are sent.
	if _, err := w.Write(jsonBytes); err != nil {
		// Log the response write error for diagnostics.
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "write response", Fn: r.URL.Path, Err: err}))
	}
}
