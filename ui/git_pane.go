package ui

import (
	"fmt"
	"hivemind/session"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
)

const lazygitTmuxPrefix = "hivemind_lazygit_"

// GitPane manages an interactive lazygit subprocess inside a tmux session,
// rendered via tmux capture-pane through an EmbeddedTerminal.
type GitPane struct {
	sessionName string
	term        *session.EmbeddedTerminal
	mu          sync.Mutex

	currentInstanceTitle string
	worktreePath         string
	width, height        int
	errorMsg             string
}

// NewGitPane creates a new GitPane (no subprocess yet).
func NewGitPane() *GitPane {
	return &GitPane{}
}

// Spawn starts a lazygit process inside a tmux session in the given worktree directory.
// If lazygit is already running for a different instance, it kills the old one first.
func (g *GitPane) Spawn(worktreePath, instanceTitle string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Kill any existing subprocess first.
	g.killLocked()

	g.worktreePath = worktreePath
	g.currentInstanceTitle = instanceTitle
	g.errorMsg = ""

	// Check that lazygit is installed.
	if _, err := exec.LookPath("lazygit"); err != nil {
		g.errorMsg = "lazygit is not installed.\n\nInstall it: https://github.com/jesseduffield/lazygit#installation"
		return
	}

	sessionName := lazygitTmuxPrefix + sanitizeForTmux(instanceTitle)

	// Kill any stale session with this name.
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Create a detached tmux session running lazygit with quit keybindings disabled.
	// Quitting lazygit destroys the tmux session and crashes the TUI.
	// Users navigate back with Escape and exit focus mode with Ctrl+O.
	configArg := lazygitConfigArg()
	createCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath,
		"lazygit", "--use-config-file="+configArg)
	createPtmx, err := pty.Start(createCmd)
	if err != nil {
		g.errorMsg = fmt.Sprintf("Failed to create tmux session: %v", err)
		return
	}
	// Wait for the session to exist, then close the creation PTY.
	deadline := time.Now().Add(2 * time.Second)
	for !tmuxSessionExists(sessionName) {
		if time.Now().After(deadline) {
			createPtmx.Close()
			g.errorMsg = "Timed out waiting for lazygit tmux session"
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	createPtmx.Close()

	cols, rows := g.width, g.height
	if cols < 10 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}

	// NewEmbeddedTerminal creates its own tmux attach PTY + VT emulator
	term, err := session.NewEmbeddedTerminal(sessionName, cols, rows)
	if err != nil {
		exec.Command("tmux", "kill-session", "-t", sessionName).Run()
		g.errorMsg = fmt.Sprintf("Failed to create embedded terminal: %v", err)
		return
	}

	g.sessionName = sessionName
	g.term = term
}

// Kill stops the lazygit subprocess and cleans up resources.
func (g *GitPane) Kill() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.killLocked()
}

// killLocked performs the actual cleanup. Caller must hold g.mu.
func (g *GitPane) killLocked() {
	if g.term != nil {
		g.term.Close()
		g.term = nil
	}
	if g.sessionName != "" {
		exec.Command("tmux", "kill-session", "-t", g.sessionName).Run()
		g.sessionName = ""
	}
	g.currentInstanceTitle = ""
	g.worktreePath = ""
}

// SendKey forwards raw key bytes to the lazygit PTY via the embedded terminal.
// If the write fails (e.g. lazygit exited), it cleans up the dead session.
func (g *GitPane) SendKey(data []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.term == nil {
		return fmt.Errorf("git pane not running")
	}
	if err := g.term.SendKey(data); err != nil {
		g.killLocked()
		return fmt.Errorf("lazygit session ended")
	}
	return nil
}

// SetSize updates the PTY and tmux window dimensions.
func (g *GitPane) SetSize(width, height int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.width = width
	g.height = height
	if g.term != nil {
		g.term.Resize(width, height)
	}
}

// IsRunning returns true if a lazygit session is active.
func (g *GitPane) IsRunning() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.term != nil
}

// NeedsRespawn returns true if the current instance differs from what's running.
func (g *GitPane) NeedsRespawn(instanceTitle string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.currentInstanceTitle != instanceTitle
}

// Render returns the current terminal frame content.
func (g *GitPane) Render() (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.errorMsg != "" {
		return g.errorMsg, true
	}
	if g.term == nil {
		return "No instance selected or instance is paused.", true
	}
	return g.term.Render()
}

// String returns the current frame for display.
func (g *GitPane) String() string {
	content, _ := g.Render()
	return content
}

// tmuxSessionExists checks if a tmux session with the given name exists.
func tmuxSessionExists(name string) bool {
	return exec.Command("tmux", "has-session", fmt.Sprintf("-t=%s", name)).Run() == nil
}

// lazygitOverrideConfig writes a temporary lazygit config that disables quit
// keybindings and returns its path. The override disables 'q' (quit/back),
// 'Q' (quit without cd), and Ctrl+C (quit-alt1). Users navigate back with
// Escape instead.
func lazygitOverrideConfig() string {
	path := filepath.Join(os.TempDir(), "hivemind-lazygit-override.yml")
	content := []byte("keybinding:\n  universal:\n    quit: ''\n    quit-alt1: ''\n    quitWithoutChangingDirectory: ''\n")
	_ = os.WriteFile(path, content, 0644)
	return path
}

// lazygitConfigArg builds the --use-config-file value for lazygit.
// It preserves the user's existing config (if any) and appends our
// quit-disabling override so it takes precedence.
func lazygitConfigArg() string {
	override := lazygitOverrideConfig()

	// Find the user's lazygit config at the platform-standard location.
	configDir, err := os.UserConfigDir()
	if err != nil {
		return override
	}
	userConfig := filepath.Join(configDir, "lazygit", "config.yml")
	if _, err := os.Stat(userConfig); err != nil {
		return override
	}

	// User config first, override last (last wins on conflicts).
	return userConfig + "," + override
}

// sanitizeForTmux removes characters that tmux doesn't allow in session names.
func sanitizeForTmux(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' || c == ':' || c == ' ' {
			result = append(result, '_')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
