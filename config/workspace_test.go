package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopeConfigHome points $CLAUDE_SQUAD_HOME at a fresh tmp dir for the duration
// of one test and returns the dir. Cleanup is handled by t.TempDir.
func scopeConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := os.Getenv(ConfigHomeEnvVar)
	require.NoError(t, os.Setenv(ConfigHomeEnvVar, dir))
	t.Cleanup(func() {
		if prev == "" {
			_ = os.Unsetenv(ConfigHomeEnvVar)
		} else {
			_ = os.Setenv(ConfigHomeEnvVar, prev)
		}
	})
	return dir
}

func TestWorkspaceID_StableAndDistinct(t *testing.T) {
	a := WorkspaceID("/a/b", "git@github.com:foo/bar.git")
	a2 := WorkspaceID("/a/b", "git@github.com:foo/bar.git")
	b := WorkspaceID("/a/b", "git@github.com:foo/other.git")
	c := WorkspaceID("/other/path", "git@github.com:foo/bar.git")

	assert.Equal(t, a, a2, "same (path, remote) must hash to same id")
	assert.NotEqual(t, a, b, "different remote must produce different id")
	assert.NotEqual(t, a, c, "different path must produce different id")
	assert.Len(t, a, 12, "id is 12 hex chars (6 bytes)")
}

func TestEnsureWorkspace_IdempotentAndPersistent(t *testing.T) {
	scopeConfigHome(t)

	reg := LoadWorkspaceRegistry()
	require.Empty(t, reg.Workspaces)

	ws1, err := reg.EnsureWorkspace("/Users/x/projects/foo", "git@github.com:x/foo.git")
	require.NoError(t, err)
	require.NotNil(t, ws1)
	assert.Equal(t, "foo", ws1.DisplayName)
	assert.Equal(t, "/Users/x/projects/foo", ws1.RepoPath)

	// Re-ensuring the same (path, remote) returns the same workspace and
	// doesn't create a duplicate.
	ws2, err := reg.EnsureWorkspace("/Users/x/projects/foo", "git@github.com:x/foo.git")
	require.NoError(t, err)
	assert.Equal(t, ws1.ID, ws2.ID)
	assert.Len(t, reg.Workspaces, 1)

	// Changing the remote URL produces a fresh workspace (real-world: user
	// swapped origin to a fork). This matches the documented hash behavior.
	ws3, err := reg.EnsureWorkspace("/Users/x/projects/foo", "git@github.com:x/fork.git")
	require.NoError(t, err)
	assert.NotEqual(t, ws1.ID, ws3.ID)
	assert.Len(t, reg.Workspaces, 2)

	// Persist + reload through disk to confirm both entries survive.
	reloaded := LoadWorkspaceRegistry()
	assert.Len(t, reloaded.Workspaces, 2)
	assert.NotNil(t, reloaded.Get(ws1.ID))
	assert.NotNil(t, reloaded.Get(ws3.ID))
}

func TestRegistry_GetFindRemoveTouch(t *testing.T) {
	scopeConfigHome(t)

	reg := LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace("/Users/x/foo", "")
	require.NoError(t, err)
	originalLastUsed := ws.LastUsedAt

	assert.Equal(t, ws.ID, reg.Get(ws.ID).ID)
	assert.Equal(t, ws.ID, reg.FindByName("foo").ID)
	assert.Equal(t, ws.ID, reg.FindByName("FOO").ID, "name lookup is case-insensitive")
	assert.Equal(t, ws.ID, reg.FindByRepoPath("/Users/x/foo").ID)
	assert.Nil(t, reg.Get("nope"))
	assert.Nil(t, reg.FindByName("nope"))

	time.Sleep(2 * time.Millisecond)
	require.NoError(t, reg.Touch(ws.ID))
	touched := reg.Get(ws.ID)
	assert.True(t, touched.LastUsedAt.After(originalLastUsed),
		"Touch must advance LastUsedAt")

	require.NoError(t, reg.Remove(ws.ID))
	assert.Nil(t, reg.Get(ws.ID))
	assert.Empty(t, LoadWorkspaceRegistry().Workspaces, "remove must persist")
}

