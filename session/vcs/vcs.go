package vcs

import (
	"claude-squad/log"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// DiffStats represents the statistics of a diff operation
type DiffStats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
	Output       string
	Error        error // Added field to store error
}

// IsEmpty returns true if there are no changes in the diff stats.
func (d *DiffStats) IsEmpty() bool {
	return d.FilesChanged == 0 && d.Insertions == 0 && d.Deletions == 0
}

// VCS defines the interface for version control system operations
type VCS interface {
	Diff() *DiffStats
	PushChanges(commitMessage string, open bool) error
	CommitChanges(commitMessage string) error
	IsDirty() (bool, error)
	IsBranchCheckedOut() (bool, error)
	OpenBranchURL() error
	Setup() error
	SetupFromExistingBranch() error
	SetupNewWorktree() error
	Cleanup() error
	Remove() error
	Prune() error
	GetWorktreePath() string
	GetBranchName() string
	GetRepoPath() string
	GetRepoName() string
	GetBaseCommitSHA() string
}

// GitWorktree manages git worktree operations for a session
type GitWorktree struct {
	repoPath      string
	worktreePath  string
	sessionName   string
	branchName    string
	baseCommitSHA string
}

// NewGitWorktreeFromStorage creates a new GitWorktree instance from stored data
func NewGitWorktreeFromStorage(repoPath string, worktreePath string, sessionName string, branchName string, baseCommitSHA string) *GitWorktree {
	return &GitWorktree{
		repoPath:      repoPath,
		worktreePath:  worktreePath,
		sessionName:   sessionName,
		branchName:    branchName,
		baseCommitSHA: baseCommitSHA,
	}
}

// NewGitWorktree creates a new GitWorktree instance
func NewGitWorktree(repoPath string, sessionName string) (tree *GitWorktree, branchname string, err error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		log.ErrorLog.Printf("git worktree path abs error, falling back to repoPath %s: %s", repoPath, err)
		absPath = repoPath
	}

	repoPath, err = findGitRepoRoot(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find git repository root: %w", err)
	}

	branchname = SanitizeBranchName(sessionName)

	return &GitWorktree{
		repoPath:     repoPath,
		worktreePath: filepath.Join(repoPath, ".git", "worktrees", branchname),
		sessionName:  sessionName,
		branchName:   branchname,
	}, branchname, nil
}

func (g *GitWorktree) GetWorktreePath() string {
	return g.worktreePath
}

func (g *GitWorktree) GetBranchName() string {
	return g.branchName
}

func (g *GitWorktree) GetRepoPath() string {
	return g.repoPath
}

func (g *GitWorktree) GetRepoName() string {
	return filepath.Base(g.repoPath)
}

func (g *GitWorktree) GetBaseCommitSHA() string {
	return g.baseCommitSHA
}

// Diff returns the git diff between the worktree and the base branch along with statistics
func (g *GitWorktree) Diff() *DiffStats {
	// Stage all changes in the worktree to get a comprehensive diff
	_, err := g.runGitCommand(g.worktreePath, "add", "-N", ".")
	if err != nil {
		log.ErrorLog.Printf("Error staging changes for diff: %v", err)
		return &DiffStats{Output: fmt.Sprintf("Error staging changes for diff: %v", err)}
	}

	content, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", g.GetBaseCommitSHA())
	if err != nil {
		log.ErrorLog.Printf("Error getting diff: %v", err)
		return &DiffStats{Output: fmt.Sprintf("Error getting diff: %v", err)}
	}

	filesChanged, insertions, deletions := parseDiffStats(content)

	return &DiffStats{
		FilesChanged: filesChanged,
		Insertions:   insertions,
		Deletions:    deletions,
		Output:       content,
	}
}

