// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package srv_test

// Import packages
import (
	"context"           // Import the context package for the generation seam signature.
	"errors"            // Import the errors package for sentinel error construction.
	"fmt"               // Import the fmt package for formatting.
	"math"              // Import the math package for unencodable confidence values.
	"net/http"          // Import the net/http package for HTTP status codes and test requests.
	"net/http/httptest" // Import the net/http/httptest package for request and response fakes.
	"strings"           // Import the strings package for payload construction and text checks.
	"testing"           // Import the testing package for testing.

	"github.com/thorsphere/tscommit"              // Import the tscommit package for the generator and message types.
	"github.com/thorsphere/tscommit/internal/srv" // Import the srv package under test.
	"github.com/thorsphere/tserr"                 // Import the tserr package for error handling.
)

// decodeRequest returns the decoded diff, HTTP error status, and error from
// running DecodeDiff against a POST request carrying body.
func decodeRequest(body string) (string, int, error) {
	// Build a response recorder to satisfy the MaxBytesReader bound.
	w := httptest.NewRecorder()
	// Build a POST request carrying the raw JSON body.
	r := httptest.NewRequest(http.MethodPost, "/commit/generate", strings.NewReader(body))
	// Decode and validate the diff from the request body.
	return srv.DecodeDiff(w, r)
}

// TestDecodeDiff tests decodeDiff: valid payloads, malformed and trailing
// JSON, unknown fields, empty diffs, and request body size boundaries.
func TestDecodeDiff(t *testing.T) {
	// The JSON envelope surrounding the diff value.
	envelope := `{"diff":""}`
	// The number of bytes available for the diff at the body size limit.
	limitDiff := srv.MaxBodyBytes - len(envelope)
	// A diff that fills the request body exactly to the size limit.
	exactBody := `{"diff":"` + strings.Repeat("x", limitDiff) + `"}`
	// A diff that exceeds the request body size limit by one byte.
	overBody := `{"diff":"` + strings.Repeat("x", limitDiff+1) + `"}`
	// A complete JSON object padded with whitespace past the size limit.
	paddedBody := `{"diff":"x"}` + strings.Repeat(" ", srv.MaxBodyBytes)
	// Define the test cases.
	cases := []struct {
		name        string // The scenario name.
		body        string // The raw JSON request body.
		want        string // The expected decoded diff.
		wantStatus  int    // The expected HTTP status code on failure.
		wantErr     bool   // Whether decoding must fail.
		wantErrText string // Substring the returned error must contain.
	}{
		// A well-formed payload decodes to its diff.
		{"valid diff", `{"diff":"diff --git a/x b/x\n"}`, "diff --git a/x b/x\n", 0, false, ""},
		// Whitespace around the JSON object is ignored.
		{"valid diff with padding", "\n\t " + `{"diff":"x"}` + " \n", "x", 0, false, ""},
		// Trailing whitespace after the object is not trailing data.
		{"trailing whitespace", `{"diff":"x"}  ` + "\n", "x", 0, false, ""},
		// An empty body is not a JSON object.
		{"empty body", "", "", http.StatusBadRequest, true, "invalid JSON"},
		// A truncated payload is malformed JSON.
		{"malformed json", `{"diff":"x"`, "", http.StatusBadRequest, true, "invalid JSON"},
		// A non-object payload is malformed for the request shape.
		{"non-object json", `["diff"]`, "", http.StatusBadRequest, true, "invalid JSON"},
		// A non-string diff fails to decode into the request struct.
		{"non-string diff", `{"diff":42}`, "", http.StatusBadRequest, true, "invalid JSON"},
		// Unknown fields are rejected so misspelled keys fail fast.
		{"unknown field", `{"diff":"x","extra":1}`, "", http.StatusBadRequest, true, "invalid JSON"},
		// A second JSON object after the first is trailing data.
		{"trailing data", `{"diff":"x"}{"diff":"y"}`, "", http.StatusBadRequest, true, "unexpected trailing data"},
		// A missing diff field decodes to the empty string and is rejected.
		{"missing diff", `{}`, "", http.StatusBadRequest, true, "diff cannot be empty"},
		// An explicitly empty diff is rejected before any generation.
		{"empty diff", `{"diff":""}`, "", http.StatusBadRequest, true, "diff cannot be empty"},
		// A whitespace-only diff is rejected as empty.
		{"whitespace diff", `{"diff":"  \n\t "}`, "", http.StatusBadRequest, true, "diff cannot be empty"},
		// A body exactly at the size limit is accepted.
		{"exact size limit", exactBody, strings.Repeat("x", limitDiff), 0, false, ""},
		// A body one byte over the size limit is rejected as too large.
		{"one byte over limit", overBody, "", http.StatusRequestEntityTooLarge, true, "request body"},
		// Whitespace padding past the limit trips the bound on the trailing read.
		{"object padded past limit", paddedBody, "", http.StatusRequestEntityTooLarge, true, "request body"},
	}
	// Run the tests.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Decode the diff from the canned request body.
			got, status, err := decodeRequest(tc.body)
			// If the error state does not match, the test fails.
			if (err != nil) != tc.wantErr {
				// Report the unexpected error presence.
				if err != nil {
					// Fail because no error was expected.
					t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "decodeDiff", Err: err}))
				} else {
					// Fail because an error was expected.
					t.Error(tserr.NilFailed("decodeDiff"))
				}
			}
			// If the status code differs from the expectation, the test fails.
			if status != tc.wantStatus {
				// Report the unexpected HTTP status code.
				t.Errorf("decodeDiff status = %v, want %v", status, tc.wantStatus)
			}
			// If the diff differs from the expectation, the test fails.
			if got != tc.want {
				// Report the decoded diff mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "decodeDiff", Actual: got, Want: tc.want}))
			}
			// If the diagnostic text is missing from the error, the test fails.
			if tc.wantErrText != "" && err != nil && !strings.Contains(err.Error(), tc.wantErrText) {
				// Report the missing error diagnostic text.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "decodeDiff error text", Actual: err.Error(), Want: fmt.Sprintf("text containing %q", tc.wantErrText)}))
			}
		})
	}
}

