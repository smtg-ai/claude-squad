package overlay

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInputOverlay represents a text input overlay with state management.
type TextInputOverlay struct {
	Value        string
	Title        string
	Multiline    bool
	FocusIndex   int // 0 for text input, 1 for enter button.
	Submitted    bool
	Canceled     bool
	OnSubmit     func()
	CursorPos    int // Track cursor position within the text.
	MaxLineWidth int // Maximum width for text wrapping.
}

// NewTextInputOverlay creates a new text input overlay with the given title and initial value.
func NewTextInputOverlay(title string, initialValue string) *TextInputOverlay {
	return &TextInputOverlay{
		Value:        initialValue,
		Title:        title,
		Multiline:    true,
		FocusIndex:   0,
		Submitted:    false,
		Canceled:     false,
		CursorPos:    len(initialValue),
		MaxLineWidth: 0,
	}
}

// Init initializes the text input overlay model
func (t *TextInputOverlay) Init() tea.Cmd {
	return nil
}

// View renders the model's view
func (t *TextInputOverlay) View() string {
	// Default to full width and height
	return t.Render(20, 80)
}

// HandleKeyPress processes a key press and updates the state accordingly.
// Returns true if the overlay should be closed.
func (t *TextInputOverlay) HandleKeyPress(key tea.KeyMsg) bool {
	switch key.Type {
	case tea.KeyLeft:
		if t.FocusIndex == 0 && t.CursorPos > 0 {
			t.CursorPos--
		}
		return false
	case tea.KeyRight:
		// Allow moving the cursor only if it is before the end of the text.
		if t.FocusIndex == 0 && t.CursorPos < len(t.Value) {
			t.CursorPos++
		}
		return false
	case tea.KeyTab:
		// Toggle focus between input and enter button.
		t.FocusIndex = (t.FocusIndex + 1) % 2
		return false
	case tea.KeyShiftTab:
		// Toggle focus in reverse.
		t.FocusIndex = (t.FocusIndex + 1) % 2
		return false
	case tea.KeyEnter:
		if t.FocusIndex == 1 {
			// Enter button is focused, so submit.
			t.Submitted = true
			if t.OnSubmit != nil {
				t.OnSubmit()
			}
			return true
		}
		// Input is focused; in multiline mode, add a new line.
		if t.FocusIndex == 0 && t.Multiline {
			beforeCursor := t.Value[:t.CursorPos]
			afterCursor := t.Value[t.CursorPos:]
			t.Value = beforeCursor + "\n" + afterCursor
			t.CursorPos++ // Move cursor past the newline.
		}
		return false
	case tea.KeyEsc:
		t.Canceled = true
		return true
	case tea.KeySpace:
		if t.FocusIndex == 0 {
			beforeCursor := t.Value[:t.CursorPos]
			afterCursor := t.Value[t.CursorPos:]
			t.Value = beforeCursor + " " + afterCursor
			t.CursorPos++
		}
		return false
	case tea.KeyRunes:
		// Handle character input only when input is focused.
		if t.FocusIndex == 0 && len(key.Runes) > 0 {
			t.Value = t.Value[:t.CursorPos] + string(key.Runes) + t.Value[t.CursorPos:]
			t.CursorPos += len(key.Runes)
		}
		return false
	case tea.KeyBackspace:
		// Handle backspace only when input is focused.
		if t.FocusIndex == 0 && t.CursorPos > 0 {
			t.Value = t.Value[:t.CursorPos-1] + t.Value[t.CursorPos:]
			t.CursorPos--
		}
		return false
	default:
		return false
	}
}