// PushChanges pushes the current branch to the remote repository
func (g *GitWorktree) PushChanges(commitMessage string, open bool) error {
	if err := checkGHCLI(); err != nil {
		return err
	}

	// Check if there are any changes to commit
	isDirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if isDirty {
		// Stage all changes
		if _, err := g.runGitCommand(g.worktreePath, "add", "."); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Create commit
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	// First push the branch to remote to ensure it exists
	pushCmd := exec.Command("gh", "repo", "sync", "--source", "-b", g.branchName)
	pushCmd.Dir = g.worktreePath
	if err := pushCmd.Run(); err != nil {
		// If sync fails, try creating the branch on remote first
		gitPushCmd := exec.Command("git", "push", "-u", "origin", g.branchName)
		gitPushCmd.Dir = g.worktreePath
		if pushOutput, pushErr := gitPushCmd.CombinedOutput(); pushErr != nil {
			log.ErrorLog.Print(pushErr)
			return fmt.Errorf("failed to push branch: %s (%w)", pushOutput, pushErr)
		}
	}

	// Now sync with remote
	syncCmd := exec.Command("gh", "repo", "sync", "-b", g.branchName)
	syncCmd.Dir = g.worktreePath
	if output, err := syncCmd.CombinedOutput(); err != nil {
		log.ErrorLog.Print(err)
		return fmt.Errorf("failed to sync changes: %s (%w)", output, err)
	}

	// Open the branch in the browser
	if open {
		if err := g.OpenBranchURL(); err != nil {
			// Just log the error but don't fail the push operation
			log.ErrorLog.Printf("failed to open branch URL: %v", err)
		}
	}

	return nil
}

// CommitChanges commits the current changes in the worktree
func (g *GitWorktree) CommitChanges(commitMessage string) error {
	// Check if there are any changes to commit
	isDirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if isDirty {
		// Stage all changes
		if _, err := g.runGitCommand(g.worktreePath, "add", "."); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to stage changes: %w", err)
		}

		// Create commit (local only)
		if _, err := g.runGitCommand(g.worktreePath, "commit", "-m", commitMessage, "--no-verify"); err != nil {
			log.ErrorLog.Print(err)
			return fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	return nil
}

// IsDirty checks if the worktree has uncommitted changes
func (g *GitWorktree) IsDirty() (bool, error) {
	output, err := g.runGitCommand(g.worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check worktree status: %w", err)
	}
	return len(strings.TrimSpace(output)) > 0, nil
}

// IsBranchCheckedOut checks if the session's branch is currently checked out in the main repository
func (g *GitWorktree) IsBranchCheckedOut() (bool, error) {
	output, err := g.runGitCommand(g.repoPath, "branch", "--show-current")
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(output) == g.branchName, nil
}

// OpenBranchURL opens the URL for the current branch on GitHub
func (g *GitWorktree) OpenBranchURL() error {
	// Check if GitHub CLI is available
	if err := checkGHCLI(); err != nil {
		return err
	}

	cmd := exec.Command("gh", "browse", "--branch", g.branchName)
	cmd.Dir = g.worktreePath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open branch URL: %w", err)
	}
	return nil
}

// Setup initializes the git worktree for the session
func (g *GitWorktree) Setup() error {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	// Check if the branch already exists
	_, err = repo.Branch(g.branchName)
	if err == nil {
		// Branch exists, set up from existing branch
		return g.SetupFromExistingBranch()
	} else if err == git.ErrBranchNotFound {
		// Branch does not exist, set up new worktree
		return g.SetupNewWorktree()
	}
	return fmt.Errorf("failed to check for existing branch: %w", err)
}

// SetupFromExistingBranch sets up the worktree from an already existing branch
func (g *GitWorktree) SetupFromExistingBranch() error {
	log.InfoLog.Printf("Setting up worktree from existing branch %s...", g.branchName)

	// Clean up any stale worktree entry
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", g.worktreePath, g.branchName); err != nil {
		return fmt.Errorf("failed to add worktree from existing branch: %w", err)
	}
	return nil
}

// SetupNewWorktree sets up a new git worktree and branch
func (g *GitWorktree) SetupNewWorktree() error {
	log.InfoLog.Printf("Setting up new worktree and branch %s...", g.branchName)

	// Clean up any stale worktree entry
	_, _ = g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath) // Ignore error if worktree doesn't exist

	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	headCommit := headRef.Hash().String()
	g.baseCommitSHA = headCommit // Store the base commit SHA

	log.InfoLog.Printf("Creating new worktree at %s with branch %s from commit %s", g.worktreePath, g.branchName, headCommit)
	if _, err := g.runGitCommand(g.repoPath, "worktree", "add", "-b", g.branchName, g.worktreePath, headCommit); err != nil {
		return fmt.Errorf("failed to add new worktree: %w", err)
	}
	return nil
}

// Cleanup removes the worktree and deletes the branch
func (g *GitWorktree) Cleanup() error {
	log.InfoLog.Printf("Cleaning up worktree at %s and branch %s...", g.worktreePath, g.branchName)

	// Remove the worktree using git command
	if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
		log.ErrorLog.Printf("Failed to remove worktree %s: %v", g.worktreePath, err)
		// Continue to try and delete the branch even if worktree removal fails
	}

	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return fmt.Errorf("failed to open git repository for branch deletion: %w", err)
	}

	// Delete the branch locally
	err = repo.DeleteBranch(g.branchName)
	if err != nil {
		if err == git.ErrBranchNotFound {
			log.InfoLog.Printf("Branch %s not found, skipping deletion.", g.branchName)
		} else {
			return fmt.Errorf("failed to delete local branch %s: %w", g.branchName, err)
		}
	} else {
		log.InfoLog.Printf("Local branch %s deleted successfully.", g.branchName)
	}

	// Prune worktrees to remove any stale entries
	if err := g.Prune(); err != nil {
		log.ErrorLog.Printf("Failed to prune git worktrees: %v", err)
	}

	return nil
}

