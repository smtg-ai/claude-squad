package app

import (
	"claude-squad/config"
	"claude-squad/session"
	"claude-squad/ui"
	"claude-squad/ui/overlay"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepo creates a directory with an initialized git repo and returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "somerepo")
	require.NoError(t, os.Mkdir(repo, 0755))
	require.NoError(t, exec.Command("git", "-C", repo, "init").Run())
	return repo
}

func TestValidateRepoDir(t *testing.T) {
	t.Run("valid git repo returns absolute path", func(t *testing.T) {
		repo := initGitRepo(t)
		got, err := validateRepoDir(repo)
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got))
		assert.Equal(t, repo, got)
	})

	t.Run("nonexistent directory errors", func(t *testing.T) {
		_, err := validateRepoDir(filepath.Join(t.TempDir(), "nope"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("directory that is not a git repo errors", func(t *testing.T) {
		_, err := validateRepoDir(t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})

	t.Run("file errors", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file.txt")
		require.NoError(t, os.WriteFile(f, []byte("x"), 0644))
		_, err := validateRepoDir(f)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})
}

// newTestHome builds a minimal home for state machine tests. keySent is set so
// handleKeyPress skips the menu-highlighting re-send on the first key.
func newTestHome() *home {
	h := &home{
		ctx:       context.Background(),
		spinner:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		menu:      ui.NewMenu(),
		errBox:    ui.NewErrBox(),
		appConfig: config.DefaultConfig(),
		state:     stateDefault,
		program:   "echo",
		keySent:   true,
	}
	h.list = ui.NewList(&h.spinner, false)
	return h
}

func TestNewSessionKeyOpensDirectoryOverlay(t *testing.T) {
	h := newTestHome()
	h.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	assert.Equal(t, stateNewDir, h.state)
	require.NotNil(t, h.textInputOverlay)
	assert.False(t, h.promptAfterName)
	assert.Equal(t, 0, h.list.NumInstances(), "no instance should exist until a directory is chosen")
}

func TestNewSessionWithPromptKeyOpensDirectoryOverlay(t *testing.T) {
	h := newTestHome()
	h.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})

	assert.Equal(t, stateNewDir, h.state)
	require.NotNil(t, h.textInputOverlay)
	assert.True(t, h.promptAfterName)
}

func TestDirectoryOverlayEscCancels(t *testing.T) {
	h := newTestHome()
	h.state = stateNewDir
	h.promptAfterName = true
	h.textInputOverlay = overlay.NewPathInputOverlay("Directory", "")

	h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})

	assert.Equal(t, stateDefault, h.state)
	assert.Nil(t, h.textInputOverlay)
	assert.False(t, h.promptAfterName)
	assert.Equal(t, 0, h.list.NumInstances())
}

func TestDirectoryOverlayEmptySubmitUsesCurrentDirectory(t *testing.T) {
	h := newTestHome()
	h.state = stateNewDir
	h.textInputOverlay = overlay.NewPathInputOverlay("Directory", "")

	h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateNew, h.state)
	assert.Nil(t, h.textInputOverlay)
	require.Equal(t, 1, h.list.NumInstances())

	cwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, cwd, h.list.GetInstances()[0].Path)
}

func TestDirectoryOverlayValidSubmitUsesChosenDirectory(t *testing.T) {
	h := newTestHome()
	repo := initGitRepo(t)
	h.state = stateNewDir
	h.textInputOverlay = overlay.NewPathInputOverlay("Directory", repo)

	h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateNew, h.state)
	require.Equal(t, 1, h.list.NumInstances())
	assert.Equal(t, repo, h.list.GetInstances()[0].Path)
}

func TestDirectoryOverlayInvalidSubmitStaysOpen(t *testing.T) {
	h := newTestHome()
	h.state = stateNewDir
	badPath := filepath.Join(t.TempDir(), "definitely-missing")
	h.textInputOverlay = overlay.NewPathInputOverlay("Directory", badPath)

	h.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateNewDir, h.state, "invalid directory should keep the overlay open")
	require.NotNil(t, h.textInputOverlay)
	assert.False(t, h.textInputOverlay.IsSubmitted())
	assert.Equal(t, 0, h.list.NumInstances())

	h.textInputOverlay.SetSize(80, 10)
	assert.True(t, strings.Contains(h.textInputOverlay.Render(), "does not exist"),
		"overlay should display the validation error")
}

func TestMenuHighlightingSkippedInDirectoryState(t *testing.T) {
	h := newTestHome()
	h.state = stateNewDir
	h.keySent = false

	// Typing 'n' inside the directory overlay must not trigger menu
	// highlighting (which would swallow and re-send the keypress).
	_, returnEarly := h.handleMenuHighlighting(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	assert.False(t, returnEarly)
}

func TestRunBranchSearchUsesSelectedInstancePath(t *testing.T) {
	h := newTestHome()
	repo := initGitRepo(t)
	gitCmd := func(args ...string) {
		full := append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)
		out, err := exec.Command("git", full...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	gitCmd("commit", "--allow-empty", "-m", "init")
	gitCmd("branch", "feature-dirselect-zzz")

	instance, err := session.NewInstance(session.InstanceOptions{Title: "t", Path: repo, Program: "echo"})
	require.NoError(t, err)
	h.list.AddInstance(instance)
	h.list.SetSelectedInstance(0)

	msg := h.runBranchSearch("feature-dirselect-zzz", 1)()
	result, ok := msg.(branchSearchResultMsg)
	require.True(t, ok, "expected branchSearchResultMsg, got %T", msg)
	require.Contains(t, result.branches, "feature-dirselect-zzz",
		"branch search must run against the selected instance's repo, not the cwd")
}
