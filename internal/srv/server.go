// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

// Package srv implements the HTTP server handler for health probes and
// authenticated Conventional Commits message generation from unified diffs.
package srv

// Import packages
import (
	"context"       // Import the context package for request deadlines and cancellation.
	"crypto/subtle" // Import the crypto/subtle package for constant-time token comparison.
	"encoding/json" // Import the encoding/json package for JSON request decoding and response encoding.
	"errors"        // Import the errors package for error inspection and construction.
	"io"            // Import the io package for EOF checks and response body writes.
	"net/http"      // Import the net/http package for HTTP routing, middleware, and handlers.
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

// GenerateTimeout is the per-request generation deadline, derived from
// tscommit.MaxGenerateDuration with a grace buffer so provider timeouts
// surface cleanly before the handler context cancels.
const GenerateTimeout = tscommit.MaxGenerateDuration + 15*time.Second

// Server endpoints and request limits.
const (
	generateEndpoint = "/commit/generate" // The endpoint for generating commit messages from diffs.
	healthEndpoint   = "/commit/healthz"  // The liveness and readiness probe endpoint.
	maxBodyBytes     = 128 << 10          // The maximum allowed HTTP request body size in bytes (128 KiB).
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
	ctx, cancel := context.WithTimeout(r.Context(), GenerateTimeout)
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
