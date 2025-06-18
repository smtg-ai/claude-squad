package project

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProjectHistory manages the recent project paths with search and filtering capabilities
type ProjectHistory struct {
	RecentProjects []string  `json:"recent_projects"` // Project paths ordered by recency (most recent first)
	MaxHistory     int       `json:"max_history"`     // Configurable limit (default 50)
	LastCleared    time.Time `json:"last_cleared"`    // Track when history was last cleared
}

// NewProjectHistory creates a new project history with default settings
func NewProjectHistory() *ProjectHistory {
	return &ProjectHistory{
		RecentProjects: make([]string, 0),
		MaxHistory:     50, // Default max history size
		LastCleared:    time.Now(),
	}
}

// AddProject adds a project path to the history, moving it to the front if it already exists
func (ph *ProjectHistory) AddProject(projectPath string) {
	// Clean the path to ensure consistency
	cleanPath := filepath.Clean(projectPath)

	// Remove if already exists (to avoid duplicates)
	ph.removeProject(cleanPath)

	// Add to front
	ph.RecentProjects = append([]string{cleanPath}, ph.RecentProjects...)

	// Enforce max history limit
	if len(ph.RecentProjects) > ph.MaxHistory {
		ph.RecentProjects = ph.RecentProjects[:ph.MaxHistory]
	}
}

// removeProject removes a project path from the history (internal helper)
func (ph *ProjectHistory) removeProject(projectPath string) {
	for i, path := range ph.RecentProjects {
		if path == projectPath {
			ph.RecentProjects = append(ph.RecentProjects[:i], ph.RecentProjects[i+1:]...)
			break
		}
	}
}

// GetRecentProjects returns a copy of recent projects (most recent first)
func (ph *ProjectHistory) GetRecentProjects() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(ph.RecentProjects))
	copy(result, ph.RecentProjects)
	return result
}

// GetTopProjects returns the first N recent projects (for quick select 0-9)
func (ph *ProjectHistory) GetTopProjects(count int) []string {
	if count > len(ph.RecentProjects) {
		count = len(ph.RecentProjects)
	}
	if count <= 0 {
		return []string{}
	}

	result := make([]string, count)
	copy(result, ph.RecentProjects[:count])
	return result
}

// FilterProjects returns projects that match the search query (case-insensitive)
func (ph *ProjectHistory) FilterProjects(query string) []string {
	if query == "" {
		return ph.GetRecentProjects()
	}

	var filtered []string
	queryLower := strings.ToLower(query)

	for _, path := range ph.RecentProjects {
		// Check both full path and just the directory name
		pathLower := strings.ToLower(path)
		dirName := strings.ToLower(filepath.Base(path))

		if strings.Contains(pathLower, queryLower) || strings.Contains(dirName, queryLower) {
			filtered = append(filtered, path)
		}
	}

	return filtered
}

// ClearHistory removes all but the last N projects from history
func (ph *ProjectHistory) ClearHistory(keepLast int) {
	if keepLast < 0 {
		keepLast = 0
	}

	if keepLast >= len(ph.RecentProjects) {
		return // Nothing to clear
	}

	// Keep only the most recent N projects
	ph.RecentProjects = ph.RecentProjects[:keepLast]
	ph.LastCleared = time.Now()
}

// Count returns the total number of projects in history
func (ph *ProjectHistory) Count() int {
	return len(ph.RecentProjects)
}

// IsEmpty returns true if there are no projects in history
func (ph *ProjectHistory) IsEmpty() bool {
	return len(ph.RecentProjects) == 0
}

// RemoveNonExistentPaths removes paths that no longer exist on the filesystem
func (ph *ProjectHistory) RemoveNonExistentPaths() int {
	var validPaths []string
	removedCount := 0

	for _, path := range ph.RecentProjects {
		if _, err := os.Stat(path); err == nil {
			validPaths = append(validPaths, path)
		} else {
			removedCount++
		}
	}

	ph.RecentProjects = validPaths
	return removedCount
}
