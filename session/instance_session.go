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

// resourceLogEvery throttles process tree debug logging to once per 30 seconds.
var resourceLogEvery = log.NewEvery(30 * time.Second)

// processInfo holds data from a single row of ps output.
type processInfo struct {
	pid  int
	ppid int
	comm string
	cpu  float64
	rss  float64 // in KB
}

// processTree maps each PID to its children for efficient tree walking.
type processTree struct {
	procs    map[int]*processInfo
	children map[int][]int // ppid → child PIDs
}

// toolProcessNames are process names that indicate a sub-agent is running a tool.
var toolProcessNames = map[string]bool{
	"git": true, "rg": true, "grep": true, "find": true,
	"python": true, "python3": true, "Python": true,
	"node": true, "npx": true, "npm": true,
	"go": true, "cargo": true, "rustc": true,
	"cat": true, "sed": true, "awk": true,
	"curl": true, "wget": true,
	"make": true, "gcc": true, "g++": true,
	"ruby": true, "perl": true,
	"docker": true, "kubectl": true,
	"uv": true, "pip": true,
}

// buildProcessTree runs `ps` once and builds an in-memory process tree.
func buildProcessTree() (*processTree, error) {
	cmd := exec.Command("ps", "-A", "-o", "pid=,ppid=,ucomm=,%cpu=,rss=")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseProcessTree(string(output))
}

// parseProcessTree parses ps output into a processTree. Exported for testing.
func parseProcessTree(output string) (*processTree, error) {
	tree := &processTree{
		procs:    make(map[int]*processInfo),
		children: make(map[int][]int),
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		// comm may contain path separators; take only the basename
		comm := fields[2]
		if idx := strings.LastIndex(comm, "/"); idx >= 0 {
			comm = comm[idx+1:]
		}
		cpu, _ := strconv.ParseFloat(fields[3], 64)
		rss, _ := strconv.ParseFloat(fields[4], 64)

		p := &processInfo{pid: pid, ppid: ppid, comm: comm, cpu: cpu, rss: rss}
		tree.procs[pid] = p
		tree.children[ppid] = append(tree.children[ppid], pid)
	}

	return tree, nil
}

// descendants returns all descendant PIDs of the given root (excluding root itself).
func (t *processTree) descendants(rootPID int) []*processInfo {
	var result []*processInfo
	stack := t.children[rootPID]
	for len(stack) > 0 {
		pid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p, ok := t.procs[pid]; ok {
			result = append(result, p)
		}
		stack = append(stack, t.children[pid]...)
	}
	return result
}

func (i *Instance) Preview() (string, error) {
	if !i.started.Load() || i.Status == Paused {
		return "", nil
	}
	if i.tmuxDead.Load() {
		return "", nil
	}
	content, err := i.tmuxSession.CapturePaneContent()
	if err != nil {
		// Check if the tmux session has actually died (agent process exited).
		// If so, mark it dead to avoid repeated failed capture attempts.
		if !i.tmuxSession.DoesSessionExist() {
			i.tmuxDead.Store(true)
			return "", nil
		}
		return "", err
	}
	return content, nil
}

func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	if !i.started.Load() || i.tmuxDead.Load() {
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
	if !i.started.Load() || i.tmuxSession == nil {
		return nil, ErrInstanceNotStarted
	}
	sessionName := i.tmuxSession.GetSanitizedName()
	return NewEmbeddedTerminal(sessionName, cols, rows)
}

// TapEnter sends an enter key press to the tmux session if AutoYes is enabled.
func (i *Instance) TapEnter() {
	if !i.started.Load() || !i.AutoYes {
		return
	}
	if err := i.tmuxSession.TapEnter(); err != nil {
		log.ErrorLog.Printf("error tapping enter: %v", err)
	}
}

func (i *Instance) Attach() (chan struct{}, error) {
	if !i.started.Load() {
		return nil, ErrInstanceNotStarted
	}
	return i.tmuxSession.Attach()
}

func (i *Instance) SetPreviewSize(width, height int) error {
	if !i.started.Load() || i.Status == Paused || i.Status == Loading || i.tmuxDead.Load() {
		return nil
	}
	return i.tmuxSession.SetDetachedSize(width, height)
}

