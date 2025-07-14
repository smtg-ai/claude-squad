package git

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// BranchInfo represents information about a git branch
type BranchInfo struct {
	Name         string
	CommitTime   time.Time
	CommitHash   string
	CommitMessage string
	IsRemote     bool
}

// ListRemoteBranchesFromRepo returns a list of remote branches sorted by most recent commit from a given repo path
func ListRemoteBranchesFromRepo(repoPath string) ([]BranchInfo, error) {
	// Find git repo root
	gitRoot, err := findGitRepoRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to find git repository: %w", err)
	}

	// Create a temporary GitWorktree just for running commands
	g := &GitWorktree{repoPath: gitRoot}
	
	return g.ListRemoteBranches()
}

// ListRemoteBranches returns a list of remote branches sorted by most recent commit
func (g *GitWorktree) ListRemoteBranches() ([]BranchInfo, error) {
	// Fetch latest remote branches
	if _, err := g.runGitCommand(g.repoPath, "fetch", "--all"); err != nil {
		// Continue even if fetch fails - we can still list cached remote branches
	}

	// Get all remote branches with their commit info
	output, err := g.runGitCommand(g.repoPath, "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)|%(committerdate:iso8601)|%(objectname:short)|%(subject)", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	branches := make([]BranchInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		// Skip HEAD references
		if strings.HasSuffix(parts[0], "/HEAD") {
			continue
		}

		// Parse commit time
		commitTime, err := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
		if err != nil {
			continue
		}

		// Remove origin/ prefix for display
		branchName := strings.TrimPrefix(parts[0], "origin/")

		branches = append(branches, BranchInfo{
			Name:          branchName,
			CommitTime:    commitTime,
			CommitHash:    parts[2],
			CommitMessage: parts[3],
			IsRemote:      true,
		})
	}

	// Sort by commit time (most recent first)
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].CommitTime.After(branches[j].CommitTime)
	})

	return branches, nil
}

// ListLocalBranches returns a list of local branches sorted by most recent commit
func (g *GitWorktree) ListLocalBranches() ([]BranchInfo, error) {
	// Get all local branches with their commit info
	output, err := g.runGitCommand(g.repoPath, "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)|%(committerdate:iso8601)|%(objectname:short)|%(subject)", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("failed to list local branches: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	branches := make([]BranchInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		// Parse commit time
		commitTime, err := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
		if err != nil {
			continue
		}

		branches = append(branches, BranchInfo{
			Name:          parts[0],
			CommitTime:    commitTime,
			CommitHash:    parts[2],
			CommitMessage: parts[3],
			IsRemote:      false,
		})
	}

	// Sort by commit time (most recent first)
	sort.Slice(branches, func(i, j int) bool {
		return branches[i].CommitTime.After(branches[j].CommitTime)
	})

	return branches, nil
}