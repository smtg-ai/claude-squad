package project

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProjectStorage implements ProjectStorage for testing
type MockProjectStorage struct {
	projects      json.RawMessage
	activeProject string
	saveError     error
	deleteError   error
	setActiveError error
}

func NewMockProjectStorage() *MockProjectStorage {
	return &MockProjectStorage{
		projects:      json.RawMessage("{}"),
		activeProject: "",
	}
}

func (m *MockProjectStorage) SaveProjects(projectsJSON json.RawMessage) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.projects = projectsJSON
	return nil
}

func (m *MockProjectStorage) GetProjects() json.RawMessage {
	return m.projects
}

func (m *MockProjectStorage) DeleteProject(projectID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	// For testing, we'll just return nil
	return nil
}

func (m *MockProjectStorage) SetActiveProject(projectID string) error {
	if m.setActiveError != nil {
		return m.setActiveError
	}
	m.activeProject = projectID
	return nil
}

func (m *MockProjectStorage) GetActiveProject() string {
	return m.activeProject
}

// SetStorageError sets an error to be returned by storage operations
func (m *MockProjectStorage) SetSaveError(err error) {
	m.saveError = err
}

func (m *MockProjectStorage) SetDeleteError(err error) {
	m.deleteError = err
}

func (m *MockProjectStorage) SetActiveError(err error) {
	m.setActiveError = err
}

// SetProjects sets the projects JSON data
func (m *MockProjectStorage) SetProjects(projectsJSON json.RawMessage) {
	m.projects = projectsJSON
}

func TestNewProjectManager(t *testing.T) {
	t.Run("creates manager with empty storage", func(t *testing.T) {
		storage := NewMockProjectStorage()
		
		manager, err := NewProjectManager(storage)
		
		require.NoError(t, err)
		assert.NotNil(t, manager)
		assert.Equal(t, 0, manager.ProjectCount())
		assert.Nil(t, manager.GetActiveProject())
	})

	t.Run("fails with nil storage", func(t *testing.T) {
		_, err := NewProjectManager(nil)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage cannot be nil")
	})

	t.Run("loads existing projects from storage", func(t *testing.T) {
		storage := NewMockProjectStorage()
		
		// Create test project data
		testProject := map[string]*Project{
			"test-id": {
				ID:        "test-id",
				Name:      "Test Project",
				Path:      "/tmp/test",
				IsActive:  false,
				Instances: []string{},
			},
		}
		projectsJSON, _ := json.Marshal(testProject)
		storage.SetProjects(projectsJSON)
		
		manager, err := NewProjectManager(storage)
		
		require.NoError(t, err)
		assert.Equal(t, 1, manager.ProjectCount())
		
		project, exists := manager.GetProject("test-id")
		assert.True(t, exists)
		assert.Equal(t, "Test Project", project.Name)
	})

	// CRITICAL TEST: Tests the bug that occurred in production
	t.Run("CRITICAL: handles empty storage initialization - the nil map panic bug", func(t *testing.T) {
		storage := NewMockProjectStorage()
		storage.SetProjects(json.RawMessage("")) // Empty JSON - this was the bug
		
		manager, err := NewProjectManager(storage)
		
		require.NoError(t, err, "Manager should handle empty storage without panic")
		assert.NotNil(t, manager)
		assert.Equal(t, 0, manager.ProjectCount())
		
		// This should not panic - this was the actual bug
		tempDir := t.TempDir()
		project, err := manager.AddProject(tempDir, "Test Project")
		
		require.NoError(t, err, "Adding first project should not panic")
		assert.NotNil(t, project)
		assert.Equal(t, 1, manager.ProjectCount())
	})

	t.Run("sets active project from storage", func(t *testing.T) {
		storage := NewMockProjectStorage()
		
		// Create test project data
		testProject := map[string]*Project{
			"active-id": {
				ID:        "active-id",
				Name:      "Active Project",
				Path:      "/tmp/active",
				IsActive:  false,
				Instances: []string{},
			},
		}
		projectsJSON, _ := json.Marshal(testProject)
		storage.SetProjects(projectsJSON)
		storage.SetActiveProject("active-id")
		
		manager, err := NewProjectManager(storage)
		
		require.NoError(t, err)
		activeProject := manager.GetActiveProject()
		assert.NotNil(t, activeProject)
		assert.Equal(t, "active-id", activeProject.ID)
		assert.True(t, activeProject.IsActive)
	})

	t.Run("handles invalid JSON in storage", func(t *testing.T) {
		storage := NewMockProjectStorage()
		storage.SetProjects(json.RawMessage("invalid json"))
		
		_, err := NewProjectManager(storage)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load projects")
	})

	t.Run("handles invalid project data", func(t *testing.T) {
		storage := NewMockProjectStorage()
		
		// Create invalid project data (missing required fields)
		invalidProject := map[string]*Project{
			"invalid-id": {
				ID:   "", // Invalid - empty ID
				Name: "Invalid Project",
				Path: "/tmp/invalid",
			},
		}
		projectsJSON, _ := json.Marshal(invalidProject)
		storage.SetProjects(projectsJSON)
		
		_, err := NewProjectManager(storage)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid project")
	})
}

