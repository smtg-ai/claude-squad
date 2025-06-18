package ui

import (
	"claude-squad/config"
	"claude-squad/project"
	"claude-squad/session"
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

				list = NewList(spinner, false, pm, config.DefaultConfig())
			} else {
				list = NewList(spinner, false, nil, config.DefaultConfig())
			}

			// Test the generateTitleText method
			result := list.generateTitleText()
			if result != tt.expected {
				t.Errorf("generateTitleText() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestInstanceRenderer_getInstanceMCPs(t *testing.T) {
	// Create a test config with MCP assignments
	cfg := config.DefaultConfig()
	cfg.WorktreeMCPs = map[string][]string{
		"/test/worktree/path": {"mcp-server1", "mcp-server2"},
		"/another/path":       {"filesystem", "github"},
	}

	// Create renderer with config
	spinner := &spinner.Model{}
	renderer := &InstanceRenderer{
		spinner: spinner,
		config:  cfg,
	}

	tests := []struct {
		name          string
		setupInstance func() *session.Instance
		expectedMCPs  []string
		expectedNil   bool
	}{
		{
			name: "instance not started",
			setupInstance: func() *session.Instance {
				// Create instance that hasn't been started
				instance, _ := session.NewInstance(session.InstanceOptions{
					Title:   "test",
					Path:    "/tmp",
					Program: "echo",
				})
				return instance
			},
			expectedMCPs: nil,
			expectedNil:  true,
		},
		{
			name: "no config",
			setupInstance: func() *session.Instance {
				instance, _ := session.NewInstance(session.InstanceOptions{
					Title:   "test",
					Path:    "/tmp",
					Program: "echo",
				})
				return instance
			},
			expectedMCPs: nil,
			expectedNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := tt.setupInstance()

			// For the no config test, set renderer config to nil
			if tt.name == "no config" {
				renderer.config = nil
			} else {
				renderer.config = cfg
			}

			result := renderer.getInstanceMCPs(instance)

			if tt.expectedNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
			} else {
				if len(result) != len(tt.expectedMCPs) {
					t.Errorf("Expected %d MCPs, got %d", len(tt.expectedMCPs), len(result))
				}
				for i, expected := range tt.expectedMCPs {
					if i >= len(result) || result[i] != expected {
						t.Errorf("Expected MCP %s at index %d, got %s", expected, i, result[i])
					}
				}
			}
		})
	}
}
