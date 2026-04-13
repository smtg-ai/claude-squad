package overlay

import (
	"claude-squad/cmd"
	"claude-squad/session/tmux"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ImportSource represents the type of session source to import from.
type ImportSource int

const (
	SourceTmux ImportSource = iota
	SourceClaudeCode
	SourceCodex
)

var importSources = []string{"tmux", "claude code", "codex"}

// SessionInfo holds information about a discoverable tmux session.
type SessionInfo struct {
	Name    string
	WorkDir string
	Program string
}

// ImportOverlay is a structured overlay for importing external tmux sessions.
// It contains a source picker, a session list, and an import button.
type ImportOverlay struct {
	// Source picker
	source ImportSource

	// Session list
	sessions []SessionInfo
	cursor   int

	// Focus management (0=source, 1=list, 2=button)
	focusIndex int
	numStops   int

	// State
	submitted bool
	canceled  bool

	// Dimensions
	width  int
	height int

	// Session discovery
	cmdExec  cmd.Executor
	existing map[string]bool // titles of already-imported sessions
}

// NewImportOverlay creates a new import overlay and discovers sessions for the
// initial source (tmux). existingTitles is the set of session names already
// managed by the app, which will be excluded from the list.
func NewImportOverlay(executor cmd.Executor, existingTitles []string) *ImportOverlay {
	existing := make(map[string]bool, len(existingTitles))
	for _, t := range existingTitles {
		existing[t] = true
	}
	o := &ImportOverlay{
		source:   SourceTmux,
		numStops: 3,
		width:    50,
		height:   20,
		cmdExec:  executor,
		existing: existing,
	}
	o.discoverSessions()
	return o
}

// HandleKeyPress processes a key press event. Returns true if the overlay
// should close.
func (o *ImportOverlay) HandleKeyPress(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyTab:
		o.focusIndex = (o.focusIndex + 1) % o.numStops
		return false
	case tea.KeyShiftTab:
		o.focusIndex = (o.focusIndex - 1 + o.numStops) % o.numStops
		return false
	case tea.KeyEsc:
		o.canceled = true
		return true
	case tea.KeyEnter:
		if o.isButton() {
			o.submitted = true
			return true
		}
		// Enter on list moves to button
		if o.isList() {
			o.focusIndex = o.numStops - 1
			return false
		}
		// Enter on source picker moves to list
		if o.isSourcePicker() {
			o.focusIndex = 1
			return false
		}
		return false
	case tea.KeyLeft:
		if o.isSourcePicker() {
			if o.source > 0 {
				o.source--
				o.discoverSessions()
			}
			return false
		}
	case tea.KeyRight:
		if o.isSourcePicker() {
			if int(o.source) < len(importSources)-1 {
				o.source++
				o.discoverSessions()
			}
			return false
		}
	case tea.KeyUp:
		if o.isList() && o.cursor > 0 {
			o.cursor--
		}
		return false
	case tea.KeyDown:
		if o.isList() && o.cursor < len(o.sessions)-1 {
			o.cursor++
		}
		return false
	}
	return false
}

// isSourcePicker returns true when the source picker row has focus.
func (o *ImportOverlay) isSourcePicker() bool {
	return o.focusIndex == 0
}

// isList returns true when the session list has focus.
func (o *ImportOverlay) isList() bool {
	return o.focusIndex == 1
}

// isButton returns true when the import button has focus.
func (o *ImportOverlay) isButton() bool {
	return o.focusIndex == o.numStops-1
}

// GetSelectedSession returns the currently highlighted session, or nil if no
// sessions are available.
func (o *ImportOverlay) GetSelectedSession() *SessionInfo {
	if len(o.sessions) == 0 || o.cursor < 0 || o.cursor >= len(o.sessions) {
		return nil
	}
	s := o.sessions[o.cursor]
	return &s
}

// IsSubmitted returns true if the user confirmed the import.
func (o *ImportOverlay) IsSubmitted() bool {
	return o.submitted
}

// IsCanceled returns true if the user dismissed the overlay.
func (o *ImportOverlay) IsCanceled() bool {
	return o.canceled
}

// SetSize sets the overlay dimensions.
func (o *ImportOverlay) SetSize(width, height int) {
	o.width = width
	o.height = height
}

