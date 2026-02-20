package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/ByteMirror/hivemind/session"

	"github.com/creack/pty"
)

const lazygitTmuxPrefix = "hivemind_lazygit_"

// GitPane manages a lazygit tmux session that persists across tab switches.
// When the user switches tabs (git → agent → git), the tmux session stays alive
// and only the EmbeddedTerminal PTY is reconnected — preserving lazygit's UI state.
// When switching instances, the old session is killed to avoid accumulating
// background lazygit processes (each runs file watchers and periodic git polls).
type GitPane struct {
	mu              sync.Mutex
	sessions        map[string]string // instanceTitle -> tmux session name
	term            *session.EmbeddedTerminal
	currentInstanceTitle string
	width, height   int
	errorMsg        string
}

// NewGitPane creates a new GitPane (no subprocess yet).
func NewGitPane() *GitPane {
	return &GitPane{
		sessions: make(map[string]string),
	}
}

// Attach creates a lazygit tmux session for the instance (if needed) and connects
// an EmbeddedTerminal to it. If already attached to a different instance, detaches first.
func (g *GitPane) Attach(worktreePath, instanceTitle string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Switching instances: kill the old lazygit session entirely (not just detach)
	// to avoid accumulating background processes with active file watchers.
	if g.currentInstanceTitle != "" && g.currentInstanceTitle != instanceTitle {
		oldTitle := g.currentInstanceTitle
		g.detachLocked()
		if name, ok := g.sessions[oldTitle]; ok {
			exec.Command("tmux", "kill-session", "-t", name).Run()
			delete(g.sessions, oldTitle)
		}
	}
	if g.term != nil {
		return // already attached to this instance
	}

	g.errorMsg = ""
	g.currentInstanceTitle = instanceTitle

	// Check that lazygit is installed.
	if _, err := exec.LookPath("lazygit"); err != nil {
		g.errorMsg = "lazygit is not installed.\n\nInstall it: https://github.com/jesseduffield/lazygit#installation"
		return
	}

	sessionName, exists := g.sessions[instanceTitle]
	if !exists {
		sessionName = lazygitTmuxPrefix + sanitizeForTmux(instanceTitle)
	}

	// Create tmux session if it doesn't exist yet
	if !tmuxSessionExists(sessionName) {
		configArg := lazygitConfigArg()
		createCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath,
			"lazygit", "--use-config-file="+configArg)
		createPtmx, err := pty.Start(createCmd)
		if err != nil {
			g.errorMsg = fmt.Sprintf("Failed to create tmux session: %v", err)
			return
		}
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
		// Hide tmux status bar so only the pane content is rendered.
		exec.Command("tmux", "set-option", "-t", sessionName, "status", "off").Run()
		g.sessions[instanceTitle] = sessionName
	}

	cols, rows := g.width, g.height
	if cols < 10 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}

	term, err := session.NewEmbeddedTerminal(sessionName, cols, rows)
	if err != nil {
		g.errorMsg = fmt.Sprintf("Failed to attach to lazygit session: %v", err)
		return
	}
	g.term = term
}

// Detach closes the EmbeddedTerminal but keeps the tmux session alive.
func (g *GitPane) Detach() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.detachLocked()
}

func (g *GitPane) detachLocked() {
	if g.term != nil {
		g.term.Close()
		g.term = nil
	}
	g.currentInstanceTitle = ""
}

// Kill detaches and kills ALL lazygit tmux sessions (app shutdown).
func (g *GitPane) Kill() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.detachLocked()
	for title, name := range g.sessions {
		exec.Command("tmux", "kill-session", "-t", name).Run()
		delete(g.sessions, title)
	}
}

// KillSession kills a single instance's lazygit tmux session.
func (g *GitPane) KillSession(instanceTitle string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.currentInstanceTitle == instanceTitle {
		g.detachLocked()
	}
	if name, ok := g.sessions[instanceTitle]; ok {
		exec.Command("tmux", "kill-session", "-t", name).Run()
		delete(g.sessions, instanceTitle)
	}
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
		// Lazygit exited — detach and remove this session
		name := g.sessions[g.currentInstanceTitle]
		g.detachLocked()
		if name != "" {
			exec.Command("tmux", "kill-session", "-t", name).Run()
		}
		// Remove from sessions map
		for title, n := range g.sessions {
			if n == name {
				delete(g.sessions, title)
				break
			}
		}
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

// WaitForRender blocks until the embedded terminal has new content or the timeout elapses.
func (g *GitPane) WaitForRender(timeout time.Duration) {
	g.mu.Lock()
	term := g.term
	g.mu.Unlock()
	if term != nil {
		term.WaitForRender(timeout)
	} else {
		time.Sleep(timeout)
	}
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
	_ = os.WriteFile(path, content, 0600)
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
