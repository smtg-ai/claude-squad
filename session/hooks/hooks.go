package hooks

import (
	"claude-squad/log"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// WorktreeContext contains information passed to worktree setup hooks
type WorktreeContext struct {
	RepoPath     string
	WorktreePath string
	BranchName   string
	SessionName  string
}

// RunWorktreeSetupHook executes the configured setup hook after worktree creation.
// Returns nil if no hook is configured or if hook succeeds.
// Returns error only if failMode is "fail" and hook fails.
func RunWorktreeSetupHook(hookCmd string, failMode string, ctx WorktreeContext) error {
	if hookCmd == "" {
		return nil
	}

	log.InfoLog.Printf("Running worktree setup hook: %s", hookCmd)

	// Parse command - support both simple commands and shell expressions
	var cmd *exec.Cmd
	if strings.Contains(hookCmd, " ") {
		// Use shell to handle complex commands
		cmd = exec.Command("sh", "-c", hookCmd)
	} else {
		cmd = exec.Command(hookCmd)
	}

	// Set working directory to the new worktree
	cmd.Dir = ctx.WorktreePath

	// Set environment variables for hook context
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CS_REPO_PATH=%s", ctx.RepoPath),
		fmt.Sprintf("CS_WORKTREE_PATH=%s", ctx.WorktreePath),
		fmt.Sprintf("CS_BRANCH=%s", ctx.BranchName),
		fmt.Sprintf("CS_SESSION=%s", ctx.SessionName),
	)

	// Capture output for logging
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.ErrorLog.Printf("Worktree setup hook failed: %v\nOutput: %s", err, string(output))

		if failMode == "fail" {
			return fmt.Errorf("worktree setup hook failed: %w", err)
		}
		// Default: log and continue
		return nil
	}

	if len(output) > 0 {
		log.InfoLog.Printf("Worktree setup hook output:\n%s", string(output))
	}

	return nil
}
