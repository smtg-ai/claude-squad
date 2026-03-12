package overlay

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInputOverlay represents a text input overlay with state management.
type TextInputOverlay struct {
	textarea     textarea.Model
	Title        string
	FocusIndex   int // 0 = text input, 1 = branch picker (if present), last = enter button
	Submitted    bool
	Canceled     bool
	OnSubmit     func()
	width        int
	height       int
	branchPicker *BranchPicker
	numStops     int // total number of focus stops
}

// NewTextInputOverlay creates a new text input overlay with the given title and initial value.
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	ti := newTextarea(initialValue)
	return &TextInputOverlay{
		textarea:   ti,
		Title:      title,
		FocusIndex: 0,
		Submitted:  false,
		Canceled:   false,
		numStops:   2, // textarea + enter button
	}
}

// NewTextInputOverlayWithBranchPicker creates a text input overlay that includes an
// empty branch picker. Results are populated asynchronously via SetBranchResults.
func NewTextInputOverlayWithBranchPicker(title string, initialValue string) *TextInputOverlay {
	ti := newTextarea(initialValue)
	bp := NewBranchPicker()
	return &TextInputOverlay{
		textarea:     ti,
		Title:        title,
		FocusIndex:   0,
		Submitted:    false,
		Canceled:     false,
		branchPicker: bp,
		numStops:     3, // textarea + branch picker + enter button
	}
}

func newTextarea(initialValue string) textarea.Model {
	ti := textarea.New()
	ti.SetValue(initialValue)
	ti.Focus()
	ti.ShowLineNumbers = false
	ti.Prompt = ""
	ti.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ti.CharLimit = 0
	ti.MaxHeight = 0
	return ti
}

func (t *TextInputOverlay) SetSize(width, height int) {
	t.textarea.SetHeight(height)
	t.width = width
	t.height = height
	if t.branchPicker != nil {
		t.branchPicker.SetWidth(width - 6)
	}
}

// Init initializes the text input overlay model
func (t *TextInputOverlay) Init() tea.Cmd {
	return textarea.Blink
}

// View renders the model's view
func (t *TextInputOverlay) View() string {
	return t.Render()
}

// isEnterButton returns true if the current focus is on the enter button.
func (t *TextInputOverlay) isEnterButton() bool {
	return t.FocusIndex == t.numStops-1
}

// isBranchPicker returns true if the current focus is on the branch picker.
func (t *TextInputOverlay) isBranchPicker() bool {
	return t.branchPicker != nil && t.FocusIndex == 1
}

// updateFocusState syncs the textarea/branchPicker focus/blur state.
func (t *TextInputOverlay) updateFocusState() {
	if t.FocusIndex == 0 {
		t.textarea.Focus()
	} else {
		t.textarea.Blur()
	}
	if t.branchPicker != nil {
		if t.isBranchPicker() {
			t.branchPicker.Focus()
		} else {
			t.branchPicker.Blur()
		}
	}
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns (shouldClose, branchFilterChanged).
func (t *TextInputOverlay) HandleKeyPress(msg tea.KeyMsg) (bool, bool) {
	switch msg.Type {
	case tea.KeyTab:
		t.FocusIndex = (t.FocusIndex + 1) % t.numStops
		t.updateFocusState()
		return false, false
	case tea.KeyShiftTab:
		t.FocusIndex = (t.FocusIndex - 1 + t.numStops) % t.numStops
		t.updateFocusState()
		return false, false
	case tea.KeyEsc:
		t.Canceled = true
		return true, false
	case tea.KeyEnter:
		if t.isEnterButton() {
			t.Submitted = true
			if t.OnSubmit != nil {
				t.OnSubmit()
			}
			return true, false
		}
		if t.isBranchPicker() {
			// Enter on branch picker = advance to enter button
			t.FocusIndex = t.numStops - 1
			t.updateFocusState()
			return false, false
		}
		// Send enter to textarea
		if t.FocusIndex == 0 {
			t.textarea, _ = t.textarea.Update(msg)
		}
		return false, false
	default:
		if t.FocusIndex == 0 {
			t.textarea, _ = t.textarea.Update(msg)
			return false, false
		}
		if t.isBranchPicker() {
			_, filterChanged := t.branchPicker.HandleKeyPress(msg)
			return false, filterChanged
		}
		return false, false
	}
}

// GetValue returns the current value of the text input.
func (t *TextInputOverlay) GetValue() string {
	return t.textarea.Value()
}

// GetSelectedBranch returns the selected branch name from the branch picker.
// Returns empty string if no branch picker is present or "New branch" is selected.
func (t *TextInputOverlay) GetSelectedBranch() string {
	if t.branchPicker == nil {
		return ""
	}
	return t.branchPicker.GetSelectedBranch()
}

// BranchFilterVersion returns the current filter version from the branch picker.
// Returns 0 if no branch picker is present.
func (t *TextInputOverlay) BranchFilterVersion() uint64 {
	if t.branchPicker == nil {
		return 0
	}
	return t.branchPicker.GetFilterVersion()
}

// BranchFilter returns the current filter text from the branch picker.
func (t *TextInputOverlay) BranchFilter() string {
	if t.branchPicker == nil {
		return ""
	}
	return t.branchPicker.GetFilter()
}

// SetBranchResults updates the branch picker with search results.
// version must match the picker's current filterVersion to be accepted.
func (t *TextInputOverlay) SetBranchResults(branches []string, version uint64) {
	if t.branchPicker == nil {
		return
	}
	t.branchPicker.SetResults(branches, version)
}

// IsSubmitted returns whether the form was submitted.
func (t *TextInputOverlay) IsSubmitted() bool {
	return t.Submitted
}

// IsCanceled returns whether the form was canceled.
func (t *TextInputOverlay) IsCanceled() bool {
	return t.Canceled
}

// SetOnSubmit sets a callback function for form submission.
func (t *TextInputOverlay) SetOnSubmit(onSubmit func()) {
	t.OnSubmit = onSubmit
}

// Render renders the text input overlay.
func (t *TextInputOverlay) Render() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginBottom(1)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	focusedButtonStyle := buttonStyle.
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("0"))

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	// Inner content width (accounting for padding and borders)
	innerWidth := t.width - 6
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Set textarea width to fit within the overlay
	t.textarea.SetWidth(innerWidth)

	// Build a horizontal divider line
	divider := dividerStyle.Render(strings.Repeat("─", innerWidth))

	// Build the view
	content := titleStyle.Render(t.Title) + "\n"
	content += t.textarea.View() + "\n\n"

	// Render branch picker if present, with dividers
	if t.branchPicker != nil {
		content += divider + "\n\n"
		content += t.branchPicker.Render() + "\n\n"
	}

	content += divider + "\n\n"

	// Render enter button with appropriate style
	enterButton := " Enter "
	if t.isEnterButton() {
		enterButton = focusedButtonStyle.Render(enterButton)
	} else {
		enterButton = buttonStyle.Render(enterButton)
	}
	content += enterButton

	return style.Render(content)
}
