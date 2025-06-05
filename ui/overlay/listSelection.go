package overlay

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	teav1 "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItem is an interface that must be implemented by items in a ListSelection
type ListItem interface {
	list.Item
	GetValue() interface{}
	GetWidth() int
}

// ListSelection is a generic list selection component that can be used to select items from a list
type ListSelection struct {
	list          list.Model
	selectedItem  ListItem
	quitting      bool
	submitted     bool
	width, height int
	hasItems      bool
	title         string
	styles        ListSelectionStyles
}

// ListSelectionStyles contains all the styles for the list selection component
type ListSelectionStyles struct {
	TitleStyle      lipgloss.Style
	PaginationStyle lipgloss.Style
	HelpStyle       lipgloss.Style
	QuitTextStyle   lipgloss.Style
	BorderStyle     lipgloss.Style
}

// DefaultListSelectionStyles returns the default styles for the list selection component
func DefaultListSelectionStyles() ListSelectionStyles {
	return ListSelectionStyles{
		TitleStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true).MarginBottom(1),
		PaginationStyle: list.DefaultStyles().PaginationStyle,
		HelpStyle:       list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1),
		QuitTextStyle:   lipgloss.NewStyle().Margin(1, 0, 2, 4),
		BorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
	}
}

// NewListSelection creates a new list selection component
func NewListSelection(items []ListItem, title string, listHeight int) *ListSelection {
	hasItems := len(items) > 0

	// Convert ListItems to list.Items
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Calculate an appropriate width based on the list items
	itemWidth := 30 // Minimum default width
	for _, item := range items {
		itemWidth = max(itemWidth, item.GetWidth())
	}

	// Add some padding to the calculated width
	listWidth := itemWidth + 10 // Add padding for better readability

	// Cap the width at a reasonable maximum if needed
	maxWidth := 80
	if listWidth > maxWidth {
		listWidth = maxWidth
	}

	styles := DefaultListSelectionStyles()

	// Create a custom delegate with our styling
	delegate := list.NewDefaultDelegate()

	// Calculate appropriate height based on number of items
	calculatedHeight := len(listItems) + 2 // Add 2 for padding/margins
	if calculatedHeight > 100 {
		calculatedHeight = 100
	}
	if calculatedHeight < listHeight {
		calculatedHeight = listHeight
	}

	l := list.New(listItems, delegate, listWidth, calculatedHeight)
	// Don't set the title here, we'll render it manually with our custom style
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.PaginationStyle = styles.PaginationStyle
	l.Styles.HelpStyle = styles.HelpStyle

	return &ListSelection{
		list:     l,
		hasItems: hasItems,
		title:    title,
		styles:   styles,
	}
}

// SetSize sets the size of the list selection component
func (ls *ListSelection) SetSize(width, height int) {
	ls.width = width
	ls.height = height
	// Only use the provided width if it's smaller than the list's natural width
	// This preserves the calculated width for narrow lists
	if width < ls.list.Width() {
		ls.list.SetWidth(width)
	}
	// Only use the provided height if it's smaller than the list's natural height
	// This preserves the calculated height for short lists
	if height < ls.list.Height() {
		ls.list.SetHeight(height)
	}
}

// Init initializes the list selection component
func (ls *ListSelection) Init() tea.Cmd {
	return nil
}

// Update updates the list selection component based on messages
func (ls *ListSelection) Update(msg tea.Msg) (*ListSelection, tea.Cmd) {
	if !ls.hasItems {
		switch msg.(type) {
		case tea.KeyPressMsg:
			ls.quitting = true
			return ls, nil
		}
		return ls, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ls.list.SetWidth(msg.Width)
		return ls, nil

	case tea.KeyPressMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			// Ctrl+C no longer quits the list selection
			return ls, nil

		case "enter":
			i, ok := ls.list.SelectedItem().(ListItem)
			if ok {
				ls.selectedItem = i
				ls.submitted = true
			}
			return ls, nil

		case "esc":
			// Escape key no longer quits the list selection
			return ls, nil
		}
	}

	var cmdv1 teav1.Cmd
	ls.list, cmdv1 = ls.list.Update(msg)
	if cmdv1 != nil {
		return ls, func() tea.Msg { return cmdv1() }
	}
	return ls, nil
}

// View renders the list selection component
func (ls *ListSelection) View() string {
	return ls.Render()
}

// Render renders the list selection component
func (ls *ListSelection) Render() string {
	// Use the actual list width for the border, not the forced width
	borderStyle := ls.styles.BorderStyle.Width(ls.list.Width())

	var content string
	if !ls.hasItems {
		content += "No items available.\n"
		content += "Press any key to cancel..."
	} else if ls.quitting {
		content = "Selection cancelled."
	} else {
		// Always add the title with proper styling
		content = ls.styles.TitleStyle.Render(ls.title) + "\n"
		// Add the list view
		content += ls.list.View()
	}

	return borderStyle.Render(content)
}

// IsSubmitted returns whether the list selection has been submitted
func (ls *ListSelection) IsSubmitted() bool {
	return ls.submitted
}

// IsQuitting returns whether the list selection is quitting
func (ls *ListSelection) IsQuitting() bool {
	return ls.quitting
}

// GetSelectedItem returns the selected item
func (ls *ListSelection) GetSelectedItem() ListItem {
	return ls.selectedItem
}

// GetSelectedValue returns the value of the selected item
func (ls *ListSelection) GetSelectedValue() interface{} {
	if ls.selectedItem == nil {
		return nil
	}
	return ls.selectedItem.GetValue()
}

// SetFilteringEnabled sets whether filtering is enabled
func (ls *ListSelection) SetFilteringEnabled(enabled bool) {
	ls.list.SetFilteringEnabled(enabled)
}

// SetShowStatusBar sets whether the status bar is shown
func (ls *ListSelection) SetShowStatusBar(enabled bool) {
	ls.list.SetShowStatusBar(enabled)
}

// SetShowTitle sets whether the title is shown
func (ls *ListSelection) SetShowTitle(enabled bool) {
	ls.list.SetShowTitle(enabled)
}

// SetShowHelp sets whether the help is shown
func (ls *ListSelection) SetShowHelp(enabled bool) {
	ls.list.SetShowHelp(enabled)
}

// SetStyles sets the styles for the list selection component
func (ls *ListSelection) SetStyles(styles ListSelectionStyles) {
	ls.styles = styles
	ls.list.Styles.Title = styles.TitleStyle
	ls.list.Styles.PaginationStyle = styles.PaginationStyle
	ls.list.Styles.HelpStyle = styles.HelpStyle
}