// TestNewCommitServer tests NewCommitServer: token validation before
// dependency construction, generator construction across API key
// configurations, and endpoint registration on the constructed server. It uses
// t.Setenv and must not call t.Parallel.
func TestNewCommitServer(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name        string // The scenario name.
		tok         string // The bearer token passed to NewCommitServer.
		key         string // The OPENROUTER_API_KEY value installed for the test.
		wantErr     bool   // Whether construction must fail.
		wantErrText string // Substring the returned error must contain.
	}{
		{
			name: "valid token and API key build server",
			tok:  "swordfish",
			key:  "sk-or-v1-test-key",
		},
		{
			name:        "empty token is rejected",
			tok:         "",
			key:         "sk-or-v1-test-key",
			wantErr:     true,
			wantErrText: "token cannot be empty",
		},
		{
			name:        "whitespace token is rejected",
			tok:         "  \t\n",
			key:         "sk-or-v1-test-key",
			wantErr:     true,
			wantErrText: "token cannot be empty",
		},
		{
			name:        "missing API key fails generator construction",
			tok:         "swordfish",
			key:         "",
			wantErr:     true,
			wantErrText: "new client",
		},
		{
			name:        "whitespace API key fails generator construction",
			tok:         "swordfish",
			key:         "   \t\n",
			wantErr:     true,
			wantErrText: "new client",
		},
		{
			name:        "control character in API key fails generator construction",
			tok:         "swordfish",
			key:         "bad\nkey",
			wantErr:     true,
			wantErrText: "new client",
		},
	}
	// Run each scenario against NewCommitServer.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario API key; t.Setenv restores it after the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Construct the server under test.
			s, err := srv.NewCommitServer(tc.tok)
			// Check the error expectation.
			if tc.wantErr {
				// Fail if an expected error is missing.
				if err == nil {
					// Report the missing construction error.
					t.Error(tserr.NilFailed("NewCommitServer"))
				} else if tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
					// Fail if the diagnostic substring is missing.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "NewCommitServer error text", Actual: err.Error(), Want: fmt.Sprintf("text containing %q", tc.wantErrText)}))
				}
				// Ensure no non-nil server leaked alongside an error.
				if s != nil {
					// Report the leaked server instance on error.
					t.Errorf("NewCommitServer = %v, want nil on error", s)
				}
				// Nothing further to assert on expected construction errors.
				return
			}
			// Fail on any unexpected error.
			if err != nil {
				// Report the unexpected construction error.
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewCommitServer", Err: err}))
				// Stop inspecting on unexpected construction failure.
				return
			}
			// Fail if the returned server is nil on success.
			if s == nil {
				// Report the unexpectedly nil server instance.
				t.Errorf("NewCommitServer = nil, want non-nil")
				// Stop inspecting a nil server.
				return
			}
			// Verify /commit/healthz responds without authentication.
			healthReq := httptest.NewRequest(http.MethodGet, "/commit/healthz", nil)
			// Build a response recorder for the health probe.
			healthRec := httptest.NewRecorder()
			// Serve the health probe through the constructed server.
			s.ServeHTTP(healthRec, healthReq)
			// Fail if the health probe does not succeed.
			if healthRec.Code != http.StatusOK {
				// Report the unexpected health probe status code.
				t.Errorf("GET /commit/healthz status = %v, want %v", healthRec.Code, http.StatusOK)
			}
			// Verify /commit/generate rejects unauthenticated requests via requireAuth.
			unauthReq := httptest.NewRequest(http.MethodPost, "/commit/generate", strings.NewReader(`{"diff":"x"}`))
			// Build a response recorder for the unauthenticated probe.
			unauthRec := httptest.NewRecorder()
			// Serve the unauthenticated probe through the constructed server.
			s.ServeHTTP(unauthRec, unauthReq)
			// Fail if requireAuth did not reject the unauthenticated request.
			if unauthRec.Code != http.StatusUnauthorized {
				// Report that /commit/generate was not guarded by requireAuth.
				t.Errorf("unauthenticated POST /commit/generate status = %v, want %v", unauthRec.Code, http.StatusUnauthorized)
			}
			// Build a scripted generation seam to verify authenticated end-to-end routing.
			fake := &fakeGenerate{msg: tscommit.Message{Text: "feat(srv): add endpoint", Confidence: 0.9}}
			// Swap the generation seam for the duration of the subtest.
			useFakeGenerate(t, fake)
			// Build an authenticated POST request for /commit/generate.
			authReq := httptest.NewRequest(http.MethodPost, "/commit/generate", strings.NewReader(`{"diff":"diff --git a/x b/x"}`))
			// Attach the configured bearer token.
			authReq.Header.Set("Authorization", "Bearer "+tc.tok)
			// Build a response recorder for the authenticated probe.
			authRec := httptest.NewRecorder()
			// Serve the authenticated probe through the constructed server.
			s.ServeHTTP(authRec, authReq)
			// Fail if requireAuth did not forward to retrieveHandler.
			if authRec.Code != http.StatusOK {
				// Report the unexpected authenticated status code.
				t.Errorf("authenticated POST /commit/generate status = %v, want %v", authRec.Code, http.StatusOK)
			}
		})
	}
}

