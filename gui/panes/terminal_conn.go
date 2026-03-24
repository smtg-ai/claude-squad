package panes

import (
	"claude-squad/log"
	"claude-squad/session"
	"os"
	"sync"

	terminal "github.com/fyne-io/terminal"
	"github.com/creack/pty"
)

// TerminalConnection manages the link between a fyne-io/terminal widget
// and a tmux session PTY.
type TerminalConnection struct {
	mu       sync.Mutex
	instance *session.Instance
	term     *terminal.Terminal
	ptmx     *os.File
	listener chan terminal.Config
	closed   bool
}

// NewTerminalConnection creates a new connection but does not start it.
func NewTerminalConnection() *TerminalConnection {
	return &TerminalConnection{
		listener: make(chan terminal.Config, 1),
	}
}

// Terminal returns the fyne terminal widget for embedding in the UI.
func (tc *TerminalConnection) Terminal() *terminal.Terminal {
	return tc.term
}

// Connect attaches the terminal widget to the given instance's tmux session.
// It disconnects any existing connection first. A new terminal widget is
// created each time to avoid races with the previous RunWithConnection goroutine.
func (tc *TerminalConnection) Connect(inst *session.Instance) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Disconnect existing connection if any
	tc.disconnectLocked()

	tmuxSession := inst.GetTmuxSession()
	if tmuxSession == nil {
		return nil
	}

	ptmx, err := tmuxSession.ConnectPTY()
	if err != nil {
		return err
	}

	// Create a fresh terminal widget to avoid races with the old RunWithConnection goroutine
	tc.term = terminal.New()
	tc.instance = inst
	tc.ptmx = ptmx
	tc.closed = false

	// Start resize listener
	tc.term.AddListener(tc.listener)
	go tc.resizeLoop()

	// Connect terminal widget to PTY
	go func() {
		if err := tc.term.RunWithConnection(ptmx, ptmx); err != nil {
			log.InfoLog.Printf("terminal connection ended: %v", err)
		}
	}()

	return nil
}

// Disconnect closes the PTY connection and cleans up.
func (tc *TerminalConnection) Disconnect() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.disconnectLocked()
}

func (tc *TerminalConnection) disconnectLocked() {
	if tc.ptmx == nil || tc.closed {
		return
	}
	tc.closed = true

	// Remove resize listener to terminate resizeLoop goroutine
	tc.term.RemoveListener(tc.listener)
	tc.listener = make(chan terminal.Config, 1) // fresh channel for next Connect

	if tc.instance != nil {
		tmuxSession := tc.instance.GetTmuxSession()
		if tmuxSession != nil {
			if err := tmuxSession.DisconnectPTY(tc.ptmx); err != nil {
				log.InfoLog.Printf("disconnect PTY error: %v", err)
			}
		}
	}
	tc.ptmx = nil
	tc.instance = nil
}

// Instance returns the currently connected instance, or nil.
func (tc *TerminalConnection) Instance() *session.Instance {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.instance
}

// resizeLoop propagates terminal widget size changes to the tmux PTY.
func (tc *TerminalConnection) resizeLoop() {
	var lastRows, lastCols uint
	for config := range tc.listener {
		if config.Rows == lastRows && config.Columns == lastCols {
			continue
		}
		lastRows, lastCols = config.Rows, config.Columns

		tc.mu.Lock()
		if tc.ptmx != nil && !tc.closed {
			if err := pty.Setsize(tc.ptmx, &pty.Winsize{
				Rows: uint16(config.Rows),
				Cols: uint16(config.Columns),
			}); err != nil {
				log.InfoLog.Printf("failed to resize PTY: %v", err)
			}
		}
		tc.mu.Unlock()
	}
}
