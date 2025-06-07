package git

import (
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
	stats := &DiffStats{}

	// -N stages untracked files (intent to add), including them in the diff
	_, err := g.runGitCommand(g.worktreePath, "add", "-N", ".")
	if err != nil {
		stats.Error = err
		return stats
	}

	content, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", g.GetBaseCommitSHA())
	if err != nil {
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