// failingWriter is an http.ResponseWriter whose writes always fail, used to
// exercise the health response write-failure path. The zero value records no
// status code, allocates headers lazily, and fails every write.
type failingWriter struct {
	header http.Header // The response headers recorded before the failing write.
	status int         // The status code passed to WriteHeader.
}

// Header returns the response headers of f for inspection.
func (f *failingWriter) Header() http.Header {
	// Lazily allocate the header map on first use.
	if f.header == nil {
		// Allocate the header map.
		f.header = make(http.Header)
	}
	// Return the header map.
	return f.header
}

// WriteHeader records the status code of f for later assertion.
func (f *failingWriter) WriteHeader(status int) {
	// Record the status code.
	f.status = status
}

// Write always fails to simulate a broken client connection.
func (f *failingWriter) Write([]byte) (int, error) {
	// Return no bytes written and an error to indicate the write failed.
	return 0, errors.New("write failed")
}

// newHealthServer constructs a CommitServer with a valid API key installed via
// t.Setenv so the health probe handler can be exercised through ServeHTTP.
func newHealthServer(t *testing.T) *srv.CommitServer {
	// Mark the test function as test helper function.
	t.Helper()
	// Install a valid API key; t.Setenv restores it after the test.
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test-key")
	// Construct the server under test.
	s, err := srv.NewCommitServer("swordfish")
	// Stop the test if the server cannot be constructed.
	if err != nil {
		// The health handler cannot be reached without a server.
		t.Fatal(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewCommitServer", Err: err}))
	}
	// Return the constructed server.
	return s
}

// TestHealthHandler tests healthHandler through ServeHTTP: GET and HEAD
// success, the 405 rejection with its Allow header, and response shaping.
func TestHealthHandler(t *testing.T) {
	// The plain-text content type every health response carries.
	const wantCT = "text/plain; charset=utf-8"
	// Define the test cases.
	cases := []struct {
		name        string // The scenario name.
		method      string // The HTTP method of the probe request.
		wantStatus  int    // The expected HTTP status code.
		wantAllow   string // The expected Allow header value, empty when unset.
		wantBody    string // The expected exact response body.
		wantBodyTxt string // A substring the response body must contain.
	}{
		{"get probe", http.MethodGet, http.StatusOK, "", "ok\n", ""},
		{"head probe", http.MethodHead, http.StatusOK, "", "", ""},
		{"post rejected", http.MethodPost, http.StatusMethodNotAllowed, "GET, HEAD", "", "method POST is not allowed for health"},
		{"put rejected", http.MethodPut, http.StatusMethodNotAllowed, "GET, HEAD", "", "method PUT is not allowed for health"},
		{"delete rejected", http.MethodDelete, http.StatusMethodNotAllowed, "GET, HEAD", "", "method DELETE is not allowed for health"},
	}
	// Run each scenario against healthHandler.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Construct the server under test.
			s := newHealthServer(t)
			// Build a probe request with the scenario method.
			r := httptest.NewRequest(tc.method, "/commit/healthz", nil)
			// Build a response recorder for the probe.
			w := httptest.NewRecorder()
			// Serve the probe through the constructed server.
			s.ServeHTTP(w, r)
			// If the status code differs from the expectation, the test fails.
			if w.Code != tc.wantStatus {
				// Report the unexpected HTTP status code.
				t.Errorf("healthHandler status = %v, want %v", w.Code, tc.wantStatus)
			}
			// If the Allow header differs from the expectation, the test fails.
			if got := w.Header().Get("Allow"); got != tc.wantAllow {
				// Report the Allow header mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "healthHandler Allow", Actual: got, Want: tc.wantAllow}))
			}
			// If the Content-Type header differs from the expectation, the test fails.
			if got := w.Header().Get("Content-Type"); got != wantCT {
				// Report the Content-Type header mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "healthHandler Content-Type", Actual: got, Want: wantCT}))
			}
			// If the body differs from the expectation, the test fails. Cases
			// asserting a diagnostic substring skip the exact match because the
			// tserr envelope wraps the message.
			if tc.wantBodyTxt == "" {
				// Compare the exact response body.
				if got := w.Body.String(); got != tc.wantBody {
					// Report the exact response body mismatch.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "healthHandler body", Actual: got, Want: tc.wantBody}))
				}
			}
			// If the diagnostic text is missing from the body, the test fails.
			if tc.wantBodyTxt != "" && !strings.Contains(w.Body.String(), tc.wantBodyTxt) {
				// Report the missing diagnostic text in the response body.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "healthHandler body text", Actual: w.Body.String(), Want: fmt.Sprintf("text containing %q", tc.wantBodyTxt)}))
			}
		})
	}
}

