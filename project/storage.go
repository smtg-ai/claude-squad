package project

import (
	"claude-squad/config"
	"encoding/json"
	"fmt"
)

// StateProjectStorage implements ProjectStorage using the application state
type StateProjectStorage struct {
	state config.StateManager
}

// NewStateProjectStorage creates a new project storage using the application state
func NewStateProjectStorage(state config.StateManager) *StateProjectStorage {
	return &StateProjectStorage{
		state: state,
	}
}

// SaveProjects saves the serialized project data to state
func (s *StateProjectStorage) SaveProjects(projectsJSON json.RawMessage) error {
	// Get current state as *State to access ProjectsData field
	if state, ok := s.state.(*config.State); ok {
		state.ProjectsData = projectsJSON
		return config.SaveState(state)
	}
	return fmt.Errorf("state is not of type *config.State")
}

// GetProjects returns the serialized project data from state
func (s *StateProjectStorage) GetProjects() json.RawMessage {
	// Get current state as *State to access ProjectsData field
	if state, ok := s.state.(*config.State); ok {
		return state.ProjectsData
	}
	return json.RawMessage("{}")
}

// DeleteProject removes a project from storage (not directly supported by this implementation)
func (s *StateProjectStorage) DeleteProject(projectID string) error {
	// Get current projects
	projectsJSON := s.GetProjects()
	if len(projectsJSON) == 0 {
		return nil // No projects to delete from
	}

	var projects map[string]*Project
	if err := json.Unmarshal(projectsJSON, &projects); err != nil {
		return fmt.Errorf("failed to unmarshal projects: %w", err)
	}

	// Remove the project
	delete(projects, projectID)

	// Save back to storage
	updatedJSON, err := json.Marshal(projects)
	if err != nil {
		return fmt.Errorf("failed to marshal updated projects: %w", err)
	}

	return s.SaveProjects(updatedJSON)
}

// SetActiveProject sets the currently active project ID
func (s *StateProjectStorage) SetActiveProject(projectID string) error {
	if state, ok := s.state.(*config.State); ok {
		state.ActiveProject = projectID
		return config.SaveState(state)
	}
	return fmt.Errorf("state is not of type *config.State")
}

// GetActiveProject returns the currently active project ID
func (s *StateProjectStorage) GetActiveProject() string {
	if state, ok := s.state.(*config.State); ok {
		return state.ActiveProject
	}
	return ""
}

// SaveProjectHistory saves the project history to state
func (s *StateProjectStorage) SaveProjectHistory(history *ProjectHistory) error {
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal project history: %w", err)
	}

	if state, ok := s.state.(*config.State); ok {
		state.ProjectHistoryData = historyJSON
		return config.SaveState(state)
	}
	return fmt.Errorf("state is not of type *config.State")
}

// GetProjectHistory returns the project history from state
func (s *StateProjectStorage) GetProjectHistory() *ProjectHistory {
	if state, ok := s.state.(*config.State); ok {
		if len(state.ProjectHistoryData) == 0 {
			return NewProjectHistory()
		}

		var history ProjectHistory
		if err := json.Unmarshal(state.ProjectHistoryData, &history); err != nil {
			// Return new history if unmarshal fails
			return NewProjectHistory()
		}
		return &history
	}
	return NewProjectHistory()
}