func TestMostRecentlyUsed(t *testing.T) {
	scopeConfigHome(t)

	reg := LoadWorkspaceRegistry()
	assert.Nil(t, reg.MostRecentlyUsed(), "empty registry has no MRU")

	a, err := reg.EnsureWorkspace("/a", "")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	b, err := reg.EnsureWorkspace("/b", "")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	c, err := reg.EnsureWorkspace("/c", "")
	require.NoError(t, err)

	assert.Equal(t, c.ID, reg.MostRecentlyUsed().ID, "newest registration is MRU")

	require.NoError(t, reg.Touch(a.ID))
	assert.Equal(t, a.ID, reg.MostRecentlyUsed().ID, "Touch makes a workspace MRU")

	_ = b // keep referenced
}

// TestResolveEnv_FullProfile exercises the entire env-resolution flow against
// real files on disk: literal env vars, EnvFiles read from credentials/, and
// AgentHome paths resolved relative to the workspace dir.
func TestResolveEnv_FullProfile(t *testing.T) {
	home := scopeConfigHome(t)

	reg := LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace("/Users/x/my-repo", "")
	require.NoError(t, err)

	// Stage credentials/api.key with a trailing newline (common in shell `echo`
	// output). The resolver should trim the trailing newline.
	wsDir := filepath.Join(home, "workspaces", ws.ID)
	credDir := filepath.Join(wsDir, "credentials")
	require.NoError(t, os.MkdirAll(credDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(credDir, "api.key"), []byte("sk-token-xyz\n"), 0600))

	profile := &WorkspaceProfile{
		Name:    "codex",
		Program: "codex",
		Env: map[string]string{
			"PINNED_FLAG": "1",
		},
		EnvFiles: map[string]string{
			"OPENAI_API_KEY": "credentials/api.key",
		},
		AgentHome: map[string]string{
			"OPENAI_CONFIG_DIR": "credentials/openai-home",
		},
	}

	env, err := ws.ResolveEnv(profile)
	require.NoError(t, err)

	// Output should be sorted for stable display in `cs debug`.
	prev := ""
	for _, kv := range env {
		assert.GreaterOrEqual(t, kv, prev, "env entries should be sorted")
		prev = kv
	}

	got := map[string]string{}
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		require.Len(t, parts, 2)
		got[parts[0]] = parts[1]
	}

	assert.Equal(t, "1", got["PINNED_FLAG"], "literal env passed through")
	assert.Equal(t, "sk-token-xyz", got["OPENAI_API_KEY"],
		"EnvFiles read from disk and trailing newline trimmed")

	expectedAgentHome := filepath.Join(wsDir, "credentials/openai-home")
	assert.Equal(t, expectedAgentHome, got["OPENAI_CONFIG_DIR"],
		"AgentHome resolves to absolute path under workspace dir")
}

func TestResolveEnv_MissingEnvFileErrors(t *testing.T) {
	scopeConfigHome(t)
	reg := LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace("/Users/x/r", "")
	require.NoError(t, err)

	profile := &WorkspaceProfile{
		Name:     "p",
		EnvFiles: map[string]string{"SECRET": "credentials/nope.key"},
	}
	_, err = ws.ResolveEnv(profile)
	require.Error(t, err, "missing env_files target must error so the user notices")
	assert.Contains(t, err.Error(), "nope.key")
}

func TestResolveEnv_NilProfile(t *testing.T) {
	scopeConfigHome(t)
	reg := LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace("/Users/x/r", "")
	require.NoError(t, err)

	env, err := ws.ResolveEnv(nil)
	require.NoError(t, err)
	assert.Nil(t, env, "nil profile yields no env (preserves no-overlay default)")
}

func TestWorkspace_FindProfileAndPaths(t *testing.T) {
	home := scopeConfigHome(t)
	reg := LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace("/repo", "")
	require.NoError(t, err)
	ws.Profiles = []WorkspaceProfile{
		{Name: "claude", Program: "claude"},
		{Name: "codex", Program: "codex"},
	}
	require.NoError(t, reg.Upsert(*ws))

	got := reg.Get(ws.ID)
	require.NotNil(t, got.FindProfile("codex"))
	assert.Equal(t, "codex", got.FindProfile("codex").Program)
	assert.Nil(t, got.FindProfile("missing"))

	dir, err := got.Dir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "workspaces", got.ID), dir)

	root, err := got.WorktreeRoot()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "worktrees"), root, "default worktree root is under workspace dir")

	// Override pin to confirm WorktreeRoot honors WorkspaceDir.
	got.WorktreeDir = "/custom/path"
	require.NoError(t, reg.Upsert(*got))
	root, err = reg.Get(got.ID).WorktreeRoot()
	require.NoError(t, err)
	assert.Equal(t, "/custom/path", root)
}
