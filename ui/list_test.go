package ui

import (
	"claude-squad/project"
	"encoding/json"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
)

// mockProjectStorage implements project.ProjectStorage interface for testing
type mockProjectStorage struct {
	activeProjectID string
}

func (m *mockProjectStorage) SaveProjects(projectsJSON json.RawMessage) error { return nil }
func (m *mockProjectStorage) GetProjects() json.RawMessage                    { return json.RawMessage("{}") }
func (m *mockProjectStorage) DeleteProject(projectID string) error            { return nil }
func (m *mockProjectStorage) SetActiveProject(projectID string) error         { return nil }
func (m *mockProjectStorage) GetActiveProject() string                        { return m.activeProjectID }

func TestList_generateTitleText(t *testing.T) {
	tests := []struct {
		name             string
		hasProjectMgr    bool
		hasActiveProject bool
		projectName      string
		expected         string
	}{
		{
			name:             "no project manager",
			hasProjectMgr:    false,
			hasActiveProject: false,
			projectName:      "",
			expected:         " Instances ",
		},
		{
			name:             "no active project",
			hasProjectMgr:    true,
			hasActiveProject: false,
			projectName:      "",
			expected:         " Instances ",
		},
		{
			name:             "with active project",
			hasProjectMgr:    true,
			hasActiveProject: true,
			projectName:      "MyProject",
			expected:         " Instances (MyProject) ",
		},
		{
			name:             "with active project - complex name",
			hasProjectMgr:    true,
			hasActiveProject: true,
			projectName:      "Complex Project Name",
			expected:         " Instances (Complex Project Name) ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a list
			spinner := &spinner.Model{}
			var list *List

			if tt.hasProjectMgr {
				// Create project manager with mock storage
				storage := &mockProjectStorage{}
				pm, err := project.NewProjectManager(storage)
				if err != nil {
					t.Fatalf("Failed to create project manager: %v", err)
				}

				if tt.hasActiveProject {
					// Add a project and make it active - use current directory as it exists
					proj, err := pm.AddProject("/tmp", tt.projectName)
					if err != nil {
						t.Fatalf("Failed to add project: %v", err)
					}
					if err := pm.SetActiveProject(proj.ID); err != nil {
						t.Fatalf("Failed to set active project: %v", err)
					}
				}

				list = NewList(spinner, false, pm)
			} else {
				list = NewList(spinner, false, nil)
			}

			// Test the generateTitleText method
			result := list.generateTitleText()
			if result != tt.expected {
				t.Errorf("generateTitleText() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