// Remove removes the worktree but keeps the branch
func (g *GitWorktree) Remove() error {
	log.InfoLog.Printf("Removing worktree at %s...", g.worktreePath)
	// Remove the worktree using git command
	if _, err := g.runGitCommand(g.repoPath, "worktree", "remove", "-f", g.worktreePath); err != nil {
		return fmt.Errorf("failed to remove worktree %s: %w", g.worktreePath, err)
	}
	return nil
}

// Prune prunes stale git worktree entries
func (g *GitWorktree) Prune() error {
	log.InfoLog.Println("Pruning git worktrees...")
	if _, err := g.runGitCommand(g.repoPath, "worktree", "prune"); err != nil {
		return fmt.Errorf("failed to prune git worktrees: %w", err)
	}
	return nil
}

// runGitCommand executes a git command and returns any error
func (g *GitWorktree) runGitCommand(path string, args ...string) (string, error) {
	baseArgs := []string{}
	if path != "" {
		baseArgs = append(baseArgs, "-C", path)
	}
	cmd := exec.Command("git", append(baseArgs, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", string(output), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SanitizeBranchName transforms an arbitrary string into a Git branch name friendly string.
// Note: Git branch names have several rules, so this function uses a simple approach
// by allowing only a safe subset of characters.
func SanitizeBranchName(s string) string {
	// Convert to lower-case
	s = strings.ToLower(s)

	// Replace spaces with a dash
	s = strings.ReplaceAll(s, " ", "-")

	// Remove any characters not allowed in our safe subset.
	// Here we allow: letters, digits, dash, underscore, slash, and dot.
	re := regexp.MustCompile(`[^a-z0-9\-_/.]+`)
	s = re.ReplaceAllString(s, "")

	// Replace multiple dashes with a single dash (optional cleanup)
	reDash := regexp.MustCompile(`-+`)
	s = reDash.ReplaceAllString(s, "-")

	// Trim leading and trailing dashes or slashes to avoid issues
	s = strings.Trim(s, "-/.") // Added '.' to trim

	// Git branch names cannot end with a slash or contain '..'
	s = strings.TrimSuffix(s, "/")
	s = strings.ReplaceAll(s, "..", "-")

	// Limit length to avoid issues, Git has a practical limit around 255 chars
	if len(s) > 100 {
		s = s[:100]
	}

	return s
}

// checkGHCLI checks if GitHub CLI is installed and configured
func checkGHCLI() error {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI (gh) is not installed. Please install it first: https://cli.github.com/")
	}

	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("GitHub CLI is not configured. Please run 'gh auth login' first: %w", err)
	}

	return nil
}

// IsGitRepo checks if the given path is within a git repository
func IsGitRepo(path string) bool {
	for {
		_, err := git.PlainOpen(path)
		if err == nil {
			return true
		}

		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
}

// findGitRepoRoot finds the root directory of the git repository
func findGitRepoRoot(path string) (string, error) {
	currentPath := path
	for {
		if IsGitRepo(currentPath) {
			return currentPath, nil
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// Reached root directory
			break
		}
		currentPath = parentPath
	}
	return "", fmt.Errorf("failed to find Git repository root from path: %s", path)
}

// parseDiffStats parses the output of git diff to extract file change statistics
func parseDiffStats(diffOutput string) (filesChanged, insertions, deletions int) {
	lines := strings.Split(diffOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
			// Example: "3 files changed, 2 insertions(+), 1 deletion(-)"
			// Example: "1 file changed, 1 insertion(+)"
			// Example: "1 file changed, 1 deletion(-)"
			parts := strings.Split(line, ", ")
			for _, part := range parts {
				if strings.Contains(part, "file") {
					fmt.Sscanf(part, "%d file", &filesChanged)
				} else if strings.Contains(part, "insertion") {
					fmt.Sscanf(part, "%d insertion", &insertions)
				} else if strings.Contains(part, "deletion") {
					fmt.Sscanf(part, "%d deletion", &deletions)
				}
			}
			break
		}
	}
	return
}

// cleanupExistingBranch performs a thorough cleanup of any existing branch or reference
func (g *GitWorktree) cleanupExistingBranch(repo *git.Repository) error {
	branchRef := plumbing.NewBranchReferenceName(g.branchName)

	// Try to remove the branch reference
	if err := repo.Storer.RemoveReference(branchRef); err != nil && err != plumbing.ErrReferenceNotFound {
		return fmt.Errorf("failed to remove branch reference %s: %w", g.branchName, err)
	}

	// Remove any worktree-specific references
	worktreeRef := plumbing.NewReferenceFromStrings(
		fmt.Sprintf("worktrees/%s/HEAD", g.branchName),
		"",
	)
	if err := repo.Storer.RemoveReference(worktreeRef.Name()); err != nil && err != plumbing.ErrReferenceNotFound {
		return fmt.Errorf("failed to remove worktree reference for %s: %w", g.branchName, err)
	}

	// Clean up configuration entries
	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("failed to get repository config: %w", err)
	}

	delete(cfg.Branches, g.branchName)
	worktreeSection := fmt.Sprintf("worktree.%s", g.branchName)
	cfg.Raw.RemoveSection(worktreeSection)

	if err := repo.Storer.SetConfig(cfg); err != nil {
		return fmt.Errorf("failed to update repository config after removing branch %s: %w", g.branchName, err)
	}

	return nil
}