// TestHealthHandlerWriteFailure tests the health response write-failure path
// with a response writer whose writes always fail.
func TestHealthHandlerWriteFailure(t *testing.T) {
	// The plain-text content type every health response carries.
	const wantCT = "text/plain; charset=utf-8"
	// Construct the server under test.
	s := newHealthServer(t)
	// Build a GET probe request.
	r := httptest.NewRequest(http.MethodGet, "/commit/healthz", nil)
	// Build a response writer whose writes always fail.
	w := &failingWriter{}
	// Serve the probe; the write failure must be logged, not panic.
	s.ServeHTTP(w, r)
	// If the status code was not written before the failing write, the test fails.
	if w.status != http.StatusOK {
		// Report the unexpected status code recorded before the failing write.
		t.Errorf("healthHandler status = %v, want %v", w.status, http.StatusOK)
	}
	// The Content-Type header must be set before the failing write.
	if got := w.Header().Get("Content-Type"); got != wantCT {
		// Report the Content-Type header mismatch.
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "healthHandler Content-Type", Actual: got, Want: wantCT}))
	}
}

// TestRequireAuth tests requireAuth: uninitialized-token rejection,
// Authorization header validation, and forwarding to the wrapped handler.
func TestRequireAuth(t *testing.T) {
	// The rejection body every 401 response carries.
	const wantRejectBody = "Unauthorized\n"
	// Define the test cases.
	cases := []struct {
		name       string // The scenario name.
		tok        string // The bearer token carried by the server.
		authHeader string // The Authorization header of the request, empty when absent.
		wantStatus int    // The expected HTTP status code.
		wantNext   bool   // Whether the wrapped handler must be invoked.
	}{
		{"empty token rejects", "", "Bearer swordfish", http.StatusUnauthorized, false},
		{"whitespace token rejects", "  \t", "Bearer swordfish", http.StatusUnauthorized, false},
		{"missing header rejects", "swordfish", "", http.StatusUnauthorized, false},
		{"wrong scheme rejects", "swordfish", "Basic swordfish", http.StatusUnauthorized, false},
		{"wrong token rejects", "swordfish", "Bearer wrong", http.StatusUnauthorized, false},
		{"lowercase scheme rejects", "swordfish", "bearer swordfish", http.StatusUnauthorized, false},
		{"bare Bearer rejects", "swordfish", "Bearer", http.StatusUnauthorized, false},
		{"extra space rejects", "swordfish", "Bearer  swordfish", http.StatusUnauthorized, false},
		{"trailing space rejects", "swordfish", "Bearer swordfish ", http.StatusUnauthorized, false},
		{"valid token forwards", "swordfish", "Bearer swordfish", http.StatusOK, true},
	}
	// Run each scenario against requireAuth.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Track whether the wrapped handler was invoked.
			called := false
			// Build the wrapped handler that records its invocation.
			next := func(w http.ResponseWriter, r *http.Request) {
				// Record the invocation.
				called = true
				// Write the success status.
				w.WriteHeader(http.StatusOK)
			}
			// Build the auth-wrapped handler under test.
			h := srv.NewRequireAuth(tc.tok, next)
			// Build a request for the protected endpoint.
			r := httptest.NewRequest(http.MethodPost, "/commit/generate", nil)
			// Set the Authorization header when the scenario provides one.
			if tc.authHeader != "" {
				// Set the scenario Authorization header.
				r.Header.Set("Authorization", tc.authHeader)
			}
			// Build a response recorder for the request.
			w := httptest.NewRecorder()
			// Serve the request through the auth-wrapped handler.
			h(w, r)
			// If the status code differs from the expectation, the test fails.
			if w.Code != tc.wantStatus {
				// Report the unexpected HTTP status code.
				t.Errorf("requireAuth status = %v, want %v", w.Code, tc.wantStatus)
			}
			// If the forwarding expectation is wrong, the test fails.
			if called != tc.wantNext {
				// Report whether the wrapped handler was unexpectedly called or skipped.
				t.Errorf("requireAuth next called = %v, want %v", called, tc.wantNext)
			}
			// Only the rejection responses carry the assertions below.
			if tc.wantStatus != http.StatusUnauthorized {
				// Nothing further to assert on the forwarding path.
				return
			}
			// The rejection must advertise the Bearer scheme per RFC 6750.
			if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
				// Report the WWW-Authenticate header mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "requireAuth WWW-Authenticate", Actual: got, Want: "Bearer"}))
			}
			// If the rejection body differs from the expectation, the test fails.
			if got := w.Body.String(); got != wantRejectBody {
				// Report the rejection body mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "requireAuth body", Actual: got, Want: wantRejectBody}))
			}
		})
	}
}

// fakeGenerate scripts the generation seam without a provider. The zero value
// returns an empty message and no error.
type fakeGenerate struct {
	msg   tscommit.Message // The canned message returned by the seam.
	err   error            // The canned error returned by the seam.
	calls int              // The number of generation seam invocations.
	diffs []string         // The diffs received by the generation seam.
}

// run records the diff and replays the canned message and error.
func (f *fakeGenerate) run(_ context.Context, _ *tscommit.Generator, diff string) (tscommit.Message, error) {
	// Record the generation seam invocation.
	f.calls++
	// Record the received diff.
	f.diffs = append(f.diffs, diff)
	// Return the canned message and error.
	return f.msg, f.err
}

// useFakeGenerate swaps the generation seam for the duration of the test.
// Tests using it must not call t.Parallel.
func useFakeGenerate(t *testing.T, f *fakeGenerate) {
	// Mark the test function as test helper function.
	t.Helper()
	// Swap the generation seam and restore it when the test ends.
	t.Cleanup(srv.SetGenerateMessage(f.run))
}

