package project

import (
	"fmt"
	"testing"
)

func TestProjectHistory_AddProject(t *testing.T) {
	history := NewProjectHistory()
	
	// Test adding a project
	history.AddProject("/path/to/project1")
	
	if history.Count() != 1 {
		t.Errorf("Expected count 1, got %d", history.Count())
	}
	
	recent := history.GetRecentProjects()
	if len(recent) != 1 || recent[0] != "/path/to/project1" {
		t.Errorf("Expected [/path/to/project1], got %v", recent)
	}
}

func TestProjectHistory_AddDuplicate(t *testing.T) {
	history := NewProjectHistory()
	
	// Add same project twice
	history.AddProject("/path/to/project1")
	history.AddProject("/path/to/project2")
	history.AddProject("/path/to/project1") // This should move to front
	
	if history.Count() != 2 {
		t.Errorf("Expected count 2, got %d", history.Count())
	}
	
	recent := history.GetRecentProjects()
	if recent[0] != "/path/to/project1" {
		t.Errorf("Expected /path/to/project1 at front, got %s", recent[0])
	}
}

func TestProjectHistory_FilterProjects(t *testing.T) {
	history := NewProjectHistory()
	
	history.AddProject("/path/to/claude-squad")
	history.AddProject("/path/to/my-web-app")
	history.AddProject("/home/user/data-analysis")
	
	// Test filtering by partial match
	filtered := history.FilterProjects("claude")
	if len(filtered) != 1 || filtered[0] != "/path/to/claude-squad" {
		t.Errorf("Expected [/path/to/claude-squad], got %v", filtered)
	}
	
	// Test filtering by directory name
	filtered = history.FilterProjects("web")
	if len(filtered) != 1 || filtered[0] != "/path/to/my-web-app" {
		t.Errorf("Expected [/path/to/my-web-app], got %v", filtered)
	}
	
	// Test empty filter returns all
	filtered = history.FilterProjects("")
	if len(filtered) != 3 {
		t.Errorf("Expected 3 projects, got %d", len(filtered))
	}
}

func TestProjectHistory_ClearHistory(t *testing.T) {
	history := NewProjectHistory()
	
	// Add multiple projects
	for i := 0; i < 15; i++ {
		history.AddProject(fmt.Sprintf("/path/to/project%d", i))
	}
	
	// Clear keeping only 10
	history.ClearHistory(10)
	
	if history.Count() != 10 {
		t.Errorf("Expected count 10, got %d", history.Count())
	}
	
	// Check that the most recent ones are kept
	recent := history.GetRecentProjects()
	if recent[0] != "/path/to/project14" {
		t.Errorf("Expected most recent project first, got %s", recent[0])
	}
}

func TestProjectHistory_GetTopProjects(t *testing.T) {
	history := NewProjectHistory()
	
	// Add projects
	for i := 0; i < 15; i++ {
		history.AddProject(fmt.Sprintf("/path/to/project%d", i))
	}
	
	// Get top 5
	top := history.GetTopProjects(5)
	
	if len(top) != 5 {
		t.Errorf("Expected 5 projects, got %d", len(top))
	}
	
	// Should be most recent first
	if top[0] != "/path/to/project14" {
		t.Errorf("Expected most recent project first, got %s", top[0])
	}
}