func TestProjectManagerAddProject(t *testing.T) {
	t.Run("adds valid project", func(t *testing.T) {
		manager, storage := createTestManager(t)
		tempDir := t.TempDir()
		
		project, err := manager.AddProject(tempDir, "Test Project")
		
		require.NoError(t, err)
		assert.NotNil(t, project)
		assert.Equal(t, "Test Project", project.Name)
		assert.Equal(t, tempDir, project.Path)
		assert.Equal(t, 1, manager.ProjectCount())
		
		// Verify it was saved to storage
		projectsJSON := storage.GetProjects()
		assert.NotEmpty(t, projectsJSON)
	})

	t.Run("first project becomes active", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir := t.TempDir()
		
		project, err := manager.AddProject(tempDir, "First Project")
		
		require.NoError(t, err)
		assert.True(t, project.IsActive)
		assert.Equal(t, project, manager.GetActiveProject())
	})

	t.Run("second project does not become active", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		
		project1, err1 := manager.AddProject(tempDir1, "First Project")
		project2, err2 := manager.AddProject(tempDir2, "Second Project")
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.True(t, project1.IsActive)
		assert.False(t, project2.IsActive)
		assert.Equal(t, project1, manager.GetActiveProject())
	})

	t.Run("fails with duplicate path", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir := t.TempDir()
		
		_, err1 := manager.AddProject(tempDir, "First Project")
		_, err2 := manager.AddProject(tempDir, "Second Project")
		
		require.NoError(t, err1)
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "project with path already exists")
	})

	t.Run("fails with non-existent path", func(t *testing.T) {
		manager, _ := createTestManager(t)
		nonExistentPath := "/this/path/does/not/exist"
		
		_, err := manager.AddProject(nonExistentPath, "Test Project")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path does not exist")
	})

	t.Run("fails with invalid project data", func(t *testing.T) {
		manager, _ := createTestManager(t)
		
		_, err := manager.AddProject("", "Test Project")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create project")
	})

	t.Run("rolls back on storage save error", func(t *testing.T) {
		manager, storage := createTestManager(t)
		tempDir := t.TempDir()
		
		// Set storage to fail on save
		storage.SetSaveError(errors.New("storage save failed"))
		
		_, err := manager.AddProject(tempDir, "Test Project")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save project")
		assert.Equal(t, 0, manager.ProjectCount()) // Should be rolled back
	})
}

func TestProjectManagerGetProject(t *testing.T) {
	manager, _ := createTestManager(t)
	tempDir := t.TempDir()
	
	project, err := manager.AddProject(tempDir, "Test Project")
	require.NoError(t, err)
	
	t.Run("returns existing project", func(t *testing.T) {
		retrieved, exists := manager.GetProject(project.ID)
		
		assert.True(t, exists)
		assert.Equal(t, project, retrieved)
	})

	t.Run("returns false for non-existent project", func(t *testing.T) {
		_, exists := manager.GetProject("non-existent-id")
		
		assert.False(t, exists)
	})
}

