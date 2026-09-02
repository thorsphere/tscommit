// Copyright (c) 2026 thorsphere.
// All Rights Reserved. Use is governed by the Functional Source License v1.1
// (FSL-1.1-ALv2) that can be found in the LICENSE file.
package tscommit

import (
	"context"
	"os"
	"os/exec"

	"github.com/thorsphere/tserr"
)

// StagedDiff returns the staged diff, truncated to maxLen bytes.
// Returns an empty string if nothing is staged.
func stagedDiff(ctx context.Context, maxLen int) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "diff", "--staged").Output()
	if err != nil || len(out) == 0 {
		return "", tserr.NoChanges("no staged changes Git knows")
	}
	diff := string(out)
	if len(diff) > maxLen {
		diff = diff[:maxLen] + "\n... (truncated)"
	}
	return diff, nil
}

// Commit creates a git commit with the given message.
func commit(ctx context.Context, msg string) error {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", msg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return tserr.Op(&tserr.OpArgs{Op: "commit", Fn: "git", Err: err})
	}
	return nil
}
