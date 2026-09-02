// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/thorsphere/tscommit"
	"github.com/thorsphere/tserr"
	"github.com/thorsphere/tslog"
)

func main() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		tslog.Error(tserr.NotSet("OPENROUTER_API_KEY"))
		os.Exit(1)
	}

	tslog.SetLevel(tslog.InfoLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := tscommit.RunContext(ctx, apiKey); err != nil {
		tslog.Error(err)
		os.Exit(1)
	}
}

/*
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/thorsphere/tserr"
	"github.com/thorsphere/tslog"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestPayload struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ResponsePayload struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func main() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		tslog.Error(tserr.NotSet("OPENROUTER_API_KEY"))
		os.Exit(1)
	}

	tslog.SetLevel(tslog.InfoLevel)

	cmd := exec.Command("git", "diff", "--staged")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		tslog.Info("No staged changes (use git add to add changes)")
		os.Exit(0)
	}

	diffText := string(out)
	if len(diffText) > 30000 {
		diffText = diffText[:30000] + "\n... (truncated)"
	}

	prompt := `You are an expert Git commit assistant. Generate a high-quality commit message based on the staged diff below.

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
` + diffText

	payload := RequestPayload{
		Model: "tencent/hy4-preview",
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "new http request", Fn: "OpenRouter", Err: err}))
		os.Exit(1)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/thorsphere/tscommit")
	req.Header.Set("X-OpenRouter-Title", "tscommit")
	req.Header.Set("X-OpenRouter-Categories", "cli-agent")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "send request", Fn: "OpenRouter", Err: err}))
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var res ResponsePayload
	if err := json.Unmarshal(respBody, &res); err != nil || len(res.Choices) == 0 {
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "unmarshal", Fn: "string(respBody)", Err: err}))
		os.Exit(1)
	}

	commitMsg := strings.TrimSpace(res.Choices[0].Message.Content)

	tslog.Info("--- Generated commit message ---\n" + commitMsg + "---------------------------------")

	// 3. Direkt committen mit der generierten Nachricht
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		tslog.Error(tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: err}))
		os.Exit(1)
	}
}
*/