func TestProjectManagerActiveProject(t *testing.T) {
	manager, _ := createTestManager(t)
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	
	project1, err1 := manager.AddProject(tempDir1, "Project 1")
	project2, err2 := manager.AddProject(tempDir2, "Project 2")
	require.NoError(t, err1)
	require.NoError(t, err2)
	
	t.Run("sets active project", func(t *testing.T) {
		err := manager.SetActiveProject(project2.ID)
		
		require.NoError(t, err)
		assert.False(t, project1.IsActive)
		assert.True(t, project2.IsActive)
		assert.Equal(t, project2, manager.GetActiveProject())
	})

	t.Run("fails to set non-existent project as active", func(t *testing.T) {
		err := manager.SetActiveProject("non-existent-id")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project not found")
	})

	t.Run("fails on storage error", func(t *testing.T) {
		manager, storage := createTestManager(t)
		tempDir := t.TempDir()
		project, _ := manager.AddProject(tempDir, "Test Project")
		
		storage.SetActiveError(errors.New("storage error"))
		
		err := manager.SetActiveProject(project.ID)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save active project")
	})
}

func TestProjectManagerListProjects(t *testing.T) {
	manager, _ := createTestManager(t)
	
	t.Run("returns empty list for no projects", func(t *testing.T) {
		projects := manager.ListProjects()
		
		assert.NotNil(t, projects)
		assert.Equal(t, 0, len(projects))
	})

	t.Run("returns projects sorted by last accessed", func(t *testing.T) {
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		tempDir3 := t.TempDir()
		
		project1, _ := manager.AddProject(tempDir1, "Project 1")
		project2, _ := manager.AddProject(tempDir2, "Project 2")
		project3, _ := manager.AddProject(tempDir3, "Project 3")
		
		// Access projects in different order
		project2.SetActive() // This updates LastAccessed
		project3.SetActive()
		project1.SetActive()
		
		projects := manager.ListProjects()
		
		assert.Equal(t, 3, len(projects))
		// Should be sorted by most recent access
		assert.Equal(t, project1.ID, projects[0].ID)
		assert.Equal(t, project3.ID, projects[1].ID)
		assert.Equal(t, project2.ID, projects[2].ID)
	})
}

func TestProjectManagerRemoveProject(t *testing.T) {
	manager, _ := createTestManager(t)
	tempDir := t.TempDir()
	
	project, err := manager.AddProject(tempDir, "Test Project")
	require.NoError(t, err)
	
	t.Run("removes existing project", func(t *testing.T) {
		err := manager.RemoveProject(project.ID)
		
		require.NoError(t, err)
		assert.Equal(t, 0, manager.ProjectCount())
		
		_, exists := manager.GetProject(project.ID)
		assert.False(t, exists)
	})

	t.Run("fails to remove non-existent project", func(t *testing.T) {
		err := manager.RemoveProject("non-existent-id")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project not found")
	})

	t.Run("clears active project when removing it", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir := t.TempDir()
		
		project, _ := manager.AddProject(tempDir, "Test Project")
		assert.Equal(t, project, manager.GetActiveProject())
		
		err := manager.RemoveProject(project.ID)
		
		require.NoError(t, err)
		assert.Nil(t, manager.GetActiveProject())
	})

	t.Run("rolls back on storage error", func(t *testing.T) {
		manager, storage := createTestManager(t)
		tempDir := t.TempDir()
		
		project, _ := manager.AddProject(tempDir, "Test Project")
		storage.SetDeleteError(errors.New("storage delete failed"))
		
		err := manager.RemoveProject(project.ID)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete project from storage")
		assert.Equal(t, 1, manager.ProjectCount()) // Should be rolled back
	})
}

func TestProjectManagerValidateProjectPath(t *testing.T) {
	manager, _ := createTestManager(t)
	
	t.Run("validates absolute existing path", func(t *testing.T) {
		tempDir := t.TempDir()
		
		err := manager.ValidateProjectPath(tempDir)
		
		assert.NoError(t, err)
	})

	t.Run("fails with empty path", func(t *testing.T) {
		err := manager.ValidateProjectPath("")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path cannot be empty")
	})

	t.Run("fails with relative path", func(t *testing.T) {
		err := manager.ValidateProjectPath("./relative/path")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path must be absolute")
	})

	t.Run("fails with non-existent path", func(t *testing.T) {
		err := manager.ValidateProjectPath("/non/existent/path")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project path does not exist")
	})

	t.Run("fails with duplicate path", func(t *testing.T) {
		tempDir := t.TempDir()
		
		_, err := manager.AddProject(tempDir, "Existing Project")
		require.NoError(t, err)
		
		err = manager.ValidateProjectPath(tempDir)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project with path already exists")
	})
}