// discoverSessions refreshes the session list for the current source.
func (o *ImportOverlay) discoverSessions() {
	o.cursor = 0
	o.sessions = nil

	allSessions, err := tmux.ListExternalSessionsWithInfo(o.cmdExec)
	if err != nil {
		return
	}

	switch o.source {
	case SourceTmux:
		for _, s := range allSessions {
			if o.existing[s.Name] {
				continue
			}
			o.sessions = append(o.sessions, SessionInfo{
				Name:    s.Name,
				WorkDir: s.WorkDir,
				Program: s.Program,
			})
		}
	case SourceClaudeCode:
		for _, s := range allSessions {
			if o.existing[s.Name] {
				continue
			}
			titleLower := strings.ToLower(s.PaneTitle)
			progLower := strings.ToLower(s.Program)
			if strings.Contains(titleLower, "claude") || strings.Contains(progLower, "claude") {
				o.sessions = append(o.sessions, SessionInfo{
					Name:    s.Name,
					WorkDir: s.WorkDir,
					Program: s.PaneTitle,
				})
			}
		}
	case SourceCodex:
		for _, s := range allSessions {
			if o.existing[s.Name] {
				continue
			}
			titleLower := strings.ToLower(s.PaneTitle)
			progLower := strings.ToLower(s.Program)
			if strings.Contains(titleLower, "codex") || strings.Contains(progLower, "codex") {
				o.sessions = append(o.sessions, SessionInfo{
					Name:    s.Name,
					WorkDir: s.WorkDir,
					Program: s.PaneTitle,
				})
			}
		}
	}
}

// Render draws the import overlay.
func (o *ImportOverlay) Render() string {
	innerWidth := o.width - 6
	if innerWidth < 1 {
		innerWidth = 1
	}

	divider := tiDividerStyle.Render(strings.Repeat("\u2500", innerWidth))

	var content string

	// -- Source picker --
	content += ioLabelStyle.Render("Source")
	if o.isSourcePicker() {
		content += ioDimStyle.Render("  \u2190/\u2192 to change")
	}
	content += "\n\n"

	for i, src := range importSources {
		if i == int(o.source) && o.isSourcePicker() {
			content += ioSelectedStyle.Render(" " + src + " ")
		} else if i == int(o.source) {
			content += " " + src + " "
		} else {
			content += ioDimStyle.Render(" " + src + " ")
		}
		if i < len(importSources)-1 {
			content += ioDimStyle.Render(" | ")
		}
	}
	content += "\n\n"
	content += divider + "\n\n"

	// -- Session list --
	content += ioLabelStyle.Render("Sessions")
	if o.isList() {
		content += ioDimStyle.Render("  \u2191/\u2193 to navigate")
	}
	content += "\n\n"

	if len(o.sessions) == 0 {
		content += ioDimStyle.Render("  No sessions found")
	} else {
		// Window around cursor, show up to maxVisible items
		maxVisible := 5
		start := 0
		if o.cursor >= maxVisible {
			start = o.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(o.sessions) {
			end = len(o.sessions)
		}

		for i := start; i < end; i++ {
			s := o.sessions[i]
			name := s.Name
			dir := shortenHome(s.WorkDir)
			if i == o.cursor && o.isList() {
				content += ioSelectedStyle.Render("> " + name)
				content += "\n"
				content += ioSelectedStyle.Render("  " + dir)
			} else if i == o.cursor {
				content += "  " + name + "\n"
				content += ioDimStyle.Render("  " + dir)
			} else {
				content += ioDimStyle.Render("  " + name)
				content += "\n"
				content += ioDimStyle.Render("  " + dir)
			}
			if i < end-1 {
				content += "\n"
			}
		}
	}

	content += "\n\n"
	content += divider + "\n\n"

	// -- Import button --
	buttonLabel := " Import "
	if o.isButton() {
		content += tiFocusedButtonStyle.Render(buttonLabel)
	} else {
		content += tiButtonStyle.Render(buttonLabel)
	}

	return ioStyle.Render(content)
}

// shortenHome replaces the user home prefix with ~.
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// Styles reused from the text input overlay where possible.
var (
	ioStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	ioLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true)

	ioSelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("0"))

	ioDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)
