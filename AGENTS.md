# Engineering & AI Collaboration Guidelines

This repository is developed using AI-assisted engineering with strict human oversight. A high bar for stability, extensive test coverage, and code hardening is maintained.

---

## 🎯 Architecture & Intent
- **Context:** Minimal, highly encapsulated Go package. The root package is a
  component-agnostic library; all binaries live under `cmd/`.
- **Evolution:** Packages in this organization typically end up as a Google
  Cloud Run microservice, sometimes accompanied by a CLI helper that calls
  the service over its API.
- **Frameworks:** Pure Go standard library and thorsphere packages (keep dependencies to an absolute minimum).
- **Configuration:** Strictly twelve-factor app principles via environment variables (`os.Getenv`).

## 🤖 AI Agent Instructions
When generating code, refactoring, or planning tasks, you MUST adhere to these technical constraints:

### 1. Components & Google Cloud Run Constraints
Every binary under `cmd/` is exactly one of two component types. Identify
which component you are touching before writing code. The constraints
differ, and rules for one MUST NOT be applied to the other. One `main` package per directory under `cmd/` rather than adding a second main to the same directory.

**a) Cloud Run service**
- **Port Binding:** The HTTP server MUST read and bind to the `$PORT` environment variable.
- **Health Checks:** The server MUST answer HTTP requests on `$PORT` (Cloud Run container health checks).
- **Statelessness:** No local file persistence. All state is per-request or
  delegated to managed GCP services.
- **Graceful Shutdown:** On SIGTERM (instance shutdown), stop accepting new
  requests and drain in-flight work before exiting.
- **Secrets:** Third-party API keys (e.g. `SERVICE_API_KEY`) are injected
  via environment variables and MUST remain server-side.

**b) CLI helper**
- **Interactive:** Runs on a developer machine with a TTY, opens `$EDITOR` for user-edited input, and shells out to local binaries when the workflow requires it (e.g. `git`).
- **No HTTP server:** NEVER bind a port or add an HTTP server to the CLI —
  outbound API calls only.
- **Statelessness:** No config files, caches, or local persistence;
  configuration comes exclusively from environment variables.
- **Service client:** Once the Cloud Run service exists, the CLI calls it
  over its HTTP API instead of talking to third-party APIs directly.

**c) Shared library (root package)**
- Must stay component-agnostic: no HTTP server wiring, no `main` imports.

### 2. Quality & Hardening Standards
- **Testing:** Every new feature or package function requires table-driven Unit Tests. Target >85% coverage.
- **Error Handling:** Explicit, idiomatic Go error handling. Do not panic. Wrap errors contextually where appropriate. Use the `tserr` package for error handling.
- **Code Formatting:** Use `gofmt` to format Go code.
- **Logging:** Service components: structured JSON logging to stdout (GCP Cloud Logging compatible) via tslog. CLI: human-readable output; tslog for diagnostics.

### 3. Coding Style (match the existing codebase exactly)
- **File header:** Every `.go` file starts with the thorsphere copyright
  notice (FSL-1.1-ALv2, see any existing file) followed by the package clause.
- **Imports:** Precede the import block with `// Import packages` and annotate
  every import inline, e.g. `"strings" // Import the strings package for string manipulation.`
- **Comments:** Doc comment on every function (exported and unexported).
  Narrate defensive steps inline: `// If the user cancels, return an error.`,
  `// Return no error to indicate success.`
- **Errors:** Check every returned error. Never panic. Wrap with the
  appropriate `tserr` constructor.
- **Tests:** External test package `<pkg>_test`. Bridge unexported
  functions via `export_test.go`. Unit tests must NOT depend on external
  binaries or remote services — tests must pass in any environment (CI,
  Cloud Build). Route every external invocation (process execution, HTTP
  calls, filesystem access) through a package-level seam (a function
  variable) and swap it in tests via a `Set<Name>` helper in
  `export_test.go`. Tests that mutate a global seam must not use
  `t.Parallel()`.

### 4. Commands
- Build: `go build ./...` — Test: `go test ./...` — Vet: `go vet ./...`
- Format check: `gofmt -l .` (must output nothing)
- Toolchain: latest Go toolchain (per `go.mod`)

### 5. Commit Messages
- Commits in this repository follow strict Conventional Commits:
  `<type>[scope][!]: <imperative description>` with a header ≤72 bytes,
  a body wrapped at 72 explaining WHAT and WHY, and a
  `BREAKING CHANGE:` footer when consumers must adapt.
- When a format or contract is defined in multiple coupled places
  (prompt template, validator, table-driven tests), a change MUST be
  applied to all of them in the same commit — never one side alone.

### 6. Deliberate Design Decisions — do NOT "fix" these
- User-edited commit messages are intentionally NOT re-validated; after the
  user edits, their text is authoritative. `validateMessage` shapes AI
  output only.
- Header length is measured in bytes (git/commitlint convention), not runes.
- Trailing newlines are ignored during validation (git convention).
- The commit format is intentionally specified in two places: the AI
  prompt (`prompt_ai.go`) and the validator (`validate.go`). Do not
  "deduplicate" them — the prompt is prose for the model, the validator
  is code. Format changes must update both, plus `validate_test.go`.

---

## 🛠️ Human Review & Hardening Process
*Every line of code generated by AI agents undergoes a mandatory human review, hardening loop, and optimization pass before being submitted to the repository. The review loop applies to the internal AI-assisted process; external contribution policy is governed by CONTRIBUTING.md*
