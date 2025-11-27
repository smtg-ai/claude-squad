package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SubmoduleInfo holds information about a git submodule
type SubmoduleInfo struct {
	// Path relative to the main repository
	Path string
	// URL of the submodule repository
	URL string
	// Current commit SHA of the submodule
	CommitSHA string
}

// SubmoduleWorktree manages a submodule's worktree
type SubmoduleWorktree struct {
	// Submodule info
	info SubmoduleInfo
	// Absolute path to the submodule's original repository
	SubmoduleRepoPath string
	// Absolute path to the submodule's worktree (inside main worktree)
	WorktreePath string
	// Branch name for the submodule worktree
	BranchName string
	// Base commit SHA for the submodule
	BaseCommitSHA string
}

// SubmoduleWorktreeData represents the serializable data of a SubmoduleWorktree
type SubmoduleWorktreeData struct {
	RelativePath      string `json:"relative_path"`
	URL               string `json:"url"`
	SubmoduleRepoPath string `json:"submodule_repo_path"`
	WorktreePath      string `json:"worktree_path"`
	BranchName        string `json:"branch_name"`
	BaseCommitSHA     string `json:"base_commit_sha"`
}

// GetSubmodules returns a list of submodules in the repository
func GetSubmodules(repoPath string) ([]SubmoduleInfo, error) {
	cmd := exec.Command("git", "-C", repoPath, "submodule", "status")
	output, err := cmd.Output()
	if err != nil {
		// No submodules or error - return empty list
		return nil, nil
	}

	var submodules []SubmoduleInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: " <sha> <path> (<branch>)" or "-<sha> <path>" (uninitialized)
		// Remove leading +/- if present
		if len(line) > 0 && (line[0] == '+' || line[0] == '-' || line[0] == ' ') {
			line = line[1:]
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			submodules = append(submodules, SubmoduleInfo{
				CommitSHA: parts[0],
				Path:      parts[1],
			})
		}
	}

	// Get URLs for each submodule
	for i := range submodules {
		url, err := getSubmoduleURL(repoPath, submodules[i].Path)
		if err == nil {
			submodules[i].URL = url
		}
	}

	return submodules, nil
}

// getSubmoduleURL gets the URL of a submodule
func getSubmoduleURL(repoPath, submodulePath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "config", "--file", ".gitmodules",
		fmt.Sprintf("submodule.%s.url", submodulePath))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetSubmoduleRepoPath returns the absolute path to a submodule's git directory
func GetSubmoduleRepoPath(mainRepoPath, submodulePath string) string {
	// Submodules store their git data in .git/modules/<path>
	return filepath.Join(mainRepoPath, ".git", "modules", submodulePath)
}

// NewSubmoduleWorktree creates a new SubmoduleWorktree instance
// mainRepoPath: the original repository path (for locating submodule git data in .git/modules/)
// mainWorktreePath: the worktree path where submodule worktree will be created
func NewSubmoduleWorktree(subInfo SubmoduleInfo, mainRepoPath, mainWorktreePath, branchName string) *SubmoduleWorktree {
	return &SubmoduleWorktree{
		info:              subInfo,
		SubmoduleRepoPath: GetSubmoduleRepoPath(mainRepoPath, subInfo.Path),
		WorktreePath:      filepath.Join(mainWorktreePath, subInfo.Path),
		BranchName:        branchName,
	}
}

// NewSubmoduleWorktreeFromStorage creates a SubmoduleWorktree from stored data
func NewSubmoduleWorktreeFromStorage(data SubmoduleWorktreeData) *SubmoduleWorktree {
	return &SubmoduleWorktree{
		info: SubmoduleInfo{
			Path: data.RelativePath,
			URL:  data.URL,
		},
		SubmoduleRepoPath: data.SubmoduleRepoPath,
		WorktreePath:      data.WorktreePath,
		BranchName:        data.BranchName,
		BaseCommitSHA:     data.BaseCommitSHA,
	}
}

// ToData converts SubmoduleWorktree to serializable data
func (s *SubmoduleWorktree) ToData() SubmoduleWorktreeData {
	return SubmoduleWorktreeData{
		RelativePath:      s.info.Path,
		URL:               s.info.URL,
		SubmoduleRepoPath: s.SubmoduleRepoPath,
		WorktreePath:      s.WorktreePath,
		BranchName:        s.BranchName,
		BaseCommitSHA:     s.BaseCommitSHA,
	}
}

