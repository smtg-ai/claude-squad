package git

import (
	"os"
	"strings"
)

// DiffStats holds statistics about the changes in a diff
type DiffStats struct {
	// Content is the full diff content
	Content string
	// Added is the number of added lines
	Added int
	// Removed is the number of removed lines
	Removed int
	// Error holds any error that occurred during diff computation
	// This allows propagating setup errors (like missing base commit) without breaking the flow
	Error error
}

func (d *DiffStats) IsEmpty() bool {
	return d.Added == 0 && d.Removed == 0 && d.Content == ""
}

// Diff returns the git diff between the worktree and the base branch along with statistics
func (g *GitWorktree) Diff() *DiffStats {
	// Use read lock to allow concurrent diff operations but prevent cleanup during diff
	g.opMu.RLock()
	defer g.opMu.RUnlock()

	stats := &DiffStats{}

	// Check if worktree directory exists before attempting git operations
	if _, err := os.Stat(g.worktreePath); os.IsNotExist(err) {
		// Worktree directory doesn't exist (likely being cleaned up), return empty stats
		return stats
	} else if err != nil {
		stats.Error = err
		return stats
	}

	// -N stages untracked files (intent to add), including them in the diff
	_, err := g.runGitCommand(g.worktreePath, "add", "-N", ".")
	if err != nil {
		// Check if error is due to missing directory (race condition)
		if strings.Contains(err.Error(), "No such file or directory") ||
			strings.Contains(err.Error(), "cannot change to") {
			// Directory was removed during operation, return empty stats
			return stats
		}
		stats.Error = err
		return stats
	}

	content, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", g.GetBaseCommitSHA())
	if err != nil {
		// Check if error is due to missing directory (race condition)
		if strings.Contains(err.Error(), "No such file or directory") ||
			strings.Contains(err.Error(), "cannot change to") {
			// Directory was removed during operation, return empty stats
			return stats
		}
		stats.Error = err
		return stats
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.Added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.Removed++
		}
	}
	stats.Content = content

	return stats
}
