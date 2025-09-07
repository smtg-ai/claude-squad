package overlay

import (
	"claude-squad/config"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ToastType defines the type of toast notification
type ToastType int

const (
	ToastInfo ToastType = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// Toast represents a single toast notification
type Toast struct {
	Message     string
	Type        ToastType
	Duration    time.Duration
	Dismissible bool
	ID          string
	CreatedAt   time.Time
}

// ToastManager manages the queue and display of toast notifications
type ToastManager struct {
	toasts    []Toast
	maxToasts int
	styles    ToastStyles
	config    *config.Config
}

// ToastStyles defines the styling for different toast types
type ToastStyles struct {
	Info    lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Base    lipgloss.Style
}

// NewToastManager creates a new toast manager
func NewToastManager(cfg *config.Config) *ToastManager {
	return &ToastManager{
		toasts:    make([]Toast, 0),
		maxToasts: 5, // Maximum number of toasts to display
		styles:    createToastStyles(),
		config:    cfg,
	}
}

// createToastStyles creates the default toast styles
func createToastStyles() ToastStyles {
	baseStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(50).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF"))

	return ToastStyles{
		Base: baseStyle,
		Info: baseStyle.Copy().
			Background(lipgloss.Color("#3B82F6")).
			Foreground(lipgloss.Color("#FFFFFF")).
			BorderForeground(lipgloss.Color("#1E40AF")),
		Success: baseStyle.Copy().
			Background(lipgloss.Color("#10B981")).
			Foreground(lipgloss.Color("#FFFFFF")).
			BorderForeground(lipgloss.Color("#065F46")),
		Warning: baseStyle.Copy().
			Background(lipgloss.Color("#F59E0B")).
			Foreground(lipgloss.Color("#000000")).
			BorderForeground(lipgloss.Color("#D97706")),
		Error: baseStyle.Copy().
			Background(lipgloss.Color("#EF4444")).
			Foreground(lipgloss.Color("#FFFFFF")).
			BorderForeground(lipgloss.Color("#B91C1C")),
	}
}

// AddToast adds a new toast to the queue
func (tm *ToastManager) AddToast(toast Toast) {
	// Generate ID if not provided
	if toast.ID == "" {
		toast.ID = generateToastID()
	}
	toast.CreatedAt = time.Now()

	// Set default duration if not specified
	if toast.Duration == 0 {
		switch toast.Type {
		case ToastError:
			toast.Duration = time.Duration(tm.config.ToastTimeouts.Error) * time.Millisecond
		case ToastWarning:
			toast.Duration = time.Duration(tm.config.ToastTimeouts.Warning) * time.Millisecond
		case ToastSuccess:
			toast.Duration = time.Duration(tm.config.ToastTimeouts.Success) * time.Millisecond
		default:
			toast.Duration = time.Duration(tm.config.ToastTimeouts.Info) * time.Millisecond
		}
	}

	// Add to beginning of slice (newest first)
	tm.toasts = append([]Toast{toast}, tm.toasts...)

	// Remove oldest toasts if we exceed the maximum
	if len(tm.toasts) > tm.maxToasts {
		tm.toasts = tm.toasts[:tm.maxToasts]
	}
}

// RemoveToast removes a toast by ID
func (tm *ToastManager) RemoveToast(id string) {
	for i, toast := range tm.toasts {
		if toast.ID == id {
			tm.toasts = append(tm.toasts[:i], tm.toasts[i+1:]...)
			break
		}
	}
}

// UpdateToasts removes expired toasts and returns IDs of removed toasts
func (tm *ToastManager) UpdateToasts() []string {
	var removed []string
	var remaining []Toast

	now := time.Now()
	for _, toast := range tm.toasts {
		if now.Sub(toast.CreatedAt) >= toast.Duration {
			removed = append(removed, toast.ID)
		} else {
			remaining = append(remaining, toast)
		}
	}

	tm.toasts = remaining
	return removed
}

// GetToasts returns all current toasts
func (tm *ToastManager) GetToasts() []Toast {
	return tm.toasts
}

// HasToasts returns true if there are any toasts
func (tm *ToastManager) HasToasts() bool {
	return len(tm.toasts) > 0
}

// Render renders all toasts as a single string
func (tm *ToastManager) Render() string {
	if len(tm.toasts) == 0 {
		return ""
	}

	var rendered []string
	for _, toast := range tm.toasts {
		style := tm.getStyleForType(toast.Type)
		rendered = append(rendered, style.Render(toast.Message))
	}

	return lipgloss.JoinVertical(lipgloss.Right, rendered...)
}

// getStyleForType returns the appropriate style for the toast type
func (tm *ToastManager) getStyleForType(toastType ToastType) lipgloss.Style {
	switch toastType {
	case ToastInfo:
		return tm.styles.Info
	case ToastSuccess:
		return tm.styles.Success
	case ToastWarning:
		return tm.styles.Warning
	case ToastError:
		return tm.styles.Error
	default:
		return tm.styles.Info
	}
}

// generateToastID generates a unique ID for a toast
func generateToastID() string {
	return time.Now().Format("20060102150405.000")
}

// Convenience methods for adding specific toast types

// AddInfoToast adds an info toast
func (tm *ToastManager) AddInfoToast(message string) {
	tm.AddToast(Toast{
		Message:     message,
		Type:        ToastInfo,
		Dismissible: true,
	})
}

// AddSuccessToast adds a success toast
func (tm *ToastManager) AddSuccessToast(message string) {
	tm.AddToast(Toast{
		Message:     message,
		Type:        ToastSuccess,
		Dismissible: true,
	})
}

// AddWarningToast adds a warning toast
func (tm *ToastManager) AddWarningToast(message string) {
	tm.AddToast(Toast{
		Message:     message,
		Type:        ToastWarning,
		Dismissible: true,
	})
}

// AddErrorToast adds an error toast
func (tm *ToastManager) AddErrorToast(message string) {
	tm.AddToast(Toast{
		Message:     message,
		Type:        ToastError,
		Dismissible: true,
	})
}
