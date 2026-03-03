package git

import (
	"claude-squad/config"
	"claude-squad/log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	defer log.Close()
	os.Exit(m.Run())
}

func TestGetWorktreeDirectoryForRepo_Subdirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = config.WorktreeRootSubdirectory
	require.NoError(t, config.SaveConfig(cfg))

	worktreeDir, err := getWorktreeDirectoryForRepo(t.TempDir())
	require.NoError(t, err)

	configDir, err := config.GetConfigDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, "worktrees"), worktreeDir)
}

func TestGetWorktreeDirectoryForRepo_Sibling(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	repoRoot := createGitRepo(t)

	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = config.WorktreeRootSibling
	require.NoError(t, config.SaveConfig(cfg))

	worktreeDir, err := getWorktreeDirectoryForRepo(repoRoot)
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(repoRoot), worktreeDir)
}

func TestGetWorktreeDirectoryForRepo_SiblingRequiresRepoPath(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cfg := config.DefaultConfig()
	cfg.WorktreeRoot = config.WorktreeRootSibling
	require.NoError(t, config.SaveConfig(cfg))

	_, err := getWorktreeDirectoryForRepo("")
	require.Error(t, err)
}

func createGitRepo(t *testing.T) string {
	t.Helper()
	repoRoot := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(repoRoot, 0755))

	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	return repoRoot
}
