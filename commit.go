// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package tscommit

import (
	"context"

	"github.com/thorsphere/tserr"
)

// Run executes the tscommit workflow using context.Background().
func Run(apiKey string) error {
	return RunContext(context.Background(), apiKey)
}

// RunContext executes the tscommit workflow with the provided context:
// reads the staged diff, generates a commit message via OpenRouter, and creates the commit.
func RunContext(ctx context.Context, apiKey string) error {
	cfg := DefaultConfig()
	p := newPrompter()

	diff, err := stagedDiff(ctx, 30000)
	if err != nil {
		return err
	}

	recent, err := recentCommits(ctx, 2000) // error handling per your preference
	if err != nil {
		return err
	}

	msg, err := genMessage(ctx, cfg.withAPIKey(apiKey), diff, recent) // <-- initial message
	if err != nil {
		return err
	}

	for {
		choice, err := promptUser(ctx, p, msg)
		if err != nil {
			return err
		}

		switch choice {
		case choiceAccept:
			return commit(ctx, msg)
		case choiceEdit:
			edited, err := editInEditor(ctx, p, msg)
			if err != nil {
				return err
			}
			msg = edited
		case choiceRegenerate:
			msg, err = genMessage(ctx, cfg.withAPIKey(apiKey), diff, recent) // <-- `=`, not `:=`
			if err != nil {
				return err
			}
		case choiceCancel:
			return tserr.Aborted("commit")
		}
	}
}
