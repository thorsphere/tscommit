// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package srv

// Import packages
import (
	"context"  // Import the context package for the seam signatures.
	"net/http" // Import the net/http package for the handler type.

	"github.com/thorsphere/lpsrv"    // Import the lpsrv package for the server option type.
	"github.com/thorsphere/tscommit" // Import the tscommit package for generator and message types.
)

// SetGenerateMessage replaces the generation seam and returns a restore function.
func SetGenerateMessage(fn func(context.Context, *tscommit.Generator, string) (tscommit.Message, error)) func() {
	// Save the original seam.
	prev := generateMessage
	// Install the test seam.
	generateMessage = fn
	// Return a function that restores generation.
	return func() { generateMessage = prev }
}

// SetRunServer replaces the lifecycle seam and returns a restore function.
func SetRunServer(fn func(context.Context, http.Handler, ...lpsrv.Option) error) func() {
	// Save the original seam.
	prev := runServer
	// Install the test seam.
	runServer = fn
	// Return a function that restores the lifecycle runner.
	return func() { runServer = prev }
}
