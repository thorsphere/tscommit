// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package srv_test

// Import packages
import (
	"context"           // Import the context package for generation and lifecycle tests.
	"errors"            // Import the errors package for sentinel construction and inspection.
	"math"              // Import the math package for an unencodable response value.
	"net/http"          // Import the net/http package for request, handler, and status types.
	"net/http/httptest" // Import the net/http/httptest package for in-process HTTP requests.
	"strings"           // Import the strings package for constructing and checking payloads.
	"testing"           // Import the testing package for subtests and assertions.
	"time"              // Import the time package for deadline and timeout checks.

	"github.com/thorsphere/lpsrv"                 // Import the lpsrv package for inspecting lifecycle options without a listener.
	"github.com/thorsphere/tscommit"              // Import the tscommit package for generator and message types.
	"github.com/thorsphere/tscommit/internal/srv" // Import the srv package under test.
	"github.com/thorsphere/tserr"                 // Import the tserr package for test errors.
)

// checkString reports a mismatch between got and want for field.
func checkString(t *testing.T, field, got, want string) {
	// Attribute any failure to the caller.
	t.Helper()
	// Compare the actual string to the expected value.
	if got != want {
		// Report the mismatch.
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: field, Actual: got, Want: want}))
	}
}

// TestNewCommitServer verifies that construction validates credentials before
// serving requests and never returns a partially configured server.
func TestNewCommitServer(t *testing.T) {
	// Define valid and invalid service configurations.
	cases := []struct {
		name    string // The scenario name.
		token   string // The bearer token passed to the constructor.
		key     string // The provider API key installed in the environment.
		wantErr string // The expected diagnostic substring, or empty on success.
	}{
		{"valid credentials", "swordfish", "sk-or-v1-test-key", ""},
		{"empty token", "", "", "token cannot be empty"},
		{"blank token", " \t\n", "sk-or-v1-test-key", "token cannot be empty"},
		{"invalid token", "two words", "", "RFC 6750"},
		{"missing provider key", "swordfish", "", "new client"},
		{"invalid provider key", "swordfish", "bad\nkey", "new client"},
	}
	// Exercise each configuration.
	for _, tc := range cases {
		// Isolate the environment for this case.
		t.Run(tc.name, func(t *testing.T) {
			// Set the provider key before constructing the server.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Construct the server under test.
			handler, err := srv.NewCommitServer(tc.token)
			// Check the success case separately from failure cases.
			if tc.wantErr == "" {
				// Reject an unexpected construction error.
				if err != nil {
					// Report the unexpected error.
					t.Fatal(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewCommitServer", Err: err}))
				}
				// Ensure the caller receives a usable handler.
				if handler == nil {
					// Report the missing handler.
					t.Fatal(tserr.Empty("handler"))
				}
				// Finish the successful case.
				return
			}
			// Require an error on invalid configuration.
			if err == nil {
				// Report the missing error.
				t.Fatal(tserr.NilFailed("NewCommitServer"))
			}
			// Verify that the intended validation failed.
			if !strings.Contains(err.Error(), tc.wantErr) {
				// Report the unexpected diagnostic.
				checkString(t, "construction error", err.Error(), tc.wantErr)
			}
			// Ensure no server is returned with an error.
			if handler != nil {
				// Report the unexpected handler.
				t.Errorf("handler = %v, want nil", handler)
			}
		})
	}
}

// TestCommitServerRoutes verifies the public HTTP contract through the real
// middleware and codecs, without a listener or provider call.
func TestCommitServerRoutes(t *testing.T) {
	// Configure the generator for construction without a real provider call.
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test-key")
	// Construct the handler with the test token.
	handler, err := srv.NewCommitServer("swordfish")
	// Stop if the test server could not be constructed.
	if err != nil {
		// Report the unexpected construction error.
		t.Fatal(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewCommitServer", Err: err}))
	}
	// Define the common response and request values.
	const (
		generate  = "/commit/generate"                 // The generation route under test.
		health    = "/commit/healthz"                  // The health route under test.
		bearer    = "Bearer swordfish"                 // The configured credential.
		validBody = `{"diff":" diff --git a/x b/x\n"}` // The request with significant whitespace.
		diff      = " diff --git a/x b/x\n"            // The original decoded diff.
		internal  = "Internal Server Error\n"          // The sanitized HTTP 500 response body.
	)
	// Define the provider response expected from successful generation.
	msg := tscommit.Message{Text: "feat(srv): add endpoint", Confidence: 0.9, Model: "test-model", TotalTokens: 320, Cost: 0.0025}
	// Construct a request that fills the body limit exactly.
	limitDiff := strings.Repeat("x", (128<<10)-len(`{"diff":""}`))
	// Describe requests and the service-visible behavior they must produce.
	cases := []struct {
		name       string           // The scenario name.
		method     string           // The HTTP method sent to the server.
		path       string           // The route sent to the server.
		auth       []string         // The Authorization header values.
		body       string           // The raw request body.
		wantStatus int              // The expected HTTP status.
		wantAllow  string           // The expected Allow header.
		wantAuth   string           // The expected authentication challenge.
		wantBody   string           // The expected exact response body when checkBody is set.
		checkBody  bool             // Whether to compare the entire response body.
		wantDetail string           // A diagnostic expected within the structured client error.
		wantDiff   string           // The diff passed to generation, or empty for no call.
		genErr     error            // The simulated generation failure.
		genMessage tscommit.Message // The simulated provider response.
	}{
		{name: "health GET", method: http.MethodGet, path: health, wantStatus: 200, wantBody: "ok\n", checkBody: true},
		{name: "health HEAD", method: http.MethodHead, path: health, wantStatus: 200, checkBody: true},
		{name: "health method rejected", method: http.MethodPost, path: health, wantStatus: 405, wantAllow: "GET, HEAD"},
		{name: "unknown route", method: http.MethodGet, path: "/other", wantStatus: 404},
		{name: "authentication before method check", method: http.MethodGet, path: generate, wantStatus: 401, wantAuth: "Bearer"},
		{name: "incorrect token", method: http.MethodPost, path: generate, auth: []string{"Bearer wrong"}, body: validBody, wantStatus: 401, wantAuth: "Bearer"},
		{name: "multiple credentials", method: http.MethodPost, path: generate, auth: []string{bearer, bearer}, body: validBody, wantStatus: 401, wantAuth: "Bearer"},
		{name: "only POST can generate", method: http.MethodGet, path: generate, auth: []string{bearer}, wantStatus: 405, wantAllow: "POST"},
		{name: "empty body", method: http.MethodPost, path: generate, auth: []string{bearer}, wantStatus: 400},
		{name: "invalid JSON", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":`, wantStatus: 400},
		{name: "incorrect field type", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":42}`, wantStatus: 400},
		{name: "unknown field", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":"x","extra":1}`, wantStatus: 400},
		{name: "second JSON value", method: http.MethodPost, path: generate, auth: []string{bearer}, body: validBody + `{}`, wantStatus: 400},
		{name: "missing diff", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{}`, wantStatus: 400, wantDetail: "diff cannot be empty"},
		{name: "blank diff", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":" \n\t"}`, wantStatus: 400, wantDetail: "diff cannot be empty"},
		{name: "exact body limit", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":"` + limitDiff + `"}`, wantStatus: 200, wantDiff: limitDiff, genMessage: msg},
		{name: "one byte over limit", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":"` + limitDiff + `x"}`, wantStatus: 413},
		{name: "padding over limit", method: http.MethodPost, path: generate, auth: []string{bearer}, body: `{"diff":"` + limitDiff + `"} `, wantStatus: 413},
		{name: "generation failure is sanitized", method: http.MethodPost, path: generate, auth: []string{bearer}, body: validBody, wantStatus: 500, wantBody: internal, checkBody: true, wantDiff: diff, genErr: errors.New("private provider diagnostic")},
		{name: "encoding failure is sanitized", method: http.MethodPost, path: generate, auth: []string{bearer}, body: validBody, wantStatus: 500, wantBody: internal, checkBody: true, wantDiff: diff, genMessage: tscommit.Message{Confidence: math.NaN()}},
		{name: "successful generation", method: http.MethodPost, path: generate, auth: []string{"bearer swordfish"}, body: validBody + " \n", wantStatus: 200, wantBody: `{"text":"feat(srv): add endpoint","confidence":0.9,"model":"test-model","total_tokens":320,"cost":0.0025}`, checkBody: true, wantDiff: diff, genMessage: msg},
	}
	// Exercise each case through the registered routes.
	for _, tc := range cases {
		// Run the scenario without parallelism because it replaces a seam.
		t.Run(tc.name, func(t *testing.T) {
			// Create a request with a deadline that generation should inherit.
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			// Release the deadline timer after the request.
			defer cancel()
			// Build the request with the caller's context.
			r := httptest.NewRequestWithContext(ctx, tc.method, tc.path, strings.NewReader(tc.body))
			// Add all Authorization values from the test case.
			for _, auth := range tc.auth {
				// Add a single credential header value.
				r.Header.Add("Authorization", auth)
			}
			// Record generation calls for the current request.
			calls := 0
			// Replace the provider seam to capture input and return the scripted result.
			t.Cleanup(srv.SetGenerateMessage(func(gotCtx context.Context, gen *tscommit.Generator, gotDiff string) (tscommit.Message, error) {
				// Record the invocation.
				calls++
				// Check that the request context reaches generation.
				if gotCtx != ctx {
					// Report an unexpected context replacement.
					t.Error("generation did not receive the request context")
				}
				// Require a configured generator.
				if gen == nil {
					// Report the missing generator.
					t.Error(tserr.Empty("generator"))
				}
				// Verify that validation did not alter the diff.
				checkString(t, "diff", gotDiff, tc.wantDiff)
				// Return the provider result for the current scenario.
				return tc.genMessage, tc.genErr
			}))
			// Capture the HTTP response without a network connection.
			w := httptest.NewRecorder()
			// Send the request through the server.
			handler.ServeHTTP(w, r)
			// Check the status code at the HTTP boundary.
			if w.Code != tc.wantStatus {
				// Report the status mismatch.
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			// Check the protocol headers for method and authentication errors.
			checkString(t, "Allow", w.Header().Get("Allow"), tc.wantAllow)
			// Check that unauthorized requests include the bearer challenge.
			checkString(t, "WWW-Authenticate", w.Header().Get("WWW-Authenticate"), tc.wantAuth)
			// Check the exact response body where the service contract specifies it.
			if tc.checkBody {
				// Report any body mismatch.
				checkString(t, "response body", w.Body.String(), tc.wantBody)
			}
			// Check client error details without depending on the generated error ID.
			if tc.wantDetail != "" && !strings.Contains(w.Body.String(), tc.wantDetail) {
				// Report the missing diagnostic.
				checkString(t, "client error detail", w.Body.String(), tc.wantDetail)
			}
			// Only a valid generation request should invoke the provider once.
			if (tc.wantDiff == "" && calls != 0) || (tc.wantDiff != "" && calls != 1) {
				// Report unexpected generation activity.
				t.Errorf("generation calls = %d, want one call: %t", calls, tc.wantDiff != "")
			}
			// Verify the response media type for generated messages.
			if tc.wantDiff != "" && tc.wantStatus == http.StatusOK {
				// Require successful generation to return JSON.
				checkString(t, "Content-Type", w.Header().Get("Content-Type"), "application/json")
			}
		})
	}
}

// TestRunContext verifies service configuration and delegation to lpsrv
// without starting a listener.
func TestRunContext(t *testing.T) {
	// Define a recognizable error returned by the lifecycle seam.
	lifecycleErr := errors.New("listener failed")
	// Define startup and delegation scenarios.
	cases := []struct {
		name        string // The scenario name.
		token       string // The configured API_TOKEN value.
		key         string // The configured provider API key.
		nilContext  bool   // Whether to pass a nil context.
		useRun      bool   // Whether to start with Run's background context.
		wantRun     bool   // Whether startup should delegate to the lifecycle seam.
		runErr      error  // The error returned from the lifecycle seam.
		wantErrText string // The expected diagnostic substring.
	}{
		{name: "nil context", nilContext: true, wantErrText: "context cannot be empty"},
		{name: "missing token", wantErrText: "API_TOKEN"},
		{name: "blank token", token: " \t", wantErrText: "API_TOKEN"},
		{name: "invalid token", token: "two words", wantErrText: "RFC 6750"},
		{name: "missing provider key", token: "swordfish", wantErrText: "new commit server"},
		{name: "successful lifecycle", token: "swordfish", key: "sk-or-v1-test-key", wantRun: true},
		{name: "background lifecycle error", token: "swordfish", key: "sk-or-v1-test-key", useRun: true, wantRun: true, runErr: lifecycleErr, wantErrText: "listener failed"},
	}
	// Exercise each startup scenario.
	for _, tc := range cases {
		// Isolate environment and seam changes for the scenario.
		t.Run(tc.name, func(t *testing.T) {
			// Install the service credentials.
			t.Setenv("API_TOKEN", tc.token)
			// Install the provider credentials.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Give RunContext a distinguishable context for forwarding checks.
			ctx, cancel := context.WithCancel(context.Background())
			// Release the context when the scenario ends.
			defer cancel()
			// Record whether startup reached the toolkit.
			runCalled := false
			// Replace the listener lifecycle with a check of the delegated configuration.
			t.Cleanup(srv.SetRunServer(func(gotCtx context.Context, handler http.Handler, opts ...lpsrv.Option) error {
				// Record the lifecycle invocation.
				runCalled = true
				// Expect the scenario's context unless Run supplied its own.
				wantCtx := ctx
				// Account for the background-context wrapper.
				if tc.useRun {
					// Expect Run's background context.
					wantCtx = context.Background()
				}
				// Check the context passed to the lifecycle runner.
				if gotCtx != wantCtx {
					// Report an unexpected context.
					t.Errorf("run context = %v, want %v", gotCtx, wantCtx)
				}
				// Instantiate the configured server without starting it.
				server, err := lpsrv.NewHTTPServer(":8080", handler, opts...)
				// Check that the delegated handler and options are usable.
				if err != nil {
					// Report unexpected configuration failure.
					t.Fatal(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewHTTPServer", Err: err}))
				}
				// Compute the expected request and socket write budgets.
				wantRequest := tscommit.MaxGenerateDuration + 15*time.Second
				// Check that the socket write timeout leaves grace after generation.
				if wantWrite := wantRequest + 15*time.Second; server.WriteTimeout != wantWrite {
					// Report an unexpected socket write timeout.
					t.Errorf("WriteTimeout = %v, want %v", server.WriteTimeout, wantWrite)
				}
				// Record the earliest possible request deadline.
				earliestDeadline := time.Now().Add(wantRequest)
				// Check the request deadline that the configured server installs.
				t.Cleanup(srv.SetGenerateMessage(func(reqCtx context.Context, _ *tscommit.Generator, _ string) (tscommit.Message, error) {
					// Read the request deadline.
					deadline, ok := reqCtx.Deadline()
					// Ensure the request got the service budget rather than the toolkit default.
					if latestDeadline := time.Now().Add(wantRequest); !ok || deadline.Before(earliestDeadline) || deadline.After(latestDeadline) {
						// Report the unexpected deadline.
						t.Errorf("request deadline present: %t, deadline: %v, want within [%v, %v]", ok, deadline, earliestDeadline, latestDeadline)
					}
					// Return a serializable response without contacting the provider.
					return tscommit.Message{}, nil
				}))
				// Build an authenticated request through the configured lifecycle handler.
				r := httptest.NewRequest(http.MethodPost, "/commit/generate", strings.NewReader(`{"diff":"x"}`))
				// Authenticate with the configured API token.
				r.Header.Set("Authorization", "Bearer "+tc.token)
				// Capture the generation response.
				w := httptest.NewRecorder()
				// Invoke the server handler with the installed middleware.
				server.Handler.ServeHTTP(w, r)
				// Check that the configured handler can serve the generation route.
				if w.Code != http.StatusOK {
					// Report a broken handler configuration.
					t.Errorf("configured server status = %d, want %d", w.Code, http.StatusOK)
				}
				// Return the scripted lifecycle result.
				return tc.runErr
			}))
			// Pass a nil context when requested by the scenario.
			if tc.nilContext {
				// Replace the context explicitly.
				ctx = nil
			}
			// Capture the error from the selected entry point.
			var err error
			// Select Run or RunContext for this scenario.
			if tc.useRun {
				// Run with a background context.
				err = srv.Run()
			} else {
				// Run with the supplied context.
				err = srv.RunContext(ctx)
			}
			// Compare whether the toolkit was called.
			if runCalled != tc.wantRun {
				// Report unexpected lifecycle delegation.
				t.Errorf("lifecycle called = %t, want %t", runCalled, tc.wantRun)
			}
			// Verify lifecycle failures are preserved for the caller.
			if tc.runErr != nil && !errors.Is(err, tc.runErr) {
				// Report a dropped lifecycle error.
				t.Errorf("Run error = %v, want %v", err, tc.runErr)
			}
			// Check for an unexpected startup failure.
			if tc.wantErrText == "" && err != nil {
				// Report the unexpected error.
				t.Fatal(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "RunContext", Err: err}))
			}
			// Check the expected diagnostic in failure scenarios.
			if tc.wantErrText != "" {
				// Require an error for a failure scenario.
				if err == nil {
					// Report the missing startup error.
					t.Fatal(tserr.NilFailed("RunContext"))
				}
				// Verify the intended failure surfaced.
				if !strings.Contains(err.Error(), tc.wantErrText) {
					// Report the unexpected diagnostic.
					checkString(t, "startup error", err.Error(), tc.wantErrText)
				}
			}
		})
	}
}
