package ui

import (
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/session/tmux"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var terminalPaneStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var terminalFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#808080", Dark: "#808080"})

// terminalSession holds a cached tmux session for a specific instance.
type terminalSession struct {
	tmuxSession  *tmux.TmuxSession
	worktreePath string
}

// TerminalPane manages shell tmux sessions in the worktree directory of selected instances.
// Sessions are cached per instance so switching between instances preserves terminal state.
type TerminalPane struct {
	mu            sync.Mutex
	width, height int
	sessions      map[string]*terminalSession // instanceTitle → session
	currentTitle  string                      // currently displayed instance
	content       string
	fallback      bool
	fallbackText  string

	isScrolling bool
	viewport    viewport.Model
}

func NewTerminalPane() *TerminalPane {
	return &TerminalPane{
		sessions: make(map[string]*terminalSession),
		viewport: viewport.New(0, 0),
	}
}

func (t *TerminalPane) SetSize(width, height int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.width = width
	t.height = height
	t.viewport.Width = width
	t.viewport.Height = height
	if s, ok := t.sessions[t.currentTitle]; ok && s.tmuxSession != nil {
		if err := s.tmuxSession.SetDetachedSize(width, height); err != nil {
			log.InfoLog.Printf("terminal pane: failed to set detached size: %v", err)
		}
	}
}

// setFallbackState sets the terminal pane to display a fallback message.
// Caller must hold t.mu.
func (t *TerminalPane) setFallbackState(message string) {
	t.fallback = true
	t.fallbackText = lipgloss.JoinVertical(lipgloss.Center, FallBackText, "", message)
	t.content = ""
}

// UpdateContent captures the tmux pane output for the terminal session.
func (t *TerminalPane) UpdateContent(instance *session.Instance) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if instance == nil {
		t.setFallbackState("Select an instance to open a terminal")
		return nil
	}
	if instance.Status == session.Paused {
		t.setFallbackState("Session is paused. Resume to use terminal.")
		return nil
	}
	if !instance.Started() {
		t.setFallbackState("Instance is not started yet.")
		return nil
	}

	// Skip content updates while in scroll mode
	if t.isScrolling {
		return nil
	}

	// Ensure we have a terminal session for this instance
	if err := t.ensureSessionLocked(instance); err != nil {
		return err
	}

	s, ok := t.sessions[t.currentTitle]
	if !ok || s.tmuxSession == nil || !s.tmuxSession.DoesSessionExist() {
		t.setFallbackState("Terminal session not available.")
		return nil
	}

	content, err := s.tmuxSession.CapturePaneContent()
	if err != nil {
		return fmt.Errorf("terminal pane: failed to capture content: %w", err)
	}

	t.fallback = false
	t.content = content
	return nil
}

// ensureSession creates or reuses a cached terminal tmux session for the given instance.
func (t *TerminalPane) ensureSession(instance *session.Instance) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ensureSessionLocked(instance)
}

// ensureSessionLocked is the lock-free implementation of ensureSession.
// Caller must hold t.mu.
func (t *TerminalPane) ensureSessionLocked(instance *session.Instance) error {
	if instance == nil || !instance.Started() || instance.Status == session.Paused {
		return nil
	}

	worktreePath := instance.GetWorktreePath()
	if worktreePath == "" {
		return nil
	}

	t.currentTitle = instance.Title

	// Check if we already have a cached session for this instance
	if s, ok := t.sessions[instance.Title]; ok {
		if s.tmuxSession != nil && s.tmuxSession.DoesSessionExist() {
			return nil
		}
		// Session died, remove stale entry and recreate below
		delete(t.sessions, instance.Title)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	termName := "term_" + instance.Title
	ts := tmux.NewTmuxSession(termName, shell)

	// Check if session already exists (e.g. from a previous run)
	if ts.DoesSessionExist() {
		if err := ts.Restore(); err != nil {
			// Session exists but can't restore, kill it and start fresh
			_ = ts.Close()
			ts = tmux.NewTmuxSession(termName, shell)
			if err := ts.Start(worktreePath); err != nil {
				return fmt.Errorf("terminal pane: failed to start session: %w", err)
			}
		}
	} else {
		if err := ts.Start(worktreePath); err != nil {
			return fmt.Errorf("terminal pane: failed to start session: %w", err)
		}
	}

	t.sessions[instance.Title] = &terminalSession{
		tmuxSession:  ts,
		worktreePath: worktreePath,
	}

	// Set the size
	if t.width > 0 && t.height > 0 {
		if err := ts.SetDetachedSize(t.width, t.height); err != nil {
			log.InfoLog.Printf("terminal pane: failed to set size: %v", err)
		}
	}

	return nil
}

// Attach attaches to the terminal tmux session (full-screen).
func (t *TerminalPane) Attach() (chan struct{}, error) {
	t.mu.Lock()
	s, ok := t.sessions[t.currentTitle]
	if !ok || s.tmuxSession == nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("no terminal session to attach to")
	}
	if !s.tmuxSession.DoesSessionExist() {
		t.mu.Unlock()
		return nil, fmt.Errorf("terminal session does not exist")
	}
	ts := s.tmuxSession
	t.mu.Unlock()
	return ts.Attach()
}

