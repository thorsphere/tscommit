// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package srv

// Import packages
import (
	"context"  // Import the context package for the generation seam signature.
	"net/http" // Import the HTTP package for handling HTTP requests and responses.

	"github.com/thorsphere/tscommit" // Import the tscommit package for the generator and message types.
)

// Exports for the external test package: bridges to unexported identifiers.
var (
	DecodeDiff   = decodeDiff   // Decodes and validates the JSON diff payload from a request.
	MaxBodyBytes = maxBodyBytes // The maximum allowed HTTP request body size in bytes.
)

// NewRequireAuth returns the requireAuth-wrapped form of next on a server
// carrying token, so tests can exercise authentication without building a
// Generator. A blank token exercises the uninitialized-token rejection branch.
func NewRequireAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	// Construct a server carrying only the token under test.
	s := &CommitServer{tok: token}
	// Return the authentication-wrapped next handler.
	return s.requireAuth(next)
}

// NewRetrieveHandler returns the retrieveHandler of a server carrying gen, so
// tests can exercise the generation endpoint without routing through
// requireAuth. A nil gen exercises the uninitialized-generator rejection
// branch.
func NewRetrieveHandler(gen *tscommit.Generator) http.HandlerFunc {
	// Construct a server carrying only the generator under test.
	s := &CommitServer{gen: gen}
	// Return the retrieval handler.
	return s.retrieveHandler
}

// SetGenerateMessage swaps the generation seam for tests and returns a restore
// function (pass it to t.Cleanup). Tests using it must not call t.Parallel.
func SetGenerateMessage(fn func(ctx context.Context, gen *tscommit.Generator, diff string) (tscommit.Message, error)) func() {
	// Save the current generation seam.
	prev := generateMessage
	// Set the new generation seam.
	generateMessage = fn
	// Return a function to restore the previous generation seam.
	return func() { generateMessage = prev }
}

// SetListenAndServe swaps the HTTP listener seam for tests and returns a
// restore function (pass it to t.Cleanup). Tests using it must not call
// t.Parallel.
func SetListenAndServe(fn func(srv *http.Server) error) func() {
	// Save the current HTTP listener seam.
	prev := listenAndServe
	// Set the new HTTP listener seam.
	listenAndServe = fn
	// Return a function to restore the previous HTTP listener seam.
	return func() { listenAndServe = prev }
}

// SetShutdownServer swaps the HTTP server shutdown seam for tests and returns a
// restore function (pass it to t.Cleanup). Tests using it must not call
// t.Parallel.
func SetShutdownServer(fn func(ctx context.Context, srv *http.Server) error) func() {
	// Save the current HTTP shutdown seam.
	prev := shutdownServer
	// Set the new HTTP shutdown seam.
	shutdownServer = fn
	// Return a function to restore the previous HTTP shutdown seam.
	return func() { shutdownServer = prev }
}