// GetGitWorktree returns the git worktree for the instance
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if !i.started.Load() {
		return nil, ErrInstanceNotStarted
	}
	return i.gitWorktree, nil
}

// SendPrompt sends a prompt to the tmux session
func (i *Instance) SendPrompt(prompt string) error {
	if !i.started.Load() {
		return ErrInstanceNotStarted
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
	if !i.started.Load() || i.Status == Paused || i.tmuxDead.Load() {
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
	if !i.started.Load() || i.Status == Paused {
		return fmt.Errorf("cannot send keys to instance that has not been started or is paused")
	}
	return i.tmuxSession.SendKeys(keys)
}

// UpdateDiffStats updates the git diff statistics for this instance
func (i *Instance) UpdateDiffStats() error {
	if !i.started.Load() {
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

// UpdateResourceUsage queries the process tree for CPU and memory usage,
// and detects sub-agent processes via tmux windows.
//
// Claude Code's tmux spawn backend creates sub-agents as additional windows
// (index > 0) in the same tmux session. We use tmux list-windows for detection
// and the process tree for resource metrics per window.
//
// Values are kept from the previous tick if the query fails, so the UI
// doesn't flicker.
func (i *Instance) UpdateResourceUsage() {
	if !i.started.Load() || i.tmuxSession == nil || i.tmuxDead.Load() {
		i.CPUPercent = 0
		i.MemMB = 0
		i.SubAgentCount = 0
		i.SubAgents = nil
		return
	}

	pid, err := i.tmuxSession.GetPanePID()
	if err != nil {
		return
	}

	tree, err := buildProcessTree()
	if err != nil {
		return
	}

	// Aggregate CPU and memory across the entire process tree of window 0.
	allDesc := tree.descendants(pid)
	var totalCPU, totalRSS float64
	for _, p := range allDesc {
		totalCPU += p.cpu
		totalRSS += p.rss
	}
	i.CPUPercent = totalCPU
	i.MemMB = totalRSS / 1024

	// Detect sub-agents via tmux windows. Claude Code's tmux backend spawns
	// each Task-tool sub-agent as a new window in the same session.
	// Window 0 is the main agent; windows 1+ are sub-agents.
	windows, err := i.tmuxSession.ListWindows()
	if err != nil {
		// Can't list windows — keep previous sub-agent state.
		if resourceLogEvery.ShouldLog() {
			log.ErrorLog.Printf("resource[%s]: failed to list tmux windows: %v", i.Title, err)
		}
		return
	}

	var subAgents []SubAgentInfo
	for _, w := range windows {
		if w.Index == 0 {
			continue // main agent window, skip
		}

		sa := SubAgentInfo{
			PID:      w.PanePID,
			Name:     "claude",
			Activity: "thinking",
		}

		// Aggregate resources from the sub-agent window's process tree.
		for _, p := range tree.descendants(w.PanePID) {
			sa.CPU += p.cpu
			sa.MemMB += p.rss / 1024
			if toolProcessNames[p.comm] {
				sa.Activity = "running " + p.comm
			}
		}

		// Also add sub-agent window resources to the instance total.
		i.CPUPercent += sa.CPU
		i.MemMB += sa.MemMB

		subAgents = append(subAgents, sa)
	}
	i.SubAgentCount = len(subAgents)
	i.SubAgents = subAgents

	if resourceLogEvery.ShouldLog() {
		log.InfoLog.Printf("resource[%s]: panePID=%d descendants=%d windows=%d cpu=%.1f%% mem=%.1fMB subAgents=%d",
			i.Title, pid, len(allDesc), len(windows), i.CPUPercent, i.MemMB, len(subAgents))
		for _, sa := range subAgents {
			log.InfoLog.Printf("resource[%s]: sub-agent: pid=%d activity=%s cpu=%.1f%% mem=%.1fMB",
				i.Title, sa.PID, sa.Activity, sa.CPU, sa.MemMB)
		}
	}
}

// GetDiffStats returns the current git diff statistics
func (i *Instance) GetDiffStats() *git.DiffStats {
	return i.diffStats
}
