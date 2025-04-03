package ui

import (
	"claude-squad/cache"
	"claude-squad/session"

	"github.com/charmbracelet/lipgloss"
)

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	highlightColor    = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	inactiveTabStyle  = lipgloss.NewStyle().
				Border(inactiveTabBorder, true).
				BorderForeground(highlightColor).
				AlignHorizontal(lipgloss.Center)
	activeTabStyle = inactiveTabStyle.
			Border(activeTabBorder, true).
			AlignHorizontal(lipgloss.Center)
	windowStyle = lipgloss.NewStyle().
			BorderForeground(highlightColor).
			Border(lipgloss.NormalBorder(), false, true, true, true)
)

const (
	PreviewTab = iota
	DiffTab
)

type Tab struct {
	Name   string
	Render func(width int, height int) string
}

// TabbedWindow has tabs at the top of a pane which can be selected. The tabs
// take up one rune of height.
type TabbedWindow struct {
	tabs []string

	activeTab int
	height    int
	width     int

	preview *PreviewPane
	diff    *DiffPane

	// Cache for rendered content
	renderCache *cache.RenderCache
}

func NewTabbedWindow(preview *PreviewPane, diff *DiffPane) *TabbedWindow {
	return &TabbedWindow{
		tabs: []string{
			"Preview",
			"Diff",
		},
		preview:     preview,
		diff:        diff,
		renderCache: cache.NewRenderCache(),
	}
}

// AdjustPreviewWidth adjusts the width of the preview pane to be 90% of the provided width.
func AdjustPreviewWidth(width int) int {
	return int(float64(width) * 0.9)
}

func (w *TabbedWindow) SetSize(width, height int) {
	w.width = AdjustPreviewWidth(width)
	w.height = height

	// Calculate the content height by subtracting:
	// 1. Tab height (including border and padding)
	// 2. Window style vertical frame size
	// 3. Additional padding/spacing (2 for the newline and spacing)
	tabHeight := activeTabStyle.GetVerticalFrameSize() + 1
	contentHeight := height - tabHeight - windowStyle.GetVerticalFrameSize() - 2
	contentWidth := w.width - windowStyle.GetHorizontalFrameSize()

	w.preview.SetSize(contentWidth, contentHeight)
	w.diff.SetSize(contentWidth, contentHeight)

	// Invalidate cache when size changes
	w.renderCache.Invalidate()
}

func (w *TabbedWindow) GetPreviewSize() (width, height int) {
	return w.preview.width, w.preview.height
}

func (w *TabbedWindow) Toggle() {
	w.activeTab = (w.activeTab + 1) % len(w.tabs)
	// Invalidate cache when tab changes
	w.renderCache.Invalidate()
}

// UpdatePreview updates the content of the preview pane. instance may be nil.
func (w *TabbedWindow) UpdatePreview(instance *session.Instance) error {
	if w.activeTab != PreviewTab {
		return nil
	}
	err := w.preview.UpdateContent(instance)
	if err == nil {
		// Invalidate cache when content changes
		w.renderCache.Invalidate()
	}
	return err
}

func (w *TabbedWindow) UpdateDiff(instance *session.Instance) {
	if w.activeTab != DiffTab {
		return
	}
	w.diff.SetDiff(instance)
	// Invalidate cache when diff changes
	w.renderCache.Invalidate()
}

// Add these new methods for handling scroll events
func (w *TabbedWindow) ScrollUp() {
	if w.activeTab == 1 { // Diff tab
		w.diff.ScrollUp()
		// Invalidate cache when scrolling
		w.renderCache.Invalidate()
	}
}

func (w *TabbedWindow) ScrollDown() {
	if w.activeTab == 1 { // Diff tab
		w.diff.ScrollDown()
		// Invalidate cache when scrolling
		w.renderCache.Invalidate()
	}
}

// IsInDiffTab returns true if the diff tab is currently active
func (w *TabbedWindow) IsInDiffTab() bool {
	return w.activeTab == 1
}

func (w *TabbedWindow) String() string {
	if w.width == 0 || w.height == 0 {
		return ""
	}

	// Use the render cache to avoid recomputing the string when nothing has changed
	return w.renderCache.Get(w.width, w.height, func(width, height int) string {
		var renderedTabs []string

		tabWidth := width / len(w.tabs)
		lastTabWidth := width - tabWidth*(len(w.tabs)-1)
		tabHeight := activeTabStyle.GetVerticalFrameSize() + 1 // get padding border margin size + 1 for character height

		for i, t := range w.tabs {
			width := tabWidth
			if i == len(w.tabs)-1 {
				width = lastTabWidth
			}

			var style lipgloss.Style
			isFirst, isLast, isActive := i == 0, i == len(w.tabs)-1, i == w.activeTab
			if isActive {
				style = activeTabStyle
			} else {
				style = inactiveTabStyle
			}
			border, _, _, _, _ := style.GetBorder()
			if isFirst && isActive {
				border.BottomLeft = "│"
			} else if isFirst && !isActive {
				border.BottomLeft = "├"
			} else if isLast && isActive {
				border.BottomRight = "│"
			} else if isLast && !isActive {
				border.BottomRight = "┤"
			}
			style = style.Border(border)
			style = style.Width(width - 1)
			renderedTabs = append(renderedTabs, style.Render(t))
		}

		row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
		var content string
		if w.activeTab == 0 {
			content = w.preview.String()
		} else {
			content = w.diff.String()
		}
		window := windowStyle.Render(
			lipgloss.Place(
				w.width, w.height-2-windowStyle.GetVerticalFrameSize()-tabHeight,
				lipgloss.Left, lipgloss.Top, content))

		return lipgloss.JoinVertical(lipgloss.Left, "\n", row, window)
	})
}