// combineErrors combines multiple errors into a single error
func CombineErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple errors occurred:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return errors.New(errMsg)
}

// GitCleanupWorktrees removes all git worktrees and their associated branches
func GitCleanupWorktrees() error {
	worktreesDir, err := getWorktreeDirectory()
	if err != nil {
		return fmt.Errorf("failed to get worktree directory: %w", err)
	}

	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return fmt.Errorf("failed to read worktree directory: %w", err)
	}

	// Get a list of all branches associated with worktrees
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Parse the output to extract branch names
	worktreeBranches := make(map[string]string)
	currentWorktree := ""
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branchPath := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/branch-name
			branchName := strings.TrimPrefix(branchPath, "refs/heads/")
			if currentWorktree != "" {
				worktreeBranches[currentWorktree] = branchName
			}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			worktreePath := filepath.Join(worktreesDir, entry.Name())

			// Delete the branch associated with this worktree if found
			for path, branch := range worktreeBranches {
				if strings.Contains(path, entry.Name()) {
					// Delete the branch
					deleteCmd := exec.Command("git", "branch", "-D", branch)
					if err := deleteCmd.Run(); err != nil {
						// Log the error but continue with other worktrees
						log.ErrorLog.Printf("failed to delete branch %s: %v", branch, err)
					}
					break
				}
			}

			// Remove the worktree directory
			os.RemoveAll(worktreePath)
		}
	}

	// You have to prune the cleaned up worktrees.
	cmd = exec.Command("git", "worktree", "prune")
	_, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	return nil
}

// getWorktreeDirectory returns the path to the .git/worktrees directory
func getWorktreeDirectory() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git dir: %w", err)
	}
	gitDir := strings.TrimSpace(string(output))
	return filepath.Join(gitDir, "worktrees"), nil
}

// JujutsuVCS (placeholder) manages Jujutsu operations for a session
type JujutsuVCS struct {
	repoPath      string
	worktreePath  string
	sessionName   string
	branchName    string
	baseCommitSHA string
}

// NewJujutsuVCS creates a new JujutsuVCS instance
func NewJujutsuVCS(repoPath string, sessionName string) (tree *JujutsuVCS, branchname string, err error) {
	return nil, "", fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) Diff() *DiffStats {
	return &DiffStats{Output: "Jujutsu VCS is not yet implemented"}
}

func (j *JujutsuVCS) PushChanges(commitMessage string, open bool) error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) CommitChanges(commitMessage string) error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) IsDirty() (bool, error) {
	return false, fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) IsBranchCheckedOut() (bool, error) {
	return false, fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) OpenBranchURL() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) Setup() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) SetupFromExistingBranch() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) SetupNewWorktree() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) Cleanup() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) Remove() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) Prune() error {
	return fmt.Errorf("Jujutsu VCS is not yet implemented")
}

func (j *JujutsuVCS) GetWorktreePath() string {
	return ""
}

func (j *JujutsuVCS) GetBranchName() string {
	return ""
}

func (j *JujutsuVCS) GetRepoPath() string {
	return ""
}

func (j *JujutsuVCS) GetRepoName() string {
	return ""
}

func (j *JujutsuVCS) GetBaseCommitSHA() string {
	return ""
}
