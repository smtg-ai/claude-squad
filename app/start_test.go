package app

import (
	"claude-squad/cmd/cmd_test"
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/session/tmux"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubPtyFactory hands out plain temp files instead of real PTYs.
type stubPtyFactory struct {
	t *testing.T
}

func (f stubPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	return os.OpenFile(
		filepath.Join(f.t.TempDir(), "pty"),
		os.O_CREATE|os.O_RDWR,
		0644,
	)
}

func (f stubPtyFactory) Close() {}

// initTestRepo creates a git repository with one commit so a worktree can be
// created from HEAD.
func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestStartInstanceAndSavePersistsWithoutFurtherSaves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := initTestRepo(t)

	storage, err := session.NewStorage(config.LoadState())
	require.NoError(t, err)

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "wip",
		Path:    repo,
		Program: "claude",
	})
	require.NoError(t, err)

	created := false
	instance.SetTmuxSession(tmux.NewTmuxSessionWithDeps("wip", "claude", stubPtyFactory{t}, cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			if strings.Contains(cmd.String(), "has-session") && !created {
				created = true
				return fmt.Errorf("session does not exist")
			}
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) {
			return []byte("output"), nil
		},
	}))

	require.NoError(t, startInstanceAndSave(instance, storage))

	// The record must already be on disk as another process would see it —
	// before the UI loop gets a chance to run any explicit save.
	stored := make([]session.InstanceData, 0)
	require.NoError(t, json.Unmarshal(config.LoadState().GetInstances(), &stored))
	require.Len(t, stored, 1)
	require.Equal(t, "wip", stored[0].Title)
}
