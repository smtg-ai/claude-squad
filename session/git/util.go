package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Pre-compiled regexes for branch name sanitization.
var (
	unsafeCharsRegex = regexp.MustCompile(`[^a-z0-9\-_/.]+`)
	multiDashRegex   = regexp.MustCompile(`-+`)
)

// sanitizeBranchName transforms an arbitrary string into a Git branch name friendly string.
func sanitizeBranchName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = unsafeCharsRegex.ReplaceAllString(s, "")
	s = multiDashRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-/")
	return s
}

// checkGHCLI checks if GitHub CLI is installed and configured
func checkGHCLI() error {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI (gh) is not installed. Please install it first")
	}

	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GitHub CLI is not configured. Please run 'gh auth login' first")
	}

	return nil
}

// IsGitRepo checks if the given path is within a git repository
func IsGitRepo(path string) bool {
	_, err := FindGitRepoRoot(path)
	return err == nil
}

// FindGitRepoRoot walks up from path until it finds a git repo root.
func FindGitRepoRoot(path string) (string, error) {
	currentPath := path
	for {
		_, err := git.PlainOpen(currentPath)
		if err == nil {
			// Found the repository root
			return currentPath, nil
		}

		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			// Reached the filesystem root without finding a repository
			return "", fmt.Errorf("failed to find Git repository root from path: %s", path)
		}
		currentPath = parent
	}
}

// runGitCommandStatic runs a git command with -C repoPath without needing a GitWorktree receiver.
func runGitCommandStatic(repoPath string, args ...string) (string, error) {
	baseArgs := []string{"-C", repoPath}
	cmd := exec.Command("git", append(baseArgs, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}
	return string(output), nil
}

// ListLocalBranches returns all local branch names for the repo at repoPath.
func ListLocalBranches(repoPath string) ([]string, error) {
	repoRoot, err := FindGitRepoRoot(repoPath)
	if err != nil {
		return nil, err
	}
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}
	iter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}
	var branches []string
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Name().Short())
		return nil
	})
	return branches, nil
}

// FindWorktreePathForBranch returns the checked-out worktree path for branchName,
// or ("", nil) if the branch exists but is not checked out anywhere.
func FindWorktreePathForBranch(repoPath, branchName string) (string, error) {
	repoRoot, err := FindGitRepoRoot(repoPath)
	if err != nil {
		return "", err
	}
	output, err := runGitCommandStatic(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}
	// Parse blocks separated by blank lines.
	// Each block looks like:
	//   worktree /path/to/wt
	//   HEAD abc123
	//   branch refs/heads/branchName
	currentPath := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimPrefix(line, "branch ")
			shortName := strings.TrimPrefix(branchRef, "refs/heads/")
			if shortName == branchName && currentPath != "" {
				return currentPath, nil
			}
		}
	}
	return "", nil
}
