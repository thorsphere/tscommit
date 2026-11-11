# tscommit

[![PkgGoDev](https://pkg.go.dev/badge/mod/github.com/thorsphere/tscommit)](https://pkg.go.dev/mod/github.com/thorsphere/tscommit)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/thorsphere/tscommit)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/thorsphere/tscommit)
![GitHub Top Language](https://img.shields.io/github/languages/top/thorsphere/tscommit)
[![CodeFactor](https://www.codefactor.io/repository/github/thorsphere/tscommit/badge)](https://www.codefactor.io/repository/github/thorsphere/tscommit)
![OSS Lifecycle](https://img.shields.io/osslifecycle/thorsphere/tscommit)

---

Minimalist Go CLI tool for generating conventional git commit messages from staged diffs with AI.

---

Package `tscommit` generates conventional git commit messages from the
staged diff via a configured AI provider and creates the commit after
interactive user review. It is a component-agnostic library: the AI
provider API key is read from the environment, and no state is persisted
locally.

---

## Features

- **Conventional Commits output** — strict `<type>(<scope>)[!]: <description>`
  headers (72-character limit, imperative mood, mandatory scope) with a
  72-column body explaining WHAT and WHY, and optional trailers such as
  `Refs #<issue>` or `BREAKING CHANGE:`.
- **Generation from the staged diff** — reads `git diff --staged -W` and
  summarizes the dominant intent rather than listing files.
- **Self-correcting generation** — generated messages are validated against
  the format rules; invalid or truncated output is fed back to the model
  with corrective feedback and regenerated.
- **Structured AI envelopes & calibrated confidence** — decodes structured
  JSON responses (`findings` and `confidence`) via
  `tsaiclient.Client.CompleteJSON`, scores confidence against a four-tier
  diff-clarity rubric (`0%`–`100%`) displayed in the review prompt (omitted
  once edited), and strips any stray markdown fences or wrapper quotes from
  the message.
- **Interactive review loop** — accept, edit in `$EDITOR`, regenerate, or
  cancel before anything is committed. User-edited messages are
  authoritative and never re-validated.
- **Breaking-change awareness** — prominent warning when the message
  carries a Conventional Commits breaking-change marker.
- **Large-diff handling** — diffs are truncated at 30,000 characters
  (runes) with an explicit truncation marker, keeping prompts within the
  model's context window.
- **Prompt-cache friendly** — one sticky session ID per run so retries and
  regenerations reuse the provider endpoint and prompt cache.
- **Graceful cancellation** — Ctrl-C or SIGTERM aborts cleanly at any step.
- **Library-friendly** — the same workflow is available programmatically
  via `tscommit.Run` and `tscommit.RunContext`.

---

## How It Works

1. Read the staged diff (truncated to 30,000 characters if needed).
2. Send the diff to the configured AI provider with a strict Conventional
   Commits JSON envelope prompt; decode the structured response, validate
   the commit message, and retry format/truncation violations with feedback.
3. Present the message and model confidence interactively: **commit**,
   **edit**, **regenerate**, or **cancel**.
4. On commit, create the git commit with the reviewed message.

---

## Getting Started

Requires Go 1.27 or later, git, and an [OpenRouter](https://openrouter.ai) API key.

There is no prebuilt binary; the CLI lives in `cmd/tscommit` and is built
from source.

Clone the repository and build the CLI:

```sh
git clone https://github.com/thorsphere/tscommit.git
cd tscommit
go build -o tscommit ./cmd/tscommit
```

Or run it directly without building:

```sh
go run ./cmd/tscommit
```

Provide your API key via the environment (twelve-factor style):

```sh
export OPENROUTER_API_KEY=sk-or-...
```

Stage your changes in the repository you want to commit in, then run the
binary (`./tscommit`, or `tscommit` if you put it on your `PATH`):

```sh
git add -A
./tscommit
```

You will be shown the generated message and asked what to do:

```sh
feat(git): add staged-diff truncation

Large diffs previously overflowed the prompt window. Truncate at 30k
characters so hunks stay parseable.

Confidence: 78%

Apply this commit? [Y/e/r/n] (Y=commit, e=edit, r=regenerate, n=cancel):
```

- commit: create the git commit with the shown message.
- edit: open the message in $EDITOR; the edited text is used as-is (and the AI confidence line is omitted on re-prompt).
- regenerate: ask the model for a fresh message and confidence score for the same diff.
- cancel: abort without committing.

### Confidence Score Rubric

The model scores its confidence (`0.0`–`1.0`, displayed as `0%`–`100%`) against the staged diff:

| Score | Meaning |
| :--- | :--- |
| **85%–100%** | Small, self-contained diff with one unambiguous intent and explicit `WHY` in comments, tests, or error messages. |
| **65%–84%** | Clear `WHAT`, but `WHY`/motivation is inferred from code context or several related files are touched. |
| **40%–64%** | Large or multi-concern diff, ambiguous scope/type, missing context, or uncertain breaking-change impact. |
| **0%–39%** | Truncated diff (`... (truncated)`), generated/opaque changes, or intent is mostly guesswork. |

---

## Configuration

All configuration comes from environment variables; there are no config
files.


| Variable | Required | Description |
| :--- | :--- | :--- |
| `OPENROUTER_API_KEY` | yes | OpenRouter API key used by the AI client. Read by the CLI at startup; missing key fails fast with a clear error. |
| `EDITOR` | no | Editor opened for the `edit` action (inherited from your shell/git setup). |

Generation defaults are fixed for consistency: max 16,000 completion
tokens, temperature 0.3, medium reasoning effort, and a 3-minute request
timeout.

---

## API reference

The root package exposes the workflow as a library:

```go
// Run executes the tscommit workflow with a background context.
func Run() error

// RunContext executes the tscommit workflow with the provided context.
func RunContext(ctx context.Context) error
```

Typical embedding:

```go
package main

import (
    "context"

    "github.com/thorsphere/tscommit"
)

func main() {
    if err := tscommit.RunContext(context.Background()); err != nil {
        panic(err)
    }
}
```

See the [Go Package Documentation](https://pkg.go.dev/github.com/thorsphere/tscommit) for the complete reference.

---

## Known Limitations

- Diffs longer than 30,000 characters are truncated (marked with a truncation marker); hunks beyond the limit are not described.
- Only staged changes are considered — unstaged or untracked work is invisible to the generator.
- The CLI is interactive and expects a TTY for prompting and editing; it is not intended for unattended CI use.
- Only the OpenRouter provider is supported, via tsaiclient.
- Messages you edit yourself are intentionally not re-validated. Your text is authoritative.
- AI-generated output quality and self-reported confidence scores depend on the underlying model; envelope/schema violations fail immediately, while message-format and truncation violations are retried once with feedback before failing.

---

## Documentation & Resources

- [Go Package Documentation](https://pkg.go.dev/github.com/thorsphere/tscommit): Complete API reference
- [Open Source Insights](https://deps.dev/go/github.com%2Fthorsphere%2Ftscommit): Dependency analysis

---

## ⚖️ License & Commercial Usage

Copyright (c) 2026 thorsphere. All rights reserved.

This project is licensed under the **Functional Source License v1.1 (FSL-1.1-ALv2)**. 

* The use, modification, and redistribution of this Go package is completely free for private, educational, non-commercial, and internal purposes. 
* If you are a company or institution looking to use this package in a commercial product, service, or business environment, you must secure a commercial license.
* Each version of this software automatically converts to the fully open-source Apache License, Version 2.0 on the second anniversary of its release.

For full details, please see the [LICENSE](LICENSE.md) file.

### 💼 Commercial Licensing & Inquiries

To purchase a commercial license or discuss support options, please reach out directly:

* 📩 **Contact:** business at thorsphere dot com
* 💬 **Response Time:** Usually within a couple of business days.

*Please include your company name and a brief overview of your use case so I can provide the right licensing details.*

