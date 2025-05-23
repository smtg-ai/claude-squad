package overlay

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItem is an interface that must be implemented by items in a ListSelection
type ListItem interface {
	list.Item
	GetValue() interface{}
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

	const defaultWidth = 40

	styles := DefaultListSelectionStyles()

	// Create a custom delegate with our styling
	delegate := list.NewDefaultDelegate()

	l := list.New(listItems, delegate, defaultWidth, listHeight)
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
	ls.list.SetWidth(width)
	ls.list.SetHeight(height)
}

// Init initializes the list selection component
func (ls *ListSelection) Init() tea.Cmd {
	return nil
}

// Update updates the list selection component based on messages
func (ls *ListSelection) Update(msg tea.Msg) (*ListSelection, tea.Cmd) {
	if !ls.hasItems {
		switch msg.(type) {
		case tea.KeyMsg:
			ls.quitting = true
			return ls, nil
		}
		return ls, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ls.list.SetWidth(msg.Width)
		return ls, nil

	case tea.KeyMsg:
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

	var cmd tea.Cmd
	ls.list, cmd = ls.list.Update(msg)
	return ls, cmd
}

// View renders the list selection component
func (ls *ListSelection) View() string {
	return ls.Render()
}

// Render renders the list selection component
func (ls *ListSelection) Render() string {
	// Get the true width of the list
	trueWidth := ls.list.Width()

	// Ensure minimum width and don't exceed maximum width
	if trueWidth < 30 {
		trueWidth = 30
	}
	if trueWidth > ls.width {
		trueWidth = ls.width
	}

	// Set the list width to match our calculated true width
	ls.list.SetWidth(trueWidth)

	borderStyle := ls.styles.BorderStyle.Width(trueWidth)

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