// newTestGenerator returns a Generator built from a valid API key installed
// via t.Setenv, so tests reach the generation seam without a provider.
func newTestGenerator(t *testing.T) *tscommit.Generator {
	// Mark the test function as test helper function.
	t.Helper()
	// Install a valid API key; t.Setenv restores it after the test.
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test-key")
	// Build the generator under test.
	gen, err := tscommit.NewGenerator()
	// Stop the test if the generator cannot be built.
	if err != nil {
		// The retrieval handler cannot be exercised without a generator.
		t.Fatal(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "NewGenerator", Err: err}))
	}
	// Return the constructed generator.
	return gen
}

// TestRetrieveHandler tests retrieveHandler: the uninitialized-generator
// guard, method enforcement, request validation, generation failure, response
// encoding failure, and the success path. It swaps the generation seam and
// must not call t.Parallel.
func TestRetrieveHandler(t *testing.T) {
	// The JSON content type of successful generation responses.
	const wantOKCT = "application/json"
	// The plain-text content type of http.Error rejection responses.
	const wantErrCT = "text/plain; charset=utf-8"
	// Install a valid API key and build the generator shared by the cases.
	gen := newTestGenerator(t)
	// A request body exceeding the size limit to exercise status propagation.
	oversizedBody := `{"diff":"` + strings.Repeat("x", srv.MaxBodyBytes) + `"}`
	// Define the test cases.
	cases := []struct {
		name        string           // The scenario name.
		noGen       bool             // Whether the server carries no generator.
		method      string           // The HTTP method of the request.
		body        string           // The raw JSON request body.
		genMsg      tscommit.Message // The message the generation seam returns.
		genErr      error            // The error the generation seam returns.
		wantStatus  int              // The expected HTTP status code.
		wantAllow   string           // The expected Allow header value, empty when unset.
		wantCT      string           // The expected Content-Type header value.
		wantBody    string           // The expected exact response body.
		wantBodyTxt string           // A substring the response body must contain.
		wantCalls   int              // The expected number of generation seam invocations.
		wantDiff    string           // The diff the generation seam must receive.
	}{
		{
			name:        "nil generator rejects",
			noGen:       true,
			method:      http.MethodPost,
			body:        `{"diff":"diff --git a/x b/x"}`,
			wantStatus:  http.StatusInternalServerError,
			wantCT:      wantErrCT,
			wantBodyTxt: "Generator cannot be empty",
			wantCalls:   0,
		},
		{
			name:        "get method rejected",
			method:      http.MethodGet,
			wantStatus:  http.StatusMethodNotAllowed,
			wantAllow:   http.MethodPost,
			wantCT:      wantErrCT,
			wantBodyTxt: "method GET is not allowed for CommitServer",
			wantCalls:   0,
		},
		{
			name:        "put method rejected",
			method:      http.MethodPut,
			wantStatus:  http.StatusMethodNotAllowed,
			wantAllow:   http.MethodPost,
			wantCT:      wantErrCT,
			wantBodyTxt: "method PUT is not allowed for CommitServer",
			wantCalls:   0,
		},
		{
			name:        "empty diff rejected",
			method:      http.MethodPost,
			body:        `{"diff":""}`,
			wantStatus:  http.StatusBadRequest,
			wantCT:      wantErrCT,
			wantBodyTxt: "diff cannot be empty",
			wantCalls:   0,
		},
		{
			name:        "oversized body rejected",
			method:      http.MethodPost,
			body:        oversizedBody,
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantCT:      wantErrCT,
			wantBodyTxt: "expected to be at most",
			wantCalls:   0,
		},
		{
			name:        "generation failure rejects",
			method:      http.MethodPost,
			body:        `{"diff":"diff --git a/x b/x"}`,
			genErr:      errors.New("provider exploded"),
			wantStatus:  http.StatusInternalServerError,
			wantCT:      wantErrCT,
			wantBodyTxt: "provider exploded",
			wantCalls:   1,
			wantDiff:    "diff --git a/x b/x",
		},
		{
			name:        "encoding failure rejects",
			method:      http.MethodPost,
			body:        `{"diff":"diff --git a/x b/x"}`,
			genMsg:      tscommit.Message{Text: "feat: x", Confidence: math.NaN()},
			wantStatus:  http.StatusInternalServerError,
			wantCT:      wantErrCT,
			wantBodyTxt: "json.Marshal",
			wantCalls:   1,
			wantDiff:    "diff --git a/x b/x",
		},
		{
			name:       "valid diff generates message",
			method:     http.MethodPost,
			body:       `{"diff":"diff --git a/x b/x"}`,
			genMsg:     tscommit.Message{Text: "feat: x", Confidence: 0.9, Model: "google/gemini-4-hydrogen", TotalTokens: 320, Cost: 0.0025},
			wantStatus: http.StatusOK,
			wantCT:     wantOKCT,
			wantBody:   `{"text":"feat: x","confidence":0.9,"model":"google/gemini-4-hydrogen","total_tokens":320,"cost":0.0025}`,
			wantCalls:  1,
			wantDiff:   "diff --git a/x b/x",
		},
	}
	// Run each scenario against retrieveHandler.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Use the shared generator unless the case exercises the guard branch.
			caseGen := gen
			// Leave the generator nil for the uninitialized-generator case.
			if tc.noGen {
				// Use no generator to trigger the guard.
				caseGen = nil
			}
			// Build the retrieval handler under test.
			h := srv.NewRetrieveHandler(caseGen)
			// Build the scripted generation seam.
			fake := &fakeGenerate{msg: tc.genMsg, err: tc.genErr}
			// Swap the generation seam for the duration of the test.
			useFakeGenerate(t, fake)
			// Build a request with the scenario method and body.
			r := httptest.NewRequest(tc.method, "/commit/generate", strings.NewReader(tc.body))
			// Build a response recorder for the request.
			w := httptest.NewRecorder()
			// Serve the request through the retrieval handler.
			h(w, r)
			// If the status code differs from the expectation, the test fails.
			if w.Code != tc.wantStatus {
				// Report the unexpected HTTP status code.
				t.Errorf("retrieveHandler status = %v, want %v", w.Code, tc.wantStatus)
			}
			// If the Allow header differs from the expectation, the test fails.
			if got := w.Header().Get("Allow"); got != tc.wantAllow {
				// Report the Allow header mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "retrieveHandler Allow", Actual: got, Want: tc.wantAllow}))
			}
			// If the Content-Type header differs from the expectation, the test fails.
			if got := w.Header().Get("Content-Type"); got != tc.wantCT {
				// Report the Content-Type header mismatch.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "retrieveHandler Content-Type", Actual: got, Want: tc.wantCT}))
			}
			// Exact body match only when no diagnostic substring is expected.
			if tc.wantBodyTxt == "" {
				// If the body differs from the expectation, the test fails.
				if got := w.Body.String(); got != tc.wantBody {
					// Report the exact response body mismatch.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "retrieveHandler body", Actual: got, Want: tc.wantBody}))
				}
			}
			// If the diagnostic text is missing from the body, the test fails.
			if tc.wantBodyTxt != "" && !strings.Contains(w.Body.String(), tc.wantBodyTxt) {
				// Report the missing diagnostic text in the response body.
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "retrieveHandler body text", Actual: w.Body.String(), Want: fmt.Sprintf("text containing %q", tc.wantBodyTxt)}))
			}
			// If the generation seam call count differs from the expectation, the test fails.
			if fake.calls != tc.wantCalls {
				// Report the generation seam invocation count mismatch.
				t.Errorf("retrieveHandler generateMessage calls = %v, want %v", fake.calls, tc.wantCalls)
			}
			// If the generation seam received a different diff, the test fails.
			if tc.wantDiff != "" && (len(fake.diffs) != 1 || fake.diffs[0] != tc.wantDiff) {
				t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "retrieveHandler generateMessage diff", Actual: fmt.Sprintf("%v", fake.diffs), Want: tc.wantDiff}))
			}
		})
	}
}

