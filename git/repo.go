package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// FindRepoRoot returns the filesystem path of the root of the git repository that contains the
// process’s current directory.
func FindRepoRoot(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to find git repo root folder - %s: %w", string(output), err)
	}

	return string(bytes.TrimSpace(output)), nil
}
