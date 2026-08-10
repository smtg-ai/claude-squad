package session

import (
	"claude-squad/config"
	"encoding/json"
	"fmt"
	"time"
)

// InstanceData represents the serializable data of an Instance
type InstanceData struct {
	Title     string    `json:"title"`
	Path      string    `json:"path"`
	Branch    string    `json:"branch"`
	Status    Status    `json:"status"`
	Height    int       `json:"height"`
	Width     int       `json:"width"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	AutoYes   bool      `json:"auto_yes"`

	Program   string          `json:"program"`
	Worktree  GitWorktreeData `json:"worktree"`
	DiffStats DiffStatsData   `json:"diff_stats"`
}

// GitWorktreeData represents the serializable data of a GitWorktree
type GitWorktreeData struct {
	RepoPath         string `json:"repo_path"`
	WorktreePath     string `json:"worktree_path"`
	SessionName      string `json:"session_name"`
	BranchName       string `json:"branch_name"`
	BaseCommitSHA    string `json:"base_commit_sha"`
	IsExistingBranch bool   `json:"is_existing_branch"`
}

// DiffStatsData represents the serializable data of a DiffStats
type DiffStatsData struct {
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Content string `json:"content"`
}

// Storage handles saving and loading instances using the state interface
type Storage struct {
	state config.InstanceStorage
}

// NewStorage creates a new storage instance
func NewStorage(state config.InstanceStorage) (*Storage, error) {
	return &Storage{
		state: state,
	}, nil
}

// unmarshalInstances parses raw instance data into a list of InstanceData.
func unmarshalInstances(jsonData json.RawMessage) ([]InstanceData, error) {
	instancesData := make([]InstanceData, 0)
	if len(jsonData) == 0 {
		return instancesData, nil
	}
	if err := json.Unmarshal(jsonData, &instancesData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instances: %w", err)
	}
	return instancesData, nil
}

// SaveInstances saves the list of instances to disk. Each instance is merged
// into the stored list by title: existing entries are updated in place and new
// ones appended. Instances saved by other claude-squad processes are
// preserved; removal only happens through DeleteInstance or
// DeleteAllInstances.
func (s *Storage) SaveInstances(instances []*Instance) error {
	return s.state.UpdateInstances(func(onDisk json.RawMessage) (json.RawMessage, error) {
		stored, err := unmarshalInstances(onDisk)
		if err != nil {
			return nil, err
		}

		for _, instance := range instances {
			if !instance.Started() {
				continue
			}
			data := instance.ToInstanceData()
			replaced := false
			for i := range stored {
				if stored[i].Title == data.Title {
					stored[i] = data
					replaced = true
					break
				}
			}
			if !replaced {
				stored = append(stored, data)
			}
		}

		jsonData, err := json.Marshal(stored)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal instances: %w", err)
		}
		return jsonData, nil
	})
}

// LoadInstances loads the list of instances from disk
func (s *Storage) LoadInstances() ([]*Instance, error) {
	instancesData, err := unmarshalInstances(s.state.GetInstances())
	if err != nil {
		return nil, err
	}

	instances := make([]*Instance, len(instancesData))
	for i, data := range instancesData {
		instance, err := FromInstanceData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to create instance %s: %w", data.Title, err)
		}
		instances[i] = instance
	}

	return instances, nil
}

// DeleteInstance removes an instance from storage
func (s *Storage) DeleteInstance(title string) error {
	found := false
	err := s.state.UpdateInstances(func(onDisk json.RawMessage) (json.RawMessage, error) {
		stored, err := unmarshalInstances(onDisk)
		if err != nil {
			return nil, err
		}

		remaining := make([]InstanceData, 0, len(stored))
		for _, data := range stored {
			if data.Title == title {
				found = true
			} else {
				remaining = append(remaining, data)
			}
		}

		jsonData, err := json.Marshal(remaining)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal instances: %w", err)
		}
		return jsonData, nil
	})
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("instance not found: %s", title)
	}
	return nil
}

// UpdateInstance updates an existing instance in storage
func (s *Storage) UpdateInstance(instance *Instance) error {
	data := instance.ToInstanceData()
	found := false
	err := s.state.UpdateInstances(func(onDisk json.RawMessage) (json.RawMessage, error) {
		stored, err := unmarshalInstances(onDisk)
		if err != nil {
			return nil, err
		}

		for i := range stored {
			if stored[i].Title == data.Title {
				stored[i] = data
				found = true
				break
			}
		}

		jsonData, err := json.Marshal(stored)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal instances: %w", err)
		}
		return jsonData, nil
	})
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("instance not found: %s", data.Title)
	}
	return nil
}

// DeleteAllInstances removes all stored instances
func (s *Storage) DeleteAllInstances() error {
	return s.state.DeleteAllInstances()
}
