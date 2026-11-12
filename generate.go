// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

// Package tscommit generates conventional git commit messages from diffs
// via a configured AI provider, and optionally drives interactive user review
// and local commit creation. It is a component-agnostic library: the AI
// provider API key is read from the environment, and no state is persisted
// locally.
package tscommit

// Import packages
import (
	"context" // Import the context package for cancellation and deadlines.
	"strings" // Import the strings package for blank-diff validation.

	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI client interactions.
	"github.com/thorsphere/tserr"      // Import the tserr package for error reporting.
)

// Message holds a generated Conventional Commits message and its model
// confidence score. The zero value represents an empty message with no
// confidence.
type Message struct {
	Text       string  // The generated commit message.
	Confidence float64 // The model confidence score in [0, 1].
}

// Generator generates Conventional Commits messages from unified diffs via a
// configured AI client and a single sticky routing key so retries and
// regenerations land on the same provider endpoint and reuse the prompt cache.
// It holds no per-request state, so one instance may safely serve many calls.
// The zero value is not usable; construct instances with NewGenerator.
type Generator struct {
	client    *tsaiclient.Client // The configured AI client used for completions.
	sessionID string             // The sticky routing key shared across calls on this generator.
}

// NewGenerator returns a Generator configured from the environment with a
// freshly minted sticky routing key, or an error if the AI provider or API key
// configuration is invalid.
func NewGenerator() (*Generator, error) {
	// Build the AI client; construction validates the provider and API key.
	client, err := newClient()
	// Return the error if the AI client cannot be built.
	if err != nil {
		return nil, err
	}
	// Mint one sticky routing key for the lifetime of the generator.
	sessionID := newSessionID()
	// Return the initialized generator and nil error to indicate success.
	return &Generator{
		client:    client,
		sessionID: sessionID,
	}, nil
}

// Generate returns a validated Conventional Commits Message and model
// confidence score for diff using the generator's sticky routing key. It is
// purely non-interactive: it never prompts the user, touches the filesystem,
// invokes git, or wires HTTP handlers. Callers are responsible for bounding
// diff size before calling Generate (for example, truncating git output in the
// CLI or enforcing an HTTP request-body limit such as http.MaxBytesReader in a
// service handler to reject oversized diffs with 400 Bad Request). It returns
// an error if g is nil or uninitialized, ctx is nil, diff is empty or
// whitespace-only, or message generation or validation fails.
func (g *Generator) Generate(ctx context.Context, diff string) (Message, error) {
	// Return an error if the generator is nil.
	if g == nil {
		return Message{}, tserr.NilPtr()
	}
	// Return an error if the generator was not initialized via NewGenerator.
	if g.client == nil {
		return Message{}, tserr.Empty("AI client")
	}
	// Return an error if the context is nil.
	if ctx == nil {
		return Message{}, tserr.Empty("context")
	}
	// Return an error if the diff is empty or whitespace-only.
	if strings.TrimSpace(diff) == "" {
		return Message{}, tserr.Empty("diff")
	}
	// Generate and validate the commit message using the sticky session ID.
	text, confidence, err := genMessage(ctx, g.client, diff, g.sessionID)
	// Return the error if generation or validation fails.
	if err != nil {
		return Message{}, err
	}
	// Return the validated message and confidence score.
	return Message{
		Text:       text,
		Confidence: confidence,
	}, nil
}
