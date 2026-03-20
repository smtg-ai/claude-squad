package overlay

import (
	"claude-squad/config"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tiStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	tiTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true).
			MarginBottom(1)

	tiButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	tiFocusedButtonStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("0"))

	tiDividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// TextInputOverlay represents a text input overlay with state management.
type TextInputOverlay struct {
	textarea      textarea.Model
	Title         string
	FocusIndex    int // index into focusable stops
	Submitted     bool
	Canceled      bool
	OnSubmit      func()
	width         int
	height        int
	profilePicker *ProfilePicker
	branchPicker  *BranchPicker
	numStops      int // total number of focus stops
}

// NewTextInputOverlay creates a new text input overlay with the given title and initial value.
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	ti := newTextarea(initialValue)
	return &TextInputOverlay{
		textarea: ti,
		Title:    title,
		numStops: 2, // textarea + enter button
	}
}

// NewTextInputOverlayWithBranchPicker creates a text input overlay that includes an
// empty branch picker. Results are populated asynchronously via SetBranchResults.
func NewTextInputOverlayWithBranchPicker(title string, initialValue string, profiles []config.Profile) *TextInputOverlay {
	ti := newTextarea(initialValue)
	bp := NewBranchPicker()

	var pp *ProfilePicker
	if len(profiles) > 0 {
		pp = NewProfilePicker(profiles)
	}

	numStops := 3 // textarea + branch picker + enter button
	if pp != nil && pp.HasMultiple() {
		numStops = 4 // profile picker + textarea + branch picker + enter button
	}

	overlay := &TextInputOverlay{
		textarea:      ti,
		Title:         title,
		profilePicker: pp,
		branchPicker:  bp,
		numStops:      numStops,
	}
	overlay.updateFocusState()
	return overlay
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
	if t.profilePicker != nil {
		t.profilePicker.SetWidth(width - 6)
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

// isProfilePicker returns true if the current focus is on the profile picker.
func (t *TextInputOverlay) isProfilePicker() bool {
	return t.profilePicker != nil && t.profilePicker.HasMultiple() && t.FocusIndex == 0
}

// isTextarea returns true if the current focus is on the textarea.
func (t *TextInputOverlay) isTextarea() bool {
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		return t.FocusIndex == 1
	}
	return t.FocusIndex == 0
}

// isEnterButton returns true if the current focus is on the enter button.
func (t *TextInputOverlay) isEnterButton() bool {
	return t.FocusIndex == t.numStops-1
}

// isBranchPicker returns true if the current focus is on the branch picker.
func (t *TextInputOverlay) isBranchPicker() bool {
	if t.branchPicker == nil {
		return false
	}
	if t.profilePicker != nil && t.profilePicker.HasMultiple() {
		return t.FocusIndex == 2
	}
	return t.FocusIndex == 1
}

// setFocusIndex sets the focus index and syncs focus state.
func (t *TextInputOverlay) setFocusIndex(i int) {
	t.FocusIndex = i
	t.updateFocusState()
}

// updateFocusState syncs the textarea/branchPicker/profilePicker focus/blur state.
func (t *TextInputOverlay) updateFocusState() {
	if t.isTextarea() {
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
	if t.profilePicker != nil {
		if t.isProfilePicker() {
			t.profilePicker.Focus()
		} else {
			t.profilePicker.Blur()
		}
	}
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns (shouldClose, branchFilterChanged).
func (t *TextInputOverlay) HandleKeyPress(msg tea.KeyMsg) (bool, bool) {
	switch msg.Type {
	case tea.KeyTab:
		t.setFocusIndex((t.FocusIndex + 1) % t.numStops)
		return false, false
	case tea.KeyShiftTab:
		t.setFocusIndex((t.FocusIndex - 1 + t.numStops) % t.numStops)
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
			t.setFocusIndex(t.numStops - 1)
			return false, false
		}
		if t.isProfilePicker() {
			// Enter on profile picker = advance to textarea
			t.setFocusIndex(t.FocusIndex + 1)
			return false, false
		}
		// Send enter to textarea
		if t.isTextarea() {
			t.textarea, _ = t.textarea.Update(msg)
		}
		return false, false
	default:
		if t.isTextarea() {
			t.textarea, _ = t.textarea.Update(msg)
			return false, false
		}
		if t.isProfilePicker() {
			if msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight {
				t.profilePicker.HandleKeyPress(msg)
			}
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

// GetSelectedProgram returns the program string from the selected profile.
// Returns empty string if no profile picker is present.
func (t *TextInputOverlay) GetSelectedProgram() string {
	if t.profilePicker == nil {
		return ""
	}
	return t.profilePicker.GetSelectedProfile().Program
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
	// Inner content width (accounting for padding and borders)
	innerWidth := t.width - 6
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Set textarea width to fit within the overlay
	t.textarea.SetWidth(innerWidth)

	// Build a horizontal divider line
	divider := tiDividerStyle.Render(strings.Repeat("─", innerWidth))

	// Build the view
	var content string

	// Render profile picker if present, above the prompt
	if t.profilePicker != nil {
		content += t.profilePicker.Render() + "\n\n"
		content += divider + "\n\n"
	}

	content += tiTitleStyle.Render(t.Title) + "\n"
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
		enterButton = tiFocusedButtonStyle.Render(enterButton)
	} else {
		enterButton = tiButtonStyle.Render(enterButton)
	}
	content += enterButton

	return tiStyle.Render(content)
}
