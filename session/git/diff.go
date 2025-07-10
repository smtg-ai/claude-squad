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
	// IsUncommitted indicates if this diff represents uncommitted changes
	IsUncommitted bool
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

// DiffUncommittedOrLastCommit returns uncommitted changes if they exist, otherwise the last commit diff
func (g *GitWorktree) DiffUncommittedOrLastCommit() *DiffStats {
	stats := &DiffStats{}

	// First, check if there are any uncommitted changes
	// Stage untracked files with intent to add
	_, err := g.runGitCommand(g.worktreePath, "add", "-N", ".")
	if err != nil {
		stats.Error = err
		return stats
	}

	// Get diff of uncommitted changes (including staged)
	content, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", "HEAD")
	if err != nil {
		stats.Error = err
		return stats
	}

	// If there are uncommitted changes, return them
	if content != "" {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				stats.Added++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				stats.Removed++
			}
		}
		stats.Content = content
		stats.IsUncommitted = true
		return stats
	}

	// No uncommitted changes, show the last commit
	content, err = g.runGitCommand(g.worktreePath, "--no-pager", "diff", "HEAD^..HEAD")
	if err != nil {
		// If HEAD^ doesn't exist (first commit), try to show the commit
		content, err = g.runGitCommand(g.worktreePath, "--no-pager", "show", "--format=", "HEAD")
		if err != nil {
			stats.Error = err
			return stats
		}
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
