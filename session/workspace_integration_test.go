package session

import (
	"claude-squad/cmd/cmd_test"
	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingPtyFactory captures the exec.Cmd handed to it (which is the
// "tmux new-session ..." invocation) so the test can assert on the env vars
// the runtime would have injected.
type recordingPtyFactory struct {
	t    *testing.T
	cmds []*exec.Cmd
}

func (p *recordingPtyFactory) Start(c *exec.Cmd) (*os.File, error) {
	f, err := os.CreateTemp(p.t.TempDir(), "pty")
	if err != nil {
		return nil, err
	}
	p.cmds = append(p.cmds, c)
	return f, nil
}
func (p *recordingPtyFactory) Close() {}

// newTmuxMockExec returns a MockCmdExec wired the same way tmux_test.go does:
// `tmux has-session` returns an error (= session does not exist) on the first
// invocation so Start proceeds, and nil thereafter. Everything else succeeds.
func newTmuxMockExec() cmd_test.MockCmdExec {
	hasSessionCalled := false
	return cmd_test.MockCmdExec{
		RunFunc: func(c *exec.Cmd) error {
			if strings.Contains(c.String(), "has-session") && !hasSessionCalled {
				hasSessionCalled = true
				return fmt.Errorf("session does not exist")
			}
			return nil
		},
		OutputFunc: func(*exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
}

// initTestRepo creates a fresh git repo in a tmp dir with one empty commit
// (sufficient for `git worktree add HEAD` to succeed). Returns the abs path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@test"},
		{"config", "user.name", "test"},
		{"commit", "-q", "--allow-empty", "-m", "Initial commit"},
	} {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := c.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	return dir
}

// TestWorkspaceFlow_EndToEnd exercises the workspace flow from registration
// through Instance.Start, against a real git repo, real worktree creation,
// and a real post-worktree hook. The only thing mocked is tmux/PTY — we
// stop short of attaching but verify that the runtime would have passed the
// resolved env into `tmux new-session -e KEY=VAL ...`.
func TestWorkspaceFlow_EndToEnd(t *testing.T) {
	log.Initialize(false)
	t.Cleanup(log.Close)

	csHome := t.TempDir()
	t.Setenv(config.ConfigHomeEnvVar, csHome)
	t.Setenv("HOME", t.TempDir()) // isolate so worktree dir doesn't bleed into real ~/.claude-squad

	repoPath := initTestRepo(t)

	// 1. Register the workspace, attach a profile with literal env, an
	//    EnvFile, an AgentHome dir, and a post-worktree hook.
	reg := config.LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace(repoPath, "")
	require.NoError(t, err)

	wsDir, err := ws.Dir()
	require.NoError(t, err)
	credDir := filepath.Join(wsDir, "credentials")
	require.NoError(t, os.MkdirAll(credDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(credDir, "key"), []byte("sk-e2e\n"), 0600))
	hookPath := filepath.Join(credDir, "hook.sh")
	require.NoError(t, os.WriteFile(hookPath, []byte(
		"#!/bin/sh\necho \"session=$CS_SESSION branch=$CS_BRANCH\" > \"$CS_WORKTREE_PATH/.hook-fired\"\n",
	), 0700))

	ws.Profiles = []config.WorkspaceProfile{{
		Name:      "codex",
		Program:   "codex",
		Env:       map[string]string{"PINNED": "yes"},
		EnvFiles:  map[string]string{"API_KEY": "credentials/key"},
		AgentHome: map[string]string{"AGENT_HOME": "credentials/agent-home"},
	}}
	ws.Hooks.PostWorktree = "sh credentials/hook.sh"
	require.NoError(t, reg.Upsert(*ws))

	// 2. Build an Instance pointing at the workspace, inject a mock tmux
	//    session, and Start(true) it (the real-world session-create path).
	inst, err := NewInstance(InstanceOptions{
		Title:       "e2e-session",
		Path:        repoPath,
		Program:     "codex",
		WorkspaceID: ws.ID,
		ProfileName: "codex",
	})
	require.NoError(t, err)

	ptyFac := &recordingPtyFactory{t: t}
	exe := newTmuxMockExec()
	mockTmux := tmux.NewTmuxSessionWithDeps("e2e-session", "codex", ws.ID, ptyFac, exe)
	inst.SetTmuxSession(mockTmux)

	require.NoError(t, inst.Start(true))
	t.Cleanup(func() { _ = inst.Kill() })

	// 3. Assertions about the worktree placement and post-hook side effects.
	wt, err := inst.GetGitWorktree()
	require.NoError(t, err)
	wtPath := wt.GetWorktreePath()

	wantRoot, err := ws.WorktreeRoot()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(wtPath, wantRoot),
		"worktree %q must be under workspace WorktreeRoot %q (proves per-workspace placement)", wtPath, wantRoot)

	_, statErr := os.Stat(wtPath)
	require.NoError(t, statErr, "worktree dir should exist on disk after Setup")

	hookMarker := filepath.Join(wtPath, ".hook-fired")
	body, err := os.ReadFile(hookMarker)
	require.NoError(t, err, "post-worktree hook must produce the marker file")
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "session=e2e-session")
	assert.Contains(t, bodyStr, "branch=")

	// 4. Inspect the tmux invocation that the runtime queued. It should be
	//    "tmux new-session -e PINNED=yes -e API_KEY=sk-e2e -e AGENT_HOME=<abs> ..."
	require.NotEmpty(t, ptyFac.cmds, "tmux session must have been started")
	startCmd := ptyFac.cmds[0]
	joined := strings.Join(startCmd.Args, " ")
	assert.Contains(t, joined, "tmux new-session", "first PTY command should be the tmux start")

	// The -e flags get appended in map-iteration order, so we look for each pair
	// individually rather than asserting on the full argv.
	expectedAgentHome := filepath.Join(wsDir, "credentials/agent-home")
	for _, want := range []string{
		"PINNED=yes",
		"API_KEY=sk-e2e",
		fmt.Sprintf("AGENT_HOME=%s", expectedAgentHome),
	} {
		assert.True(t, containsArg(startCmd.Args, "-e", want),
			"tmux new-session argv should include '-e %s'; got: %v", want, startCmd.Args)
	}

	// 5. AgentHome's mkdir side effect: the runtime path mkdir's AgentHome
	//    targets (cs debug doesn't). Confirm it actually got created.
	info, err := os.Stat(expectedAgentHome)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestWorkspaceFlow_NoProfileNoOverlay verifies the back-compat fast path:
// a workspace with no profiles produces no env overlay and no hook execution,
// matching pre-workspace behavior.
func TestWorkspaceFlow_NoProfileNoOverlay(t *testing.T) {
	log.Initialize(false)
	t.Cleanup(log.Close)

	csHome := t.TempDir()
	t.Setenv(config.ConfigHomeEnvVar, csHome)
	t.Setenv("HOME", t.TempDir())

	repoPath := initTestRepo(t)
	reg := config.LoadWorkspaceRegistry()
	ws, err := reg.EnsureWorkspace(repoPath, "")
	require.NoError(t, err)

	inst, err := NewInstance(InstanceOptions{
		Title:       "plain",
		Path:        repoPath,
		Program:     "claude",
		WorkspaceID: ws.ID, // workspace set, but no profiles defined
	})
	require.NoError(t, err)

	ptyFac := &recordingPtyFactory{t: t}
	mockTmux := tmux.NewTmuxSessionWithDeps("plain", "claude", ws.ID, ptyFac, newTmuxMockExec())
	inst.SetTmuxSession(mockTmux)

	require.NoError(t, inst.Start(true))
	t.Cleanup(func() { _ = inst.Kill() })

	// Worktree still lands under the workspace dir even without profiles.
	wt, err := inst.GetGitWorktree()
	require.NoError(t, err)
	root, _ := ws.WorktreeRoot()
	assert.True(t, strings.HasPrefix(wt.GetWorktreePath(), root))

	// No -e flags in the tmux argv (no env overlay when no profile is defined).
	require.NotEmpty(t, ptyFac.cmds)
	for _, a := range ptyFac.cmds[0].Args {
		assert.False(t, strings.HasPrefix(a, "-e"),
			"no -e injection expected when workspace has no profiles, got %q", a)
	}
}

// TestWorkspaceMigration_BackfillsWorkspaceID exercises the on-load migration:
// instances persisted under the pre-workspace schema (no WorkspaceID, only a
// Worktree.RepoPath) should pick up a derived workspace on next load.
func TestWorkspaceMigration_BackfillsWorkspaceID(t *testing.T) {
	log.Initialize(false)
	t.Cleanup(log.Close)

	t.Setenv(config.ConfigHomeEnvVar, t.TempDir())
	t.Setenv("HOME", t.TempDir())

	repoPath := initTestRepo(t)
	reg := config.LoadWorkspaceRegistry()
	require.Empty(t, reg.Workspaces, "registry starts empty")

	// Simulate the migration code path: derive (canonical, remote) → id and
	// register. This mirrors what migrateInstancesToWorkspaces in main.go does
	// for each pre-existing instance.
	canonical := repoPath
	remote := git.FirstRemoteURL(canonical) // "" — no remote configured
	derived, err := reg.EnsureWorkspace(canonical, remote)
	require.NoError(t, err)
	assert.Equal(t, config.WorkspaceID(canonical, remote), derived.ID,
		"derived workspace id must equal the content-addressed hash")

	// Re-deriving for the same repo path is idempotent — the case where a
	// user has several pre-workspace instances all pointing at the same repo.
	again, err := reg.EnsureWorkspace(canonical, remote)
	require.NoError(t, err)
	assert.Equal(t, derived.ID, again.ID)
	assert.Len(t, reg.Workspaces, 1)
}

// containsArg returns true if args contains a "flag value" pair in that order.
// Used to assert presence of `-e KEY=VAL` regardless of position.
func containsArg(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