// runGitCommand executes a git command in the specified path
func (s *SubmoduleWorktree) runGitCommand(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}
	return string(output), nil
}

// Setup creates a worktree for the submodule
func (s *SubmoduleWorktree) Setup() error {
	// Get current HEAD of the submodule
	output, err := s.runGitCommand(s.SubmoduleRepoPath, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get submodule HEAD (submodule may not be initialized): %w", err)
	}
	s.BaseCommitSHA = strings.TrimSpace(output)

	// First, try to remove any existing worktree at this path
	s.runGitCommand(s.SubmoduleRepoPath, "worktree", "remove", "-f", s.WorktreePath)

	// Check if branch already exists
	_, err = s.runGitCommand(s.SubmoduleRepoPath, "rev-parse", "--verify", s.BranchName)
	branchExists := err == nil

	if branchExists {
		// Create worktree from existing branch
		if _, err := s.runGitCommand(s.SubmoduleRepoPath, "worktree", "add",
			s.WorktreePath, s.BranchName); err != nil {
			return fmt.Errorf("failed to create submodule worktree from existing branch: %w", err)
		}
	} else {
		// Create new worktree with a new branch
		if _, err := s.runGitCommand(s.SubmoduleRepoPath, "worktree", "add",
			"-b", s.BranchName, s.WorktreePath, s.BaseCommitSHA); err != nil {
			return fmt.Errorf("failed to create submodule worktree: %w", err)
		}
	}

	return nil
}

// Cleanup removes the submodule worktree and branch
func (s *SubmoduleWorktree) Cleanup() error {
	// Remove the worktree (ignore error if worktree doesn't exist)
	s.runGitCommand(s.SubmoduleRepoPath, "worktree", "remove", "-f", s.WorktreePath)

	// Remove the branch (ignore error if branch doesn't exist)
	s.runGitCommand(s.SubmoduleRepoPath, "branch", "-D", s.BranchName)

	// Prune worktrees
	s.runGitCommand(s.SubmoduleRepoPath, "worktree", "prune")

	return nil
}

// Remove removes the worktree but keeps the branch (for pause operation)
func (s *SubmoduleWorktree) Remove() error {
	// Check if worktree path exists before attempting removal (consistent with main repo Cleanup)
	if _, err := os.Stat(s.WorktreePath); os.IsNotExist(err) {
		return nil // Worktree doesn't exist, nothing to remove
	}

	if _, err := s.runGitCommand(s.SubmoduleRepoPath, "worktree", "remove", "-f", s.WorktreePath); err != nil {
		return fmt.Errorf("failed to remove submodule worktree: %w", err)
	}
	return nil
}

// Restore recreates the worktree from the existing branch (for resume operation)
func (s *SubmoduleWorktree) Restore() error {
	// Remove any existing worktree first
	s.runGitCommand(s.SubmoduleRepoPath, "worktree", "remove", "-f", s.WorktreePath)

	// Create worktree from existing branch
	if _, err := s.runGitCommand(s.SubmoduleRepoPath, "worktree", "add",
		s.WorktreePath, s.BranchName); err != nil {
		return fmt.Errorf("failed to restore submodule worktree: %w", err)
	}

	return nil
}

// CommitChanges commits changes in the submodule worktree
func (s *SubmoduleWorktree) CommitChanges(commitMessage string) error {
	// Check if there are changes
	output, err := s.runGitCommand(s.WorktreePath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to check submodule status: %w", err)
	}

	if len(output) == 0 {
		return nil // No changes
	}

	// Stage all changes
	if _, err := s.runGitCommand(s.WorktreePath, "add", "."); err != nil {
		return fmt.Errorf("failed to stage submodule changes: %w", err)
	}

	// Commit
	if _, err := s.runGitCommand(s.WorktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
		return fmt.Errorf("failed to commit submodule changes: %w", err)
	}

	return nil
}

// PushChanges pushes changes in the submodule worktree to remote
func (s *SubmoduleWorktree) PushChanges() error {
	if _, err := s.runGitCommand(s.WorktreePath, "push", "-u", "origin", s.BranchName); err != nil {
		return fmt.Errorf("failed to push submodule changes: %w", err)
	}
	return nil
}

// IsDirty checks if the submodule worktree has uncommitted changes
func (s *SubmoduleWorktree) IsDirty() (bool, error) {
	output, err := s.runGitCommand(s.WorktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check submodule status: %w", err)
	}
	return len(output) > 0, nil
}
