// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.

package tscommit

// promptTemplate is the instruction text prepended to the staged diff when
// asking the model to generate a commit message.
const promptTemplate = `You are an expert Git commit assistant. Generate a high-quality commit message based on the staged diff below.

STRICT FORMAT RULES:
1. Header Line: <type>(<scope>)[!]: <short description>
   - Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore.
   - Scope: required, lowercase, derived from the primary package/directory changed (e.g. "git", "gateway").
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
6. Output Format: respond with ONLY a single JSON object, no prose before
   or after it, in this exact shape:
   {"findings": ["<commit message>"], "confidence": <0.0-1.0>}
   - "findings": a single-element array containing the plain-text commit
     message (use "\n\n" between header and body; no markdown code fences
     or extra wrapper quotes inside the string).
   - "confidence": a calibrated number from 0.0 to 1.0 scored strictly
     from the diff using this rubric (do NOT default to 0.9+):
     * 0.85-1.00: small, self-contained diff with one unambiguous intent
       and explicit WHY in comments, tests, or error messages.
     * 0.65-0.84: clear WHAT, but WHY/motivation is inferred from code
       context or several related files are touched.
     * 0.40-0.64: large or multi-concern diff, ambiguous scope/type,
       missing context, or uncertain breaking-change impact.
     * 0.00-0.39: truncated diff ("... (truncated)"), generated/opaque
       changes, or intent is mostly guesswork.

Example output:
{"findings": ["feat(git): add staged-diff truncation\n\nLarge diffs previously overflowed the prompt window. Truncate at 30k\ncharacters so hunks stay parseable."], "confidence": 0.78}

Staged diff:
`

// buildPrompt assembles the AI prompt from the staged diff.
func buildPrompt(diff string) string {
	return promptTemplate + diff
}
