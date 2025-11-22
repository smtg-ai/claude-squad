package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultBranch(t *testing.T) {
	// Create a temporary directory for test repos
	tempDir := t.TempDir()

	t.Run("detects main branch", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "repo-with-main")
		setupTestRepo(t, repoPath, "main")

		branch, err := GetDefaultBranch(repoPath)
		assert.NoError(t, err)
		assert.Equal(t, "main", branch)
	})

	t.Run("detects master branch", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "repo-with-master")
		setupTestRepo(t, repoPath, "master")

		branch, err := GetDefaultBranch(repoPath)
		assert.NoError(t, err)
		assert.Equal(t, "master", branch)
	})

	t.Run("prefers main over master", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "repo-with-both")
		setupTestRepo(t, repoPath, "main")
		// Also create a master branch
		cmd := exec.Command("git", "checkout", "-b", "master")
		cmd.Dir = repoPath
		require.NoError(t, cmd.Run())
		cmd = exec.Command("git", "checkout", "main")
		cmd.Dir = repoPath
		require.NoError(t, cmd.Run())

		branch, err := GetDefaultBranch(repoPath)
		assert.NoError(t, err)
		assert.Equal(t, "main", branch)
	})

	t.Run("handles missing default branch", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "repo-with-custom")
		setupTestRepo(t, repoPath, "develop")

		_, err := GetDefaultBranch(repoPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not determine default branch")
	})
}

// setupTestRepo creates a test git repository with the specified default branch
func setupTestRepo(t *testing.T, repoPath string, defaultBranch string) {
	t.Helper()

	// Create directory
	require.NoError(t, os.MkdirAll(repoPath, 0755))

	// Initialize repo with explicit initial branch
	cmd := exec.Command("git", "init", "-b", defaultBranch)
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Set user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	// Create initial commit
	testFile := filepath.Join(repoPath, "README.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test Repo"), 0644))

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	require.NoError(t, cmd.Run())
}