// Close kills all cached terminal tmux sessions and cleans up.
func (t *TerminalPane) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for title, s := range t.sessions {
		if s.tmuxSession != nil {
			if err := s.tmuxSession.Close(); err != nil {
				log.InfoLog.Printf("terminal pane: failed to close session for %s: %v", title, err)
			}
		}
	}
	t.sessions = make(map[string]*terminalSession)
	t.currentTitle = ""
	t.content = ""
	t.fallback = false
	t.fallbackText = ""
}

// CloseForInstance kills the cached terminal session for a specific instance.
func (t *TerminalPane) CloseForInstance(title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.sessions[title]; ok {
		if s.tmuxSession != nil {
			if err := s.tmuxSession.Close(); err != nil {
				log.InfoLog.Printf("terminal pane: failed to close session for %s: %v", title, err)
			}
		}
		delete(t.sessions, title)
	}
	if t.currentTitle == title {
		t.currentTitle = ""
		t.content = ""
		t.fallback = false
		t.fallbackText = ""
	}
}

func (t *TerminalPane) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	width := t.width
	height := t.height

	if width == 0 || height == 0 {
		return strings.Repeat("\n", height)
	}

	if t.isScrolling {
		return t.viewport.View()
	}

	fallback := t.fallback
	fallbackText := t.fallbackText
	content := t.content

	if fallback {
		// 3 = tab bar height (border + padding + text), 4 = window style frame (top/bottom border + padding)
		availableHeight := height - 3 - 4
		fallbackLines := len(strings.Split(fallbackText, "\n"))
		totalPadding := availableHeight - fallbackLines
		topPadding := 0
		bottomPadding := 0
		if totalPadding > 0 {
			topPadding = totalPadding / 2
			bottomPadding = totalPadding - topPadding
		}

		var lines []string
		if topPadding > 0 {
			lines = append(lines, strings.Repeat("\n", topPadding))
		}
		lines = append(lines, fallbackText)
		if bottomPadding > 0 {
			lines = append(lines, strings.Repeat("\n", bottomPadding))
		}

		return terminalPaneStyle.
			Width(width).
			Align(lipgloss.Center).
			Render(strings.Join(lines, ""))
	}

	// Normal mode: show captured content
	lines := strings.Split(content, "\n")

	if height > 0 {
		if len(lines) > height {
			lines = lines[len(lines)-height:]
		} else {
			padding := height - len(lines)
			lines = append(lines, make([]string, padding)...)
		}
	}

	contentStr := strings.Join(lines, "\n")
	return terminalPaneStyle.Width(width).Render(contentStr)
}

// enterScrollMode captures the full terminal history and enters scroll mode.
// Caller must hold t.mu.
func (t *TerminalPane) enterScrollMode() error {
	s, ok := t.sessions[t.currentTitle]
	if !ok || s.tmuxSession == nil || !s.tmuxSession.DoesSessionExist() {
		return nil
	}

	content, err := s.tmuxSession.CapturePaneContentWithOptions("-", "-")
	if err != nil {
		return fmt.Errorf("terminal pane: failed to capture full history: %w", err)
	}

	footer := terminalFooterStyle.Render("ESC to exit scroll mode")
	contentWithFooter := lipgloss.JoinVertical(lipgloss.Left, content, footer)
	t.viewport.SetContent(contentWithFooter)
	t.viewport.GotoBottom()
	t.isScrolling = true
	return nil
}

// ScrollUp enters scroll mode (if not already) and scrolls up.
func (t *TerminalPane) ScrollUp() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.isScrolling {
		return t.enterScrollMode()
	}
	t.viewport.LineUp(1)
	return nil
}

// ScrollDown enters scroll mode (if not already) and scrolls down.
func (t *TerminalPane) ScrollDown() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.isScrolling {
		return t.enterScrollMode()
	}
	t.viewport.LineDown(1)
	return nil
}

// ResetToNormalMode exits scroll mode and restores normal content display.
func (t *TerminalPane) ResetToNormalMode() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.isScrolling {
		return
	}
	t.isScrolling = false
	t.viewport.SetContent("")
	t.viewport.GotoTop()
}

// IsScrolling returns whether the terminal pane is in scroll mode.
func (t *TerminalPane) IsScrolling() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.isScrolling
}
