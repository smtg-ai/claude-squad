package session

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ByteMirror/hivemind/log"
	"github.com/ByteMirror/hivemind/session/git"
	"github.com/ByteMirror/hivemind/session/tmux"
)

func (i *Instance) Preview() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContent()
}

func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started {
		return false, false
	}
	return i.tmuxSession.HasUpdated()
}

// GetPaneContent returns the current tmux pane content for activity parsing.
func (i *Instance) GetPaneContent() (string, error) {
	return i.Preview()
}

// NewEmbeddedTerminalForInstance creates an embedded terminal emulator connected
// to this instance's tmux PTY for zero-latency interactive focus mode.
func (i *Instance) NewEmbeddedTerminalForInstance(cols, rows int) (*EmbeddedTerminal, error) {
	if !i.started || i.tmuxSession == nil {
		return nil, fmt.Errorf("instance not started")
	}
	sessionName := i.tmuxSession.GetSanitizedName()
	return NewEmbeddedTerminal(sessionName, cols, rows)
}

// TapEnter sends an enter key press to the tmux session if AutoYes is enabled.
func (i *Instance) TapEnter() {
	if !i.started || !i.AutoYes {
		return
	}
	if err := i.tmuxSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

func (i *Instance) Attach() (chan struct{}, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot attach instance that has not been started")
	}
	return i.tmuxSession.Attach()
}

func (i *Instance) SetPreviewSize(width, height int) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot set preview size for instance that has not been started or " +
			"is paused")
	}
	return i.tmuxSession.SetDetachedSize(width, height)
}

// GetGitWorktree returns the git worktree for the instance
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if !i.started {
		return nil, fmt.Errorf("cannot get git worktree for instance that has not been started")
	}
	return i.gitWorktree, nil
}

// SendPrompt sends a prompt to the tmux session
func (i *Instance) SendPrompt(prompt string) error {
	if !i.started {
		return fmt.Errorf("instance not started")
	}
	if i.tmuxSession == nil {
		return fmt.Errorf("tmux session not initialized")
	}
	if err := i.tmuxSession.SendKeys(prompt); err != nil {
		return fmt.Errorf("error sending keys to tmux session: %w", err)
	}

	// Brief pause to prevent carriage return from being interpreted as newline
	time.Sleep(100 * time.Millisecond)
	if err := i.tmuxSession.TapEnter(); err != nil {
		return fmt.Errorf("error tapping enter: %w", err)
	}

	return nil
}

// PreviewFullHistory captures the entire tmux pane output including full scrollback history
func (i *Instance) PreviewFullHistory() (string, error) {
	if !i.started || i.Status == Paused {
		return "", nil
	}
	return i.tmuxSession.CapturePaneContentWithOptions("-", "-")
}

// SetTmuxSession sets the tmux session for testing purposes
func (i *Instance) SetTmuxSession(session *tmux.TmuxSession) {
	i.tmuxSession = session
}

// SendKeys sends keys to the tmux session
func (i *Instance) SendKeys(keys string) error {
	if !i.started || i.Status == Paused {
		return fmt.Errorf("cannot send keys to instance that has not been started or is paused")
	}
	return i.tmuxSession.SendKeys(keys)
}

// UpdateDiffStats updates the git diff statistics for this instance
func (i *Instance) UpdateDiffStats() error {
	if !i.started {
		i.diffStats = nil
		return nil
	}

	if i.Status == Paused {
		// Keep the previous diff stats if the instance is paused
		return nil
	}

	stats := i.gitWorktree.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			// Worktree is not fully set up yet, not an error
			i.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}

	i.diffStats = stats
	return nil
}

// UpdateResourceUsage queries the process tree for CPU and memory usage.
// Values are kept from the previous tick if the query fails, so the UI
// doesn't flicker between showing and hiding the resource bar.
func (i *Instance) UpdateResourceUsage() {
	if !i.started || i.tmuxSession == nil {
		i.CPUPercent = 0
		i.MemMB = 0
		return
	}

	pid, err := i.tmuxSession.GetPanePID()
	if err != nil {
		return
	}

	// The pane PID is the shell process (e.g., zsh). The actual program
	// (claude, aider, etc.) runs as a child. Find it with pgrep.
	targetPid := strconv.Itoa(pid)
	childCmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	if childOutput, err := childCmd.Output(); err == nil {
		if children := strings.Fields(strings.TrimSpace(string(childOutput))); len(children) > 0 {
			targetPid = children[0]
		}
	}

	// Get CPU and RSS for the target process
	psCmd := exec.Command("ps", "-o", "%cpu=,rss=", "-p", targetPid)
	output, err := psCmd.Output()
	if err != nil {
		return
	}

	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) < 2 {
		return
	}

	cpu, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return
	}
	rssKB, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return
	}
	i.CPUPercent = cpu
	i.MemMB = rssKB / 1024
}

// GetDiffStats returns the current git diff statistics
func (i *Instance) GetDiffStats() *git.DiffStats {
	return i.diffStats
}
