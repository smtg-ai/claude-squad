package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstanceStatus represents the status of a Hivemind instance as stored in state.json.
// Values match the iota order in session/instance.go.
type InstanceStatus int

const (
	StatusRunning InstanceStatus = iota
	StatusReady
	StatusLoading
	StatusPaused
)

// String returns a human-readable status string.
func (s InstanceStatus) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusReady:
		return "ready"
	case StatusLoading:
		return "loading"
	case StatusPaused:
		return "paused"
	default:
		return "unknown"
	}
}

// WorktreeInfo represents the git worktree data for an instance.
type WorktreeInfo struct {
	RepoPath      string `json:"repo_path"`
	WorktreePath  string `json:"worktree_path"`
	SessionName   string `json:"session_name"`
	BranchName    string `json:"branch_name"`
	BaseCommitSHA string `json:"base_commit_sha"`
}

// DiffStatsInfo represents the diff stats for an instance.
type DiffStatsInfo struct {
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Content string `json:"content,omitempty"`
}

// InstanceInfo represents a single Hivemind instance as stored in state.json.
type InstanceInfo struct {
	Title     string         `json:"title"`
	Path      string         `json:"path"`
	Branch    string         `json:"branch"`
	Status    InstanceStatus `json:"status"`
	Program   string         `json:"program"`
	TopicName string         `json:"topic_name,omitempty"`
	Worktree  WorktreeInfo   `json:"worktree"`
	DiffStats DiffStatsInfo  `json:"diff_stats"`
}

// stateFile represents the top-level structure of state.json.
type stateFile struct {
	Instances json.RawMessage `json:"instances"`
}

// StateReader reads instance data from the Hivemind state file.
type StateReader struct {
	hivemindDir string
}

// NewStateReader creates a StateReader that reads from the given Hivemind directory.
func NewStateReader(hivemindDir string) *StateReader {
	return &StateReader{hivemindDir: hivemindDir}
}

// ReadInstances reads and parses all instances from state.json.
func (r *StateReader) ReadInstances() ([]InstanceInfo, error) {
	statePath := filepath.Join(r.hivemindDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	if len(state.Instances) == 0 || string(state.Instances) == "null" {
		return nil, nil
	}

	var instances []InstanceInfo
	if err := json.Unmarshal(state.Instances, &instances); err != nil {
		return nil, fmt.Errorf("parsing instances: %w", err)
	}

	return instances, nil
}
