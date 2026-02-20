package ui

import (
	"github.com/charmbracelet/lipgloss"
)

const readyIcon = "● "
const pausedIcon = "\uf04c "

var readyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

var notifyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F0A868"))

var addedLinesStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#51bd73", Dark: "#51bd73"})

var removedLinesStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#de613e"))

var pausedStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"})

var titleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var listDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})

var evenRowTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.AdaptiveColor{Light: "#f5f5f5", Dark: "#1e1e1e"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})

var evenRowDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.AdaptiveColor{Light: "#f5f5f5", Dark: "#1e1e1e"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})

var selectedTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

var selectedDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#1a1a1a"})

// Active (unfocused) styles — muted version of selected
var activeTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#666666"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1a1a"})

var activeDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#666666"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#1a1a1a"})

var mainTitle = lipgloss.NewStyle().
	Background(lipgloss.Color("216")).
	Foreground(lipgloss.Color("230"))

var autoYesStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.Color("#1a1a1a"))

var resourceStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#777777"})

var activityStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#666666"})

var loadingStepStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#808080", Dark: "#808080"})

// Status filter tab styles
var activeFilterTab = lipgloss.NewStyle().
	Background(lipgloss.Color("216")).
	Foreground(lipgloss.Color("230")).
	Padding(0, 1)

var inactiveFilterTab = lipgloss.NewStyle().
	Background(lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#444444"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#999999"}).
	Padding(0, 1)

// StatusFilter determines which instances are shown based on their status.
type StatusFilter int

const (
	StatusFilterAll    StatusFilter = iota // Show all instances
	StatusFilterActive                     // Show only non-paused instances
)

// SortMode determines how instances are ordered.
type SortMode int

const (
	SortNewest SortMode = iota // Most recently updated first (default)
	SortOldest                 // Oldest first
	SortName                   // Alphabetical by title
	SortStatus                 // Grouped by status: running, ready, paused
)

var sortModeLabels = map[SortMode]string{
	SortNewest: "Newest",
	SortOldest: "Oldest",
	SortName:   "Name",
	SortStatus: "Status",
}

var sortDropdownStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7EC8D8")).
	Padding(0, 1)

var subAgentCountStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#7EC8D8", Dark: "#7EC8D8"})

var subAgentRowStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})

const branchIcon = "\uf126"

// roleIcons maps agent roles to Nerd Font icons.
var roleIcons = map[string]string{
	"coder":     "\uf121",  //  (code)
	"reviewer":  "\uf002",  //  (search/review)
	"architect": "\uf1b2",  //  (cube/architecture)
	"tester":    "\uf0c3",  //  (flask/test)
	"writer":    "\uf040",  //  (pencil/edit)
}

// roleIconFallback is used when role is set but not in the map.
const roleIconFallback = "\uf007" //  (user)

var roleIconStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#7EC8D8", Dark: "#7EC8D8"})

// Child instance styles for brain-spawned sub-agent rows.
var childTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#999999"})

var childDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Foreground(lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#777777"})

var childSelectedTitleStyle = lipgloss.NewStyle().
	Padding(1, 1, 0, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#555555"})

var childSelectedDescStyle = lipgloss.NewStyle().
	Padding(0, 1, 1, 1).
	Background(lipgloss.Color("#dde4f0")).
	Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#555555"})
