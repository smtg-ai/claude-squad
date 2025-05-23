package app

import (
	"claude-squad/config"
	"claude-squad/ui/overlay"

	tea "github.com/charmbracelet/bubbletea"
)

const profileListHeight = 14

// ProfileItem implements the overlay.ListItem interface for profile selection
type ProfileItem struct {
	name    string
	profile config.Profile
}

// Title returns the display title for the profile item
func (i ProfileItem) Title() string { return i.name }

// Description returns the description for the profile item
func (i ProfileItem) Description() string { return i.profile.Command }

// FilterValue returns the value used for filtering
func (i ProfileItem) FilterValue() string { return i.name }

// GetValue returns the underlying value of the profile item
func (i ProfileItem) GetValue() interface{} { return i.profile }

// GetWidth returns the width of the profile item
func (i ProfileItem) GetWidth() int { return max(len(i.name), len(i.profile.Command)) }

// ProfileSelector is a component for selecting profiles
type ProfileSelector struct {
	listSelection *overlay.ListSelection
	selectedName  string
}

// NewProfileSelector creates a new profile selector
func NewProfileSelector(profiles map[string]config.Profile) *ProfileSelector {
	// Convert profiles to list items
	items := make([]overlay.ListItem, 0, len(profiles))
	for name, profile := range profiles {
		items = append(items, ProfileItem{
			name:    name,
			profile: profile,
		})
	}

	// Create a new list selection
	listSelection := overlay.NewListSelection(items, "Choose AI Assistant Profile", profileListHeight)

	return &ProfileSelector{
		listSelection: listSelection,
	}
}

// SetSize sets the size of the profile selector
func (ps *ProfileSelector) SetSize(width, height int) {
	ps.listSelection.SetSize(width, height)
}

// Init initializes the profile selector
func (ps *ProfileSelector) Init() tea.Cmd {
	return ps.listSelection.Init()
}

// Update updates the profile selector based on messages
func (ps *ProfileSelector) Update(msg tea.Msg) (*ProfileSelector, tea.Cmd) {
	listSelection, cmd := ps.listSelection.Update(msg)
	ps.listSelection = listSelection

	// If a selection was made, store the selected name
	if ps.listSelection.IsSubmitted() {
		if item, ok := ps.listSelection.GetSelectedItem().(ProfileItem); ok {
			ps.selectedName = item.name
		}
	}

	return ps, cmd
}

// View renders the profile selector
func (ps *ProfileSelector) View() string {
	return ps.Render()
}

// Render renders the profile selector
func (ps *ProfileSelector) Render() string {
	return ps.listSelection.Render()
}

// IsSubmitted returns whether a profile has been selected
func (ps *ProfileSelector) IsSubmitted() bool {
	return ps.listSelection.IsSubmitted()
}

// IsQuitting returns whether the profile selector is quitting
func (ps *ProfileSelector) IsQuitting() bool {
	return ps.listSelection.IsQuitting()
}

// GetSelectedProfile returns the selected profile
func (ps *ProfileSelector) GetSelectedProfile() config.Profile {
	value := ps.listSelection.GetSelectedValue()
	if value == nil {
		return config.Profile{}
	}
	return value.(config.Profile)
}

// GetSelectedName returns the name of the selected profile
func (ps *ProfileSelector) GetSelectedName() string {
	return ps.selectedName
}
