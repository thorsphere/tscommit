// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit_test

// Import packages
import (
	"context" // Import the context package for context handling.
	"errors"  // Import the errors package for error handling.
	"fmt"     // Import the fmt package for formatted diagnostics.
	"testing" // Import the testing package for unit tests.

	"github.com/thorsphere/tsaiclient" // Import the tsaiclient package for AI completion requests and responses.
	"github.com/thorsphere/tscommit"   // Import the tscommit package for the generator and AI completion seam.
)

// chatTurn is one scripted provider response: a canned result and an error.
type chatTurn struct {
	res tsaiclient.JSONResult // The canned structured result for this call.
	err error                 // The canned error for this call.
}

// okTurn returns a scripted chatTurn carrying a single-finding envelope with
// msg and confidence.
func okTurn(msg string, confidence float64) chatTurn {
	// Wrap the message and confidence in a single-finding structured result.
	return chatTurn{
		res: tsaiclient.JSONResult{
			Envelope: tsaiclient.Envelope{
				Findings:   []string{msg},
				Confidence: confidence,
			},
		},
	}
}

// fakeChat implements the AI completion seam without a provider. It records
// every request and replays scripted turns in order.
type fakeChat struct {
	calls  int        // The number of times the seam was invoked.
	reqs   []string   // The recorded request renderings, one per call.
	script []chatTurn // The canned turns replayed in order.
}

// run records the request and replays the next scripted turn.
func (f *fakeChat) run(_ context.Context, _ *tsaiclient.Client, req *tsaiclient.Request) (tsaiclient.JSONResult, error) {
	// Record the rendered request so prompts and session IDs can be asserted.
	f.reqs = append(f.reqs, fmt.Sprintf("%+v", req))
	// Count the invocation.
	f.calls++
	// Return an error when the script is exhausted; the call-count
	// assertion reports the over-invocation.
	if f.calls > len(f.script) {
		// Return an error for the unexpected extra call.
		return tsaiclient.JSONResult{}, errors.New("fakeChat: script exhausted")
	}
	// Replay the next scripted turn.
	turn := f.script[f.calls-1]
	// Return the canned result and error.
	return turn.res, turn.err
}

// useFakeChat swaps the AI completion seam for the duration of the test.
// Tests using it must not call t.Parallel.
func useFakeChat(t *testing.T, f *fakeChat) {
	// Mark the test function as test helper function.
	t.Helper()
	// Swap the completion seam for the duration of the test and undo the
	// swap at test end.
	t.Cleanup(tscommit.SetCompleteChat(f.run))
}