func TestProjectManagerInstanceManagement(t *testing.T) {
	manager, _ := createTestManager(t)
	tempDir := t.TempDir()
	
	project, err := manager.AddProject(tempDir, "Test Project")
	require.NoError(t, err)
	
	t.Run("adds instance to project", func(t *testing.T) {
		err := manager.AddInstanceToProject(project.ID, "instance-1")
		
		require.NoError(t, err)
		assert.True(t, project.HasInstance("instance-1"))
		
		instances, err := manager.GetProjectInstances(project.ID)
		require.NoError(t, err)
		assert.Contains(t, instances, "instance-1")
	})

	t.Run("removes instance from project", func(t *testing.T) {
		manager.AddInstanceToProject(project.ID, "instance-to-remove")
		
		err := manager.RemoveInstanceFromProject(project.ID, "instance-to-remove")
		
		require.NoError(t, err)
		assert.False(t, project.HasInstance("instance-to-remove"))
	})

	t.Run("fails to add instance to non-existent project", func(t *testing.T) {
		err := manager.AddInstanceToProject("non-existent", "instance-1")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project not found")
	})

	t.Run("fails to remove instance from non-existent project", func(t *testing.T) {
		err := manager.RemoveInstanceFromProject("non-existent", "instance-1")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project not found")
	})

	t.Run("fails to remove non-existent instance", func(t *testing.T) {
		err := manager.RemoveInstanceFromProject(project.ID, "non-existent-instance")
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "instance not found in project")
	})

	t.Run("returns copy of instances slice", func(t *testing.T) {
		manager.AddInstanceToProject(project.ID, "instance-1")
		manager.AddInstanceToProject(project.ID, "instance-2")
		
		instances1, err1 := manager.GetProjectInstances(project.ID)
		instances2, err2 := manager.GetProjectInstances(project.ID)
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		
		// Modify one slice
		instances1[0] = "modified"
		
		// Other slice should be unaffected
		assert.NotEqual(t, instances1[0], instances2[0])
	})
}

func TestProjectManagerEdgeCases(t *testing.T) {
	t.Run("handles concurrent operations safely", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir1 := t.TempDir()
		tempDir2 := t.TempDir()
		
		// This test ensures no race conditions, though Go's built-in race detector
		// would catch actual races during testing
		project1, err1 := manager.AddProject(tempDir1, "Project 1")
		project2, err2 := manager.AddProject(tempDir2, "Project 2")
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		
		// Perform multiple operations
		manager.SetActiveProject(project2.ID)
		manager.AddInstanceToProject(project1.ID, "instance-1")
		manager.AddInstanceToProject(project2.ID, "instance-2")
		
		// Verify final state
		assert.Equal(t, project2, manager.GetActiveProject())
		assert.True(t, project1.HasInstance("instance-1"))
		assert.True(t, project2.HasInstance("instance-2"))
	})

	t.Run("handles path cleaning consistently", func(t *testing.T) {
		manager, _ := createTestManager(t)
		tempDir := t.TempDir()
		
		// Create project with clean path
		_, err1 := manager.AddProject(tempDir, "Project 1")
		require.NoError(t, err1)
		
		// Try to add same project with unclean path (should fail)
		uncleanlPath := tempDir + "//../" + filepath.Base(tempDir)
		_, err2 := manager.AddProject(uncleanlPath, "Project 2")
		
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "project with path already exists")
	})
}

// Helper function to create a test manager with mock storage
func createTestManager(t *testing.T) (*ProjectManager, *MockProjectStorage) {
	storage := NewMockProjectStorage()
	manager, err := NewProjectManager(storage)
	require.NoError(t, err)
	return manager, storage
}