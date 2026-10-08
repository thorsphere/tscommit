// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

const promptTemplate = `You are an expert Git commit assistant. Generate a high-quality commit message based on the staged diff below.

STRICT FORMAT RULES:
1. Header Line: <type>[optional scope][!]: <short description>
   - Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore.
   - Scope: optional, lowercase, derived from the primary package/directory changed (e.g. "git", "gateway").
   - Use imperative, present tense ("rename" not "renamed").
   - Max 72 characters; no trailing period.
   - Summarize the single dominant intent, not a file-by-file list.
2. Blank Line: exactly one empty line between header and body.
3. Body: wrapped at 72 characters. Explain WHAT changed, WHY (intent,
   context, or issue solved), and relevant side-effects.
4. Breaking Changes: if the diff removes/changes public APIs, CLI flags,
   config keys, file formats, or behavior that callers must adapt to:
   - Append "!" after type/scope in the header (e.g. "feat(api)!: ...").
   - Add a footer on its own line: "BREAKING CHANGE: <what broke and how
     to migrate>".
   - Only use this when action is actually required from consumers.
5. Footers (optional, one per line, trailer format "Token: value"):
   - "Refs #<issue>" when the diff context references an issue.
   - "BREAKING CHANGE: <description>" as described above.
6. Style: match the tone, terminology, and conventions of the recent
   commit messages listed below. Use them as style reference only — do
   not copy them, quote them, or mention them in the output.
7. Output Format: raw plain text only. No markdown code blocks, quotes,
   or intro/outro commentary.

Example output:
feat(git): add staged-diff truncation

Large diffs previously overflowed the prompt window. Truncate at 30k
bytes on a line boundary so hunks stay parseable.

Recent commit messages (style reference only):
`

const diffSection = `

Staged diff:
`

// buildPrompt assembles the AI prompt from recent commit subjects and the
// staged diff.
func buildPrompt(recent, diff string) string {
	return promptTemplate + recent + diffSection + diff
}
