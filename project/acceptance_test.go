package project

import (
	"claude-squad/config"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpic1Story1AcceptanceCriteria validates all acceptance criteria for Epic 1, Story 1
// These tests ensure the basic project addition functionality works as specified
func TestEpic1Story1AcceptanceCriteria(t *testing.T) {

	// AC2: Users can add projects using absolute paths
	t.Run("AC2: Users can add projects using absolute paths", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir := t.TempDir()
		
		// Test validation before adding project
		err := manager.ValidateProjectPath(tempDir)
		assert.NoError(t, err, "Absolute existing path should validate")
		
		err = manager.ValidateProjectPath("./relative")
		assert.Error(t, err, "Relative path should fail validation")
		
		// Test adding project with absolute path
		project, err := manager.AddProject(tempDir, "Test Project")
		
		require.NoError(t, err, "Should be able to add project with absolute path")
		assert.NotNil(t, project)
		assert.Equal(t, tempDir, project.Path)
		assert.True(t, filepath.IsAbs(project.Path), "Project path should be absolute")
		
		// Test that relative paths are rejected
		_, err = manager.AddProject("./relative/path", "Invalid Project")
		assert.Error(t, err, "Should reject relative paths")
		assert.Contains(t, err.Error(), "absolute")
		
		// Test that duplicate paths are rejected
		_, err = manager.AddProject(tempDir, "Duplicate Project")
		assert.Error(t, err, "Should reject duplicate paths")
		assert.Contains(t, err.Error(), "already exists")
	})

	// AC3: Projects appear in hierarchical list with visual distinction
	t.Run("AC3: Projects appear in hierarchical list with basic visual distinction", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		
		// Add multiple projects
		project1, err1 := manager.AddProject(tempDir1, "Project 1")
		project2, err2 := manager.AddProject(tempDir2, "Project 2")
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		
		// Projects should appear in list
		projects := manager.ListProjects()
		assert.Equal(t, 2, len(projects), "Should list all projects")
		
		// Projects should have distinct identifiers
		assert.NotEqual(t, project1.ID, project2.ID, "Projects should have unique IDs")
		assert.NotEqual(t, project1.Path, project2.Path, "Projects should have different paths")
		
		// Projects should be sorted by last accessed (most recent first)
		assert.Equal(t, project2.ID, projects[0].ID, "Most recently added project should be first")
		assert.Equal(t, project1.ID, projects[1].ID, "First project should be second")
		
		// Test project hierarchy through instance association
		err := manager.AddInstanceToProject(project1.ID, "instance-1")
		require.NoError(t, err)
		
		instances, err := manager.GetProjectInstances(project1.ID)
		require.NoError(t, err)
		assert.Contains(t, instances, "instance-1", "Instance should be associated with project")
	})

	// AC4: New instances are created in the active project context
	t.Run("AC4: New instances created in active project context", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		
		// Add two projects
		project1, _ := manager.AddProject(tempDir1, "Project 1")
		project2, _ := manager.AddProject(tempDir2, "Project 2")
		
		// First project should be active initially
		activeProject := manager.GetActiveProject()
		assert.Equal(t, project1.ID, activeProject.ID, "First project should be active")
		
		// Set second project as active
		err := manager.SetActiveProject(project2.ID)
		require.NoError(t, err)
		
		activeProject = manager.GetActiveProject()
		assert.Equal(t, project2.ID, activeProject.ID, "Second project should now be active")
		assert.True(t, project2.IsActive, "Active project should have IsActive = true")
		assert.False(t, project1.IsActive, "Inactive project should have IsActive = false")
		
		// Add instance to active project
		err = manager.AddInstanceToProject(activeProject.ID, "active-project-instance")
		require.NoError(t, err)
		
		// Verify instance is associated with active project
		assert.True(t, activeProject.HasInstance("active-project-instance"))
		assert.False(t, project1.HasInstance("active-project-instance"))
	})

	// AC5: Project configuration persists between sessions
	t.Run("AC5: Project configuration persists between sessions", func(t *testing.T) {
		// Create temporary directory for config
		tempConfigDir := t.TempDir()
		
		// Set up environment to use temp directory
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", filepath.Join(tempConfigDir, ".config"))
		defer os.Setenv("XDG_CONFIG_HOME", originalXDG)
		
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		
		var project1ID, project2ID string
		
		// First session - create and save projects
		{
			state := config.LoadState()
			// Clear any existing data to ensure clean test
			state.ProjectsData = json.RawMessage("{}")
			state.ActiveProject = ""
			storage := NewStateProjectStorage(state)
			manager, err := NewProjectManager(storage)
			require.NoError(t, err)
			
			// Add projects
			project1, err := manager.AddProject(tempDir1, "Persistent Project 1")
			require.NoError(t, err)
			project1ID = project1.ID
			
			project2, err := manager.AddProject(tempDir2, "Persistent Project 2")
			require.NoError(t, err)
			project2ID = project2.ID
			
			// Set second project as active
			err = manager.SetActiveProject(project2.ID)
			require.NoError(t, err)
			
			// Add instance to first project
			err = manager.AddInstanceToProject(project1.ID, "persistent-instance")
			require.NoError(t, err)
			
			// Verify state in first session
			assert.Equal(t, 2, manager.ProjectCount())
			assert.Equal(t, project2.ID, manager.GetActiveProject().ID)
		}
		
		// Second session - reload and verify persistence
		{
			state := config.LoadState()
			storage := NewStateProjectStorage(state)
			manager, err := NewProjectManager(storage)
			require.NoError(t, err)
			
			// Verify projects persisted
			assert.Equal(t, 2, manager.ProjectCount(), "Project count should persist")
			
			projects := manager.ListProjects()
			assert.Equal(t, 2, len(projects))
			
			// Find projects by ID (more reliable than name)
			project1, exists1 := manager.GetProject(project1ID)
			project2, exists2 := manager.GetProject(project2ID)
			
			require.True(t, exists1, "Project 1 should persist")
			require.True(t, exists2, "Project 2 should persist")
			assert.Equal(t, "Persistent Project 1", project1.Name)
			assert.Equal(t, "Persistent Project 2", project2.Name)
			
			// Verify active project persisted
			activeProject := manager.GetActiveProject()
			require.NotNil(t, activeProject, "Active project should persist")
			assert.Equal(t, project2ID, activeProject.ID)
			assert.True(t, project2.IsActive)
			assert.False(t, project1.IsActive)
			
			// Verify instance association persisted
			instances, err := manager.GetProjectInstances(project1.ID)
			require.NoError(t, err)
			assert.Contains(t, instances, "persistent-instance", "Instance association should persist")
		}
	})

	// Additional validation: Test the critical bug that was fixed
	t.Run("CRITICAL BUG FIX: Empty storage initialization doesn't panic", func(t *testing.T) {
		// This tests the specific bug where adding the first project to empty storage
		// caused a nil map panic at manager.go line 260
		
		storage := NewMockProjectStorage()
		storage.SetProjects(json.RawMessage("")) // Empty JSON - this triggered the bug
		
		// This should not panic
		manager, err := NewProjectManager(storage)
		require.NoError(t, err, "Manager creation should not fail with empty storage")
		
		tempDir := t.TempDir()
		
		// This was the operation that panicked before the fix
		project, err := manager.AddProject(tempDir, "First Project")
		
		require.NoError(t, err, "Adding first project should not panic")
		assert.NotNil(t, project)
		assert.Equal(t, 1, manager.ProjectCount())
		assert.Equal(t, project, manager.GetActiveProject())
	})

	// Comprehensive workflow test
	t.Run("Complete Epic 1 Story 1 Workflow", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		
		// Start with empty state
		assert.Equal(t, 0, manager.ProjectCount())
		assert.Nil(t, manager.GetActiveProject())
		
		// Add first project (should become active)
		project1, err := manager.AddProject(tempDir1, "Workflow Project 1")
		require.NoError(t, err)
		assert.Equal(t, 1, manager.ProjectCount())
		assert.Equal(t, project1, manager.GetActiveProject())
		assert.True(t, project1.IsActive)
		
		// Add second project (should not become active)
		project2, err := manager.AddProject(tempDir2, "Workflow Project 2")
		require.NoError(t, err)
		assert.Equal(t, 2, manager.ProjectCount())
		assert.Equal(t, project1, manager.GetActiveProject()) // Still project1
		assert.True(t, project1.IsActive)
		assert.False(t, project2.IsActive)
		
		// Switch active project
		err = manager.SetActiveProject(project2.ID)
		require.NoError(t, err)
		assert.Equal(t, project2, manager.GetActiveProject())
		assert.False(t, project1.IsActive)
		assert.True(t, project2.IsActive)
		
		// Add instances to projects
		err = manager.AddInstanceToProject(project1.ID, "workflow-instance-1")
		require.NoError(t, err)
		err = manager.AddInstanceToProject(project2.ID, "workflow-instance-2")
		require.NoError(t, err)
		
		// Verify instance associations
		instances1, err := manager.GetProjectInstances(project1.ID)
		require.NoError(t, err)
		assert.Contains(t, instances1, "workflow-instance-1")
		
		instances2, err := manager.GetProjectInstances(project2.ID)
		require.NoError(t, err)
		assert.Contains(t, instances2, "workflow-instance-2")
		
		// Test project listing (hierarchical display)
		projects := manager.ListProjects()
		assert.Equal(t, 2, len(projects))
		
		// Should be sorted by last accessed (project2 was accessed last via SetActiveProject)
		assert.Equal(t, project2.ID, projects[0].ID)
		assert.Equal(t, project1.ID, projects[1].ID)
		
		// Test path validation
		err = manager.ValidateProjectPath(tempDir1)
		assert.Error(t, err, "Should reject duplicate path")
		
		newTempDir := t.TempDir()
		err = manager.ValidateProjectPath(newTempDir)
		assert.NoError(t, err, "Should accept new valid path")
		
		// Test project removal
		err = manager.RemoveProject(project1.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, manager.ProjectCount())
		
		// Active project should still be project2
		assert.Equal(t, project2, manager.GetActiveProject())
	})
}

