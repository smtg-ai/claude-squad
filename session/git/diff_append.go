package git

import (
	"fmt"
	"strings"
)

// DiffCommitAtOffset returns the diff of a commit at the specified offset from HEAD
// offset 0 = HEAD, offset 1 = HEAD~1, etc.
func (g *GitWorktree) DiffCommitAtOffset(offset int) *DiffStats {
	stats := &DiffStats{}

	// Get the diff of the commit at the specified offset
	var content string
	var err error
	
	if offset == 0 {
		// For HEAD, show diff between HEAD^ and HEAD
		content, err = g.runGitCommand(g.worktreePath, "--no-pager", "diff", "HEAD^..HEAD")
		if err != nil {
			// If HEAD^ doesn't exist (first commit), try to show the commit
			content, err = g.runGitCommand(g.worktreePath, "--no-pager", "show", "--format=", "HEAD")
			if err != nil {
				stats.Error = err
				return stats
			}
		}
	} else {
		// For older commits, show diff between commit~1 and commit
		fromRef := fmt.Sprintf("HEAD~%d", offset+1)
		toRef := fmt.Sprintf("HEAD~%d", offset)
		content, err = g.runGitCommand(g.worktreePath, "--no-pager", "diff", fromRef+".."+toRef)
		if err != nil {
			// If the parent doesn't exist, try to show the commit itself
			content, err = g.runGitCommand(g.worktreePath, "--no-pager", "show", "--format=", toRef)
			if err != nil {
				stats.Error = err
				return stats
			}
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

// GetCommitInfo returns the commit hash and message at the specified offset
func (g *GitWorktree) GetCommitInfo(offset int) (hash string, message string, err error) {
	ref := "HEAD"
	if offset > 0 {
		ref = fmt.Sprintf("HEAD~%d", offset)
	}
	
	// Get commit hash
	hash, err = g.runGitCommand(g.worktreePath, "rev-parse", "--short", ref)
	if err != nil {
		return "", "", err
	}
	hash = strings.TrimSpace(hash)
	
	// Get commit message (first line only)
	message, err = g.runGitCommand(g.worktreePath, "log", "-1", "--pretty=%s", ref)
	if err != nil {
		return "", "", err
	}
	message = strings.TrimSpace(message)
	
	return hash, message, nil
}

// DiffUncommitted returns only the uncommitted changes (staged and unstaged)
func (g *GitWorktree) DiffUncommitted() *DiffStats {
	stats := &DiffStats{}

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