// TestRetrieveHandlerWriteFailure tests the response write-failure path of
// retrieveHandler with a response writer whose writes always fail. It swaps
// the generation seam and must not call t.Parallel.
func TestRetrieveHandlerWriteFailure(t *testing.T) {
	// The JSON content type of successful generation responses.
	const wantCT = "application/json"
	// Install a valid API key and build the generator under test.
	gen := newTestGenerator(t)
	// Build the retrieval handler under test.
	h := srv.NewRetrieveHandler(gen)
	// Build the scripted generation seam returning a well-formed message.
	fake := &fakeGenerate{msg: tscommit.Message{Text: "feat: x", Confidence: 0.9}}
	// Swap the generation seam for the duration of the test.
	useFakeGenerate(t, fake)
	// Build a POST request carrying a valid diff.
	r := httptest.NewRequest(http.MethodPost, "/commit/generate", strings.NewReader(`{"diff":"diff --git a/x b/x"}`))
	// Build a response writer whose writes always fail.
	w := &failingWriter{}
	// Serve the request; the write failure must be logged, not panic.
	h(w, r)
	// The Content-Type header must be set before the failing write.
	if got := w.Header().Get("Content-Type"); got != wantCT {
		// Report the Content-Type header mismatch.
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "retrieveHandler Content-Type", Actual: got, Want: wantCT}))
	}
	// The generation seam must have been invoked exactly once.
	if fake.calls != 1 {
		// Report the generation seam invocation count mismatch.
		t.Errorf("retrieveHandler generateMessage calls = %v, want 1", fake.calls)
	}
}

// fakeLifecycle scripts the HTTP listener and graceful shutdown seams without
// opening a network socket. The zero value blocks listen until closed and
// succeeds on shutdown.
type fakeLifecycle struct {
	listenErr       error              // The error returned by listenAndServe; nil maps to http.ErrServerClosed on shutdown.
	shutdownErr     error              // The error returned by shutdownServer.
	cancelOnListen  context.CancelFunc // Optional cancel function invoked once listenAndServe starts.
	closed          chan struct{}      // Closed when shutdownServer runs so a blocking listenAndServe returns.
	listenCalls     int                // The number of listenAndServe invocations.
	shutdownCalls   int                // The number of shutdownServer invocations.
	addr            string             // The configured Addr observed on the http.Server.
	hasHandler      bool               // Whether the http.Server carried a non-nil Handler.
	hasTimeouts     bool               // Whether the http.Server carried positive timeouts on every socket bound.
	hasShutdownLine bool               // Whether the shutdown context carried a deadline.
}

