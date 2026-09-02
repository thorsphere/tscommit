// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package tscommit

const promptTemplate = `You are an expert Git commit assistant. Generate a high-quality commit message based on the staged diff below.

STRICT FORMAT RULES:
1. Header Line: Must strictly follow Conventional Commits standard: <type>[optional scope]: <short description>
   - Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore.
   - Use imperative, present tense ("rename" not "renamed").
   - Do NOT end the header line with a period.
2. Blank Line: Separate header line and body with exactly one empty line.
3. Body Paragraph: Provide 1 to 3 clear sentences explaining:
   - WHAT was changed.
   - WHY it was changed (the intent, context, or issue solved).
   - Any relevant side-effects (e.g., "No content changes").
4. Output Format: Return ONLY raw plain text. Do NOT wrap the output in markdown code blocks, quotes, or intro/outro commentary.

Diff:
`

func buildPrompt(diff string) string {
	return promptTemplate + diff
}
