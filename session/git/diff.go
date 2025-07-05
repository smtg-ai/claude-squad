package git

import (
	"strings"
)

func NewDiffStats(
	content string,
	added int,
	removed int,
	err error,
) *DiffStats {
	return &DiffStats{
		content: content,
		added:   added,
		removed: removed,
		err:     err,
	}
}

// DiffStats holds statistics about the changes in a diff
type DiffStats struct {
	// content is the full diff content after sanitizing the git diff output
	content string
	// added is the number of added lines
	added int
	// Removed is the number of removed lines
	removed int
	// err holds any error that occurred during diff computation
	// This allows propagating setup errors (like missing base commit) without breaking the flow
	err error
}

func (d *DiffStats) Content() string {
	return d.content
}

func (d *DiffStats) Added() int {
	return d.added
}

func (d *DiffStats) Removed() int {
	return d.removed
}

func (d *DiffStats) Error() error {
	return d.err
}

func (d *DiffStats) IsEmpty() bool {
	return d.added == 0 && d.removed == 0 && d.content == ""
}

// Diff returns the git diff between the worktree and the base branch along with statistics
func (g *GitWorktree) Diff() *DiffStats {
	stats := &DiffStats{}

	// -N stages untracked files (intent to add), including them in the diff
	_, err := g.runGitCommand(g.worktreePath, "add", "-N", ".")
	if err != nil {
		stats.err = err
		return stats
	}

	content, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", g.GetBaseCommitSHA())
	if err != nil {
		stats.err = err
		return stats
	}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.removed++
		}
	}
	stats.content = content

	return stats
}