// newFakeLifecycle returns an initialized fakeLifecycle ready for seam injection.
func newFakeLifecycle(listenErr, shutdownErr error, cancelOnListen context.CancelFunc) *fakeLifecycle {
	// Return a fakeLifecycle with an initialized close channel.
	return &fakeLifecycle{
		listenErr:      listenErr,
		shutdownErr:    shutdownErr,
		cancelOnListen: cancelOnListen,
		closed:         make(chan struct{}),
	}
}

// listen records the server configuration, optionally cancels the run context,
// and either returns listenErr immediately or waits for shutdown.
func (f *fakeLifecycle) listen(s *http.Server) error {
	// Record the listener invocation.
	f.listenCalls++
	// Record the bound server address.
	f.addr = s.Addr
	// Record whether a request handler was wired onto the server.
	f.hasHandler = s.Handler != nil
	// Record whether all four HTTP socket timeouts were configured.
	f.hasTimeouts = s.ReadHeaderTimeout > 0 && s.ReadTimeout > 0 && s.WriteTimeout > 0 && s.IdleTimeout > 0
	// Return the scripted listener failure immediately when set.
	if f.listenErr != nil {
		// Replay the scripted listener error.
		return f.listenErr
	}
	// Trigger context cancellation once the listener is active when requested.
	if f.cancelOnListen != nil {
		// Cancel the caller's run context to initiate graceful shutdown.
		f.cancelOnListen()
	}
	// Block until shutdownServer closes the fake listener.
	<-f.closed
	// Return http.ErrServerClosed to match net/http Shutdown semantics.
	return http.ErrServerClosed
}

// shutdown records the shutdown context deadline, unblocks listen, and returns
// the scripted shutdown error.
func (f *fakeLifecycle) shutdown(ctx context.Context, _ *http.Server) error {
	// Record the shutdown invocation.
	f.shutdownCalls++
	// Record whether RunContext attached a bounded deadline to the shutdown context.
	_, f.hasShutdownLine = ctx.Deadline()
	// Unblock the waiting fake listener on the first shutdown call.
	if f.shutdownCalls == 1 {
		// Close the channel so listen returns http.ErrServerClosed.
		close(f.closed)
	}
	// Return the scripted shutdown outcome.
	return f.shutdownErr
}

// useFakeLifecycle swaps the listener and shutdown seams for the duration of
// the test. Tests using it must not call t.Parallel.
func useFakeLifecycle(t *testing.T, f *fakeLifecycle) {
	// Mark the test function as test helper function.
	t.Helper()
	// Swap the listener seam and restore it at test cleanup.
	t.Cleanup(srv.SetListenAndServe(f.listen))
	// Swap the shutdown seam and restore it at test cleanup.
	t.Cleanup(srv.SetShutdownServer(f.shutdown))
}

// TestRunContext tests RunContext across nil-context validation, PORT fallback
// and custom port binding, API_TOKEN and OPENROUTER_API_KEY validation,
// listener startup errors, graceful shutdown on context cancellation, and
// shutdown failures. It mutates environment variables and package seams and
// must not call t.Parallel.
func TestRunContext(t *testing.T) {
	// Define the test cases.
	cases := []struct {
		name         string // The scenario name.
		nilCtx       bool   // Whether to pass a nil context to RunContext.
		port         string // The PORT environment variable installed for the subtest.
		token        string // The API_TOKEN environment variable installed for the subtest.
		key          string // The OPENROUTER_API_KEY environment variable installed for the subtest.
		listenErr    error  // The scripted error returned by listenAndServe.
		shutdownErr  error  // The scripted error returned by shutdownServer.
		wantErr      bool   // Whether RunContext must return an error.
		wantErrText  string // Substring the returned error must contain.
		wantAddr     string // The expected http.Server Addr when listenAndServe runs.
		wantListen   int    // The expected number of listenAndServe calls.
		wantShutdown int    // The expected number of shutdownServer calls.
	}{
		{
			name:        "nil context fails before reading config",
			nilCtx:      true,
			port:        "9090",
			token:       "swordfish",
			key:         "sk-or-v1-test-key",
			wantErr:     true,
			wantErrText: "context cannot be empty",
		},
		{
			name:        "missing API_TOKEN fails before starting listener",
			port:        "9090",
			token:       "",
			key:         "sk-or-v1-test-key",
			wantErr:     true,
			wantErrText: "API_TOKEN",
		},
		{
			name:        "whitespace API_TOKEN fails before starting listener",
			port:        "9090",
			token:       "   \t\n",
			key:         "sk-or-v1-test-key",
			wantErr:     true,
			wantErrText: "API_TOKEN",
		},
		{
			name:        "missing OPENROUTER_API_KEY fails server construction",
			port:        "9090",
			token:       "swordfish",
			key:         "",
			wantErr:     true,
			wantErrText: "new commit server",
		},
		{
			name:         "unset PORT defaults to 8080 and shuts down cleanly",
			port:         "",
			token:        "swordfish",
			key:          "sk-or-v1-test-key",
			wantAddr:     ":8080",
			wantListen:   1,
			wantShutdown: 1,
		},
		{
			name:         "whitespace PORT defaults to 8080 and shuts down cleanly",
			port:         "  \t ",
			token:        "swordfish",
			key:          "sk-or-v1-test-key",
			wantAddr:     ":8080",
			wantListen:   1,
			wantShutdown: 1,
		},
		{
			name:         "custom PORT is trimmed, bound, and shuts down cleanly",
			port:         " 9090 ",
			token:        "swordfish",
			key:          "sk-or-v1-test-key",
			wantAddr:     ":9090",
			wantListen:   1,
			wantShutdown: 1,
		},
		{
			name:        "listener failure returns error without running shutdown",
			port:        "9090",
			token:       "swordfish",
			key:         "sk-or-v1-test-key",
			listenErr:   errors.New("address already in use"),
			wantErr:     true,
			wantErrText: "address already in use",
			wantAddr:    ":9090",
			wantListen:  1,
		},
		{
			name:         "shutdown failure returns wrapped error",
			port:         "9090",
			token:        "swordfish",
			key:          "sk-or-v1-test-key",
			shutdownErr:  context.DeadlineExceeded,
			wantErr:      true,
			wantErrText:  "Shutdown",
			wantAddr:     ":9090",
			wantListen:   1,
			wantShutdown: 1,
		},
	}
	// Run each scenario against RunContext.
	for _, tc := range cases {
		// Run the test.
		t.Run(tc.name, func(t *testing.T) {
			// Install the scenario PORT value for the subtest.
			t.Setenv("PORT", tc.port)
			// Install the scenario API_TOKEN value for the subtest.
			t.Setenv("API_TOKEN", tc.token)
			// Install the scenario OPENROUTER_API_KEY value for the subtest.
			t.Setenv("OPENROUTER_API_KEY", tc.key)
			// Build a cancellable context so fakeLifecycle can trigger graceful shutdown.
			runCtx, cancel := context.WithCancel(context.Background())
			// Ensure the context resources are released at subtest cleanup.
			defer cancel()
			// Install the scripted HTTP lifecycle seams.
			fake := newFakeLifecycle(tc.listenErr, tc.shutdownErr, cancel)
			// Swap the listener and shutdown seams for the duration of the subtest.
			useFakeLifecycle(t, fake)
			// Select the context passed to RunContext.
			var ctx context.Context = runCtx
			// Replace ctx with nil when testing the nil-context guard.
			if tc.nilCtx {
				// Use a nil context to exercise the guard branch.
				ctx = nil
			}
			// Execute RunContext with the scenario configuration.
			err := srv.RunContext(ctx)
			// Check the error expectation.
			if tc.wantErr {
				// Fail if an expected error is missing.
				if err == nil {
					// Report the missing RunContext error.
					t.Error(tserr.NilFailed("RunContext"))
				} else if tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
					// Report the missing diagnostic substring in the error.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "RunContext error text", Actual: err.Error(), Want: fmt.Sprintf("text containing %q", tc.wantErrText)}))
				}
			} else if err != nil {
				// Report the unexpected RunContext error.
				t.Error(tserr.NilExpected(&tserr.NilExpectedArgs{Op: "RunContext", Err: err}))
			}
			// Check the listener seam call count.
			if fake.listenCalls != tc.wantListen {
				// Report the listener invocation count mismatch.
				t.Errorf("RunContext listenAndServe calls = %v, want %v", fake.listenCalls, tc.wantListen)
			}
			// Check the shutdown seam call count.
			if fake.shutdownCalls != tc.wantShutdown {
				// Report the shutdown invocation count mismatch.
				t.Errorf("RunContext shutdownServer calls = %v, want %v", fake.shutdownCalls, tc.wantShutdown)
			}
			// Verify the constructed http.Server fields whenever listenAndServe ran.
			if tc.wantListen > 0 {
				// Check the bound server address.
				if fake.addr != tc.wantAddr {
					// Report the server address mismatch.
					t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "RunContext server Addr", Actual: fake.addr, Want: tc.wantAddr}))
				}
				// Verify a non-nil CommitServer handler was attached.
				if !fake.hasHandler {
					// Report the missing HTTP handler on the server.
					t.Errorf("RunContext server Handler = nil, want non-nil")
				}
				// Verify all four HTTP socket timeouts were configured.
				if !fake.hasTimeouts {
					// Report missing HTTP socket timeouts.
					t.Errorf("RunContext server timeouts incomplete, want positive ReadHeader, Read, Write, and Idle timeouts")
				}
			}
			// Verify graceful shutdown received a bounded deadline context.
			if tc.wantShutdown > 0 && !fake.hasShutdownLine {
				// Report the missing shutdown deadline.
				t.Errorf("RunContext shutdown context has no deadline, want shutdownTimeout deadline")
			}
		})
	}
}

// TestRun tests that Run delegates to RunContext with a background context. It
// mutates environment variables and package seams and must not call t.Parallel.
func TestRun(t *testing.T) {
	// Install a valid port, bearer token, and provider API key.
	t.Setenv("PORT", "8080")
	// Install the bearer token for server construction.
	t.Setenv("API_TOKEN", "swordfish")
	// Install the provider API key for generator construction.
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test-key")
	// Script a listener failure so Run returns deterministically without signal cancellation.
	fake := newFakeLifecycle(errors.New("bind failed"), nil, nil)
	// Swap the listener and shutdown seams for the duration of the test.
	useFakeLifecycle(t, fake)
	// Invoke Run and verify the listener error propagates back.
	err := srv.Run()
	// Fail if Run did not surface the listener error.
	if err == nil {
		// Report the missing error from Run.
		t.Error(tserr.NilFailed("Run"))
	} else if !strings.Contains(err.Error(), "bind failed") {
		// Report the missing listener diagnostic text.
		t.Error(tserr.EqualStr(&tserr.EqualStrArgs{Var: "Run error text", Actual: err.Error(), Want: `text containing "bind failed"`}))
	}
}
