package overlay

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ToastType represents the type of toast notification
type ToastType int

const (
	// ToastTypeSuccess indicates a successful operation
	ToastTypeSuccess ToastType = iota
	// ToastTypeError indicates an error occurred
	ToastTypeError
	// ToastTypeInfo indicates informational message
	ToastTypeInfo
)

// Toast represents a temporary notification message
type Toast struct {
	// Message to display
	Message string
	// Type of toast (success, error, info)
	Type ToastType
	// When the toast was created
	CreatedAt time.Time
	// Duration to show the toast
	Duration time.Duration
}

// ToastOverlay manages toast notifications
type ToastOverlay struct {
	// Current toast being displayed (nil if none)
	currentToast *Toast
	// Width of the toast
	width int
}

// NewToastOverlay creates a new toast overlay manager
func NewToastOverlay() *ToastOverlay {
	return &ToastOverlay{
		width: 50,
	}
}

// Show displays a new toast notification
func (t *ToastOverlay) Show(message string, toastType ToastType, duration time.Duration) {
	t.currentToast = &Toast{
		Message:   message,
		Type:      toastType,
		CreatedAt: time.Now(),
		Duration:  duration,
	}
}

// ShowSuccess displays a success toast
func (t *ToastOverlay) ShowSuccess(message string) {
	t.Show(message, ToastTypeSuccess, 3*time.Second)
}

// ShowError displays an error toast
func (t *ToastOverlay) ShowError(message string) {
	t.Show(message, ToastTypeError, 5*time.Second)
}

// ShowInfo displays an info toast
func (t *ToastOverlay) ShowInfo(message string) {
	t.Show(message, ToastTypeInfo, 3*time.Second)
}

// Clear removes the current toast
func (t *ToastOverlay) Clear() {
	t.currentToast = nil
}

// IsVisible returns true if a toast is currently visible
func (t *ToastOverlay) IsVisible() bool {
	if t.currentToast == nil {
		return false
	}
	// Check if the toast has expired
	if time.Since(t.currentToast.CreatedAt) > t.currentToast.Duration {
		t.currentToast = nil
		return false
	}
	return true
}

// SetWidth sets the width of the toast
func (t *ToastOverlay) SetWidth(width int) {
	t.width = width
}

// Render renders the toast notification
func (t *ToastOverlay) Render() string {
	if !t.IsVisible() {
		return ""
	}

	// Choose colors based on toast type
	var borderColor lipgloss.Color
	var icon string
	switch t.currentToast.Type {
	case ToastTypeSuccess:
		borderColor = lipgloss.Color("#04B575") // Green
		icon = "[OK] "
	case ToastTypeError:
		borderColor = lipgloss.Color("#FF0000") // Red
		icon = "[!] "
	case ToastTypeInfo:
		borderColor = lipgloss.Color("#0088FF") // Blue
		icon = "[i] "
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 2).
		Width(t.width)

	content := icon + t.currentToast.Message

	return style.Render(content)
}

// GetCurrentToast returns the current toast (for testing)
func (t *ToastOverlay) GetCurrentToast() *Toast {
	return t.currentToast
}
