package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ByteMirror/hivemind/session"

	"github.com/creack/pty"
)

const terminalTmuxPrefix = "hivemind_terminal_"

// TerminalPane manages persistent shell tmux sessions per instance.
// Unlike GitPane (which kills and recreates on each spawn), TerminalPane keeps
// tmux sessions alive across tab switches so scrollback and processes are preserved.
type TerminalPane struct {
	mu              sync.Mutex
	sessions        map[string]string // instanceTitle -> tmux session name
	term            *session.EmbeddedTerminal
	currentInstance string
	width, height   int
	errorMsg        string
}

func NewTerminalPane() *TerminalPane {
	return &TerminalPane{
		sessions: make(map[string]string),
	}
}

// Attach creates a tmux session for the instance (if needed) and connects an
// EmbeddedTerminal to it. If already attached to a different instance, detaches first.
func (t *TerminalPane) Attach(worktreePath, instanceTitle string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Detach from current if switching instances
	if t.term != nil && t.currentInstance != instanceTitle {
		t.detachLocked()
	}
	if t.term != nil {
		return // already attached to this instance
	}

	t.errorMsg = ""
	t.currentInstance = instanceTitle

	sessionName, exists := t.sessions[instanceTitle]
	if !exists {
		sessionName = terminalTmuxPrefix + sanitizeForTmux(instanceTitle)
	}

	// Create tmux session if it doesn't exist yet
	if !tmuxSessionExists(sessionName) {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		createCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath, shell)
		createPtmx, err := pty.Start(createCmd)
		if err != nil {
			t.errorMsg = fmt.Sprintf("Failed to create terminal tmux session: %v", err)
			return
		}
		deadline := time.Now().Add(2 * time.Second)
		for !tmuxSessionExists(sessionName) {
			if time.Now().After(deadline) {
				createPtmx.Close()
				t.errorMsg = "Timed out waiting for terminal tmux session"
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		createPtmx.Close()
		// Hide tmux status bar so only the pane content is rendered.
		exec.Command("tmux", "set-option", "-t", sessionName, "status", "off").Run()
		t.sessions[instanceTitle] = sessionName
	}

	cols, rows := t.width, t.height
	if cols < 10 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}

	term, err := session.NewEmbeddedTerminal(sessionName, cols, rows)
	if err != nil {
		t.errorMsg = fmt.Sprintf("Failed to attach to terminal session: %v", err)
		return
	}
	t.term = term
}

// Detach closes the EmbeddedTerminal but keeps the tmux session alive.
func (t *TerminalPane) Detach() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.detachLocked()
}

func (t *TerminalPane) detachLocked() {
	if t.term != nil {
		t.term.Close()
		t.term = nil
	}
	t.currentInstance = ""
}

// Kill detaches and kills ALL tmux sessions (app shutdown).
func (t *TerminalPane) Kill() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.detachLocked()
	for title, name := range t.sessions {
		exec.Command("tmux", "kill-session", "-t", name).Run()
		delete(t.sessions, title)
	}
}

// KillSession kills a single instance's terminal tmux session.
func (t *TerminalPane) KillSession(instanceTitle string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentInstance == instanceTitle {
		t.detachLocked()
	}
	if name, ok := t.sessions[instanceTitle]; ok {
		exec.Command("tmux", "kill-session", "-t", name).Run()
		delete(t.sessions, instanceTitle)
	}
}

// SendKey forwards raw key bytes to the EmbeddedTerminal.
func (t *TerminalPane) SendKey(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil {
		return fmt.Errorf("terminal pane not attached")
	}
	return t.term.SendKey(data)
}

// SetSize updates the dimensions and resizes the active terminal if present.
func (t *TerminalPane) SetSize(width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.width = width
	t.height = height
	if t.term != nil {
		t.term.Resize(width, height)
	}
}

// IsAttached returns true if an EmbeddedTerminal is active.
func (t *TerminalPane) IsAttached() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.term != nil
}

// CurrentInstance returns the title of the currently attached instance.
func (t *TerminalPane) CurrentInstance() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentInstance
}

// Render returns the current terminal frame content.
func (t *TerminalPane) Render() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.errorMsg != "" {
		return t.errorMsg, true
	}
	if t.term == nil {
		return "No instance selected or instance is paused.", true
	}
	return t.term.Render()
}

// String returns the current frame for display.
func (t *TerminalPane) String() string {
	content, _ := t.Render()
	return content
}

// WaitForRender blocks until the embedded terminal has new content or the timeout elapses.
func (t *TerminalPane) WaitForRender(timeout time.Duration) {
	t.mu.Lock()
	term := t.term
	t.mu.Unlock()
	if term != nil {
		term.WaitForRender(timeout)
	} else {
		time.Sleep(timeout)
	}
}