// GetValue returns the current value of the text input.
func (t *TextInputOverlay) GetValue() string {
	return t.Value
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
func (t *TextInputOverlay) Render(height, width int, opts ...WhitespaceOption) string {
	// Style the title.
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		MarginBottom(1)

	// Style the input text.
	inputStyle := lipgloss.NewStyle()

	// Style for the input box.
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	// Style for the enter button.
	enterButtonStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 2)

	// Calculate available width for text.
	availableWidth := width - 10 // Account for borders and padding.
	if t.MaxLineWidth == 0 || t.MaxLineWidth > availableWidth {
		t.MaxLineWidth = availableWidth
	}

	// Highlight the focused element.
	if t.FocusIndex == 0 {
		inputBoxStyle = inputBoxStyle.BorderForeground(lipgloss.Color("#7D56F4"))
		enterButtonStyle = enterButtonStyle.BorderForeground(lipgloss.Color("#AAAAAA"))
	} else {
		inputBoxStyle = inputBoxStyle.BorderForeground(lipgloss.Color("#AAAAAA"))
		enterButtonStyle = enterButtonStyle.BorderForeground(lipgloss.Color("#7D56F4"))
	}

	var renderedText string
	if !t.Multiline {
		if t.FocusIndex == 0 {
			if t.CursorPos < len(t.Value) {
				// Overlay the character at the cursor position.
				char := t.Value[t.CursorPos : t.CursorPos+1]
				renderedText = inputStyle.Render(t.Value[:t.CursorPos]) +
					lipgloss.NewStyle().
						Background(lipgloss.Color("#7D56F4")).
						Foreground(lipgloss.Color("#FFFFFF")).
						Render(char) +
					inputStyle.Render(t.Value[t.CursorPos+1:])
			} else {
				// At the end: render a styled space with the same width as a normal character.
				renderedText = inputStyle.Render(t.Value) +
					lipgloss.NewStyle().
						Background(lipgloss.Color("#7D56F4")).
						Foreground(lipgloss.Color("#FFFFFF")).
						Render(" ")
			}
		} else {
			renderedText = inputStyle.Render(t.Value)
		}
	} else {
		// For multiline input:
		// Split text into lines.
		lines := strings.Split(t.Value, "\n")
		pos := t.CursorPos
		cursorRow := 0
		cursorCol := pos
		for i, line := range lines {
			if pos > len(line) {
				// Account for the newline.
				pos -= len(line) + 1
			} else {
				cursorRow = i
				cursorCol = pos
				break
			}
		}
		var wrappedLines []string
		for i, line := range lines {
			wrapped := wrapText(line, t.MaxLineWidth)
			// If this is the line that should display the cursor...
			if i == cursorRow {
				col := cursorCol
				for j, subline := range wrapped {
					if col > len(subline) {
						col -= len(subline)
					} else {
						if col < len(subline) {
							// Overlay the character under the cursor.
							char := subline[col : col+1]
							wrapped[j] = inputStyle.Render(subline[:col]) +
								lipgloss.NewStyle().
									Background(lipgloss.Color("#7D56F4")).
									Foreground(lipgloss.Color("#FFFFFF")).
									Render(char) +
								inputStyle.Render(subline[col+1:])
						} else {
							// Cursor at the very end: render a styled space.
							wrapped[j] = inputStyle.Render(subline) +
								lipgloss.NewStyle().
									Background(lipgloss.Color("#7D56F4")).
									Foreground(lipgloss.Color("#FFFFFF")).
									Render(" ")
						}
						break
					}
				}
			}
			// Render each wrapped subline with the input style.
			for _, wl := range wrapped {
				wrappedLines = append(wrappedLines, inputStyle.Render(wl))
			}
		}
		renderedText = strings.Join(wrappedLines, "\n")
	}

	// Create the input box with the title.
	inputBoxContent := titleStyle.Render(t.Title) + "\n" + renderedText
	inputBox := inputBoxStyle.
		Width(width - 6).
		Height(height - 8). // Leave room for the enter button.
		Render(inputBoxContent)

	// Create the enter button.
	enterButton := enterButtonStyle.Render("Enter")
	inputBoxWidth := lipgloss.Width(inputBox)
	enterButtonWidth := lipgloss.Width(enterButton)
	enterButtonPadding := inputBoxWidth - enterButtonWidth

	// Combine the input box and enter button (positioning the button at the bottom right).
	content := inputBox + "\n" +
		lipgloss.NewStyle().
			PaddingLeft(enterButtonPadding).
			Render(enterButton)

	return PlaceOverlay(0, 0, content, strings.Repeat("\n", height), true, true, opts...)
}

// wrapText wraps a given text into lines with a maximum width.
func wrapText(text string, maxWidth int) []string {
	if len(text) == 0 {
		return []string{""}
	}

	var lines []string
	var currentLine string
	var currentWidth int

	// Process each character individually to preserve all whitespace
	for _, char := range text {
		if char == '\n' {
			// Handle explicit line breaks
			lines = append(lines, currentLine)
			currentLine = ""
			currentWidth = 0
			continue
		}

		// If adding this char would exceed maxWidth, start a new line
		if currentWidth >= maxWidth {
			lines = append(lines, currentLine)
			currentLine = string(char)
			currentWidth = 1
		} else {
			currentLine += string(char)
			currentWidth++
		}
	}

	// Add the last line if it's not empty
	if currentLine != "" || len(lines) == 0 {
		lines = append(lines, currentLine)
	}

	return lines
}