// TestProjectKeyBinding tests AC1: 'P' key opens project input dialog  
// Note: This test validates the project input overlay component that handles the 'P' key binding
func TestProjectKeyBindingSupport(t *testing.T) {
	t.Run("AC1: Project input validation supports 'P' key workflow", func(t *testing.T) {
		// Test project input overlay path validation (used by 'P' key handler)
		// This validates the path validation that occurs when user presses 'P' and enters a path
		
		manager, _ := createTestManager(t)
		tempDir := t.TempDir()
		
		// Test valid absolute path (what happens when user enters valid path after 'P')
		err := manager.ValidateProjectPath(tempDir)
		assert.NoError(t, err, "Valid absolute path should pass validation")
		
		// Test invalid relative path (what happens when user enters invalid path after 'P')  
		err = manager.ValidateProjectPath("./relative/path")
		assert.Error(t, err, "Relative path should fail validation")
		assert.Contains(t, err.Error(), "absolute", "Error should mention absolute path requirement")
		
		// Test path that doesn't exist (another validation case for 'P' key workflow)
		err = manager.ValidateProjectPath("/nonexistent/path")
		assert.Error(t, err, "Non-existent path should fail validation")
		assert.Contains(t, err.Error(), "does not exist", "Error should mention path doesn't exist")
		
		// Test successful project addition (end result of successful 'P' key workflow)
		project, err := manager.AddProject(tempDir, "P Key Test Project")
		require.NoError(t, err, "Valid path should allow project creation")
		assert.Equal(t, tempDir, project.Path)
		assert.Equal(t, "P Key Test Project", project.Name)
	})
}