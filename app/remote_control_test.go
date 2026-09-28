package app

import (
	"claude-squad/cmd/cmd_test"
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/session/tmux"
	"claude-squad/ui"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRemoteControlTestHome(mode config.RemoteControlMode) *home {
	cfg := config.DefaultConfig()
	cfg.RemoteControl = mode
	return &home{
		ctx:       context.Background(),
		state:     stateNew,
		appConfig: cfg,
		menu:      ui.NewMenu(),
	}
}

func newUnstartedInstance(t *testing.T, program string) *session.Instance {
	t.Helper()
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test",
		Path:    t.TempDir(),
		Program: program,
	})
	require.NoError(t, err)
	return instance
}

func TestGetRemoteControlDefaultsWhenUnset(t *testing.T) {
	assert.Equal(t, config.DefaultRemoteControl, (&config.Config{}).GetRemoteControl())
	assert.Equal(t, config.DefaultRemoteControl, (&config.Config{RemoteControl: "bogus"}).GetRemoteControl())
	assert.Equal(t, config.RemoteControlNever, (&config.Config{RemoteControl: "never"}).GetRemoteControl())
}

func TestWithRemoteControlChoice(t *testing.T) {
	startCmd := func() tea.Msg { return nil }

	t.Run("ask mode defers start until y", func(t *testing.T) {
		h := newRemoteControlTestHome(config.RemoteControlAsk)
		instance := newUnstartedInstance(t, "/usr/local/bin/claude --verbose")
		started := false

		h.withRemoteControlChoice(instance, func() tea.Cmd { started = true; return startCmd })

		assert.False(t, started)
		assert.Equal(t, stateConfirm, h.state)
		require.NotNil(t, h.confirmationOverlay)

		_, cmd := h.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		assert.True(t, started)
		assert.True(t, instance.RemoteControl)
		assert.NotNil(t, cmd)
		assert.Nil(t, h.confirmationOverlay)
		assert.Nil(t, h.pendingConfirmCmd)
	})

	t.Run("ask mode starts without remote control on n", func(t *testing.T) {
		h := newRemoteControlTestHome(config.RemoteControlAsk)
		instance := newUnstartedInstance(t, "claude")
		started := false

		h.withRemoteControlChoice(instance, func() tea.Cmd { started = true; return startCmd })
		h.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

		assert.True(t, started)
		assert.False(t, instance.RemoteControl)
	})

	for _, tc := range []struct {
		name    string
		mode    config.RemoteControlMode
		program string
		want    bool
	}{
		{"always mode skips prompt", config.RemoteControlAlways, "claude", true},
		{"never mode skips prompt", config.RemoteControlNever, "claude", false},
		{"non-claude program skips prompt", config.RemoteControlAsk, "aider", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRemoteControlTestHome(tc.mode)
			instance := newUnstartedInstance(t, tc.program)
			started := false

			h.withRemoteControlChoice(instance, func() tea.Cmd { started = true; return startCmd })

			assert.True(t, started)
			assert.Equal(t, tc.want, instance.RemoteControl)
			assert.Nil(t, h.confirmationOverlay)
		})
	}
}

// recordingPtyFactory hands out a single temp file as the PTY so tests can read
// back what was written to it.
type recordingPtyFactory struct {
	path string
}

func (f recordingPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	return os.OpenFile(f.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
}

func (f recordingPtyFactory) Close() {}

func TestSendStartupMessagesCmd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := initTestRepo(t)
	ptyPath := filepath.Join(t.TempDir(), "pty")

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "rc",
		Path:    repo,
		Program: "claude",
	})
	require.NoError(t, err)

	created := false
	instance.SetTmuxSession(tmux.NewTmuxSessionWithDeps("rc", "claude", recordingPtyFactory{ptyPath}, cmd_test.MockCmdExec{
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
	require.NoError(t, instance.Start(true))
	t.Cleanup(func() { _ = instance.Kill() })

	assert.Nil(t, sendStartupMessagesCmd(instance), "nothing to send")

	instance.RemoteControl = true
	instance.Prompt = "hello"
	cmd := sendStartupMessagesCmd(instance)
	require.NotNil(t, cmd)
	assert.False(t, instance.RemoteControl)
	assert.Empty(t, instance.Prompt)

	assert.Nil(t, cmd())

	written, err := os.ReadFile(ptyPath)
	require.NoError(t, err)
	assert.Equal(t, remoteControlCommand+"\rhello\r", string(written))
}
