package mcp

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	maxMessages   = 50
	staleAgentAge = time.Hour
)

// brainFile is the on-disk format for the per-repo brain file.
type brainFile struct {
	Agents   map[string]*agentStatus `json:"agents"`
	Messages []brainMessage          `json:"messages"`
}

// agentStatus tracks what an agent is currently working on.
type agentStatus struct {
	Feature   string   `json:"feature"`
	Files     []string `json:"files"`
	UpdatedAt string   `json:"updated_at"`
}

// brainMessage is a directed message between agents.
type brainMessage struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// brainPath returns the path to the brain file for a specific repo.
// Brain files are scoped per repo: ~/.hivemind/brains/<hash>.json
// so agents in different repos don't see each other.
func brainPath(hivemindDir, repoPath string) string {
	h := sha256.Sum256([]byte(repoPath))
	name := fmt.Sprintf("%x.json", h[:8])
	return filepath.Join(hivemindDir, "brains", name)
}

// readBrain reads and parses the brain file, returning an empty brain if the file doesn't exist.
func readBrain(hivemindDir, repoPath string) (*brainFile, error) {
	data, err := os.ReadFile(brainPath(hivemindDir, repoPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &brainFile{Agents: make(map[string]*agentStatus)}, nil
		}
		return nil, err
	}

	var brain brainFile
	if err := json.Unmarshal(data, &brain); err != nil {
		return nil, err
	}
	if brain.Agents == nil {
		brain.Agents = make(map[string]*agentStatus)
	}
	return &brain, nil
}

// writeBrain atomically writes the brain file, creating the brains directory if needed.
func writeBrain(hivemindDir, repoPath string, brain *brainFile) error {
	data, err := json.MarshalIndent(brain, "", "  ")
	if err != nil {
		return err
	}

	bp := brainPath(hivemindDir, repoPath)
	if err := os.MkdirAll(filepath.Dir(bp), 0700); err != nil {
		return err
	}

	tmpPath := bp + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, bp)
}

// pruneStaleAgents removes agents that haven't updated in the given duration.
func pruneStaleAgents(brain *brainFile, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	for id, agent := range brain.Agents {
		t, err := time.Parse(time.RFC3339, agent.UpdatedAt)
		if err != nil || t.Before(cutoff) {
			delete(brain.Agents, id)
		}
	}
}

// fileConflicts returns a map of files claimed by multiple agents.
// Key is the file path, value is the list of agent IDs working on it.
func fileConflicts(brain *brainFile, excludeAgent string) map[string][]string {
	filesToAgents := make(map[string][]string)
	for id, agent := range brain.Agents {
		if id == excludeAgent {
			continue
		}
		for _, f := range agent.Files {
			filesToAgents[f] = append(filesToAgents[f], id)
		}
	}
	return filesToAgents
}
