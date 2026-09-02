// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package tscommit

import "context"

// Run executes the tscommit workflow using context.Background().
func Run(apiKey string) error {
	return RunContext(context.Background(), apiKey)
}

// RunContext executes the tscommit workflow with the provided context:
// reads the staged diff, generates a commit message via OpenRouter, and creates the commit.
func RunContext(ctx context.Context, apiKey string) error {
	cfg := DefaultConfig()

	diff, err := stagedDiff(ctx, 30000)
	if err != nil {
		return err
	}

	msg, err := genMessage(ctx, cfg.withAPIKey(apiKey), diff)
	if err != nil {
		return err
	}

	return commit(ctx, msg)
}
