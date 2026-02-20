package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/ByteMirror/hivemind/brain"
)

// BrainClient abstracts brain state access for MCP tool handlers.
// Implemented by brain.Client (socket) and fileBrainClient (file fallback).
type BrainClient interface {
	GetBrain(repoPath, instanceID string) (*brain.BrainState, error)
	UpdateStatus(repoPath, instanceID, feature string, files []string) (*brain.UpdateStatusResult, error)
	SendMessage(repoPath, from, to, content string) error
	RemoveAgent(repoPath, instanceID string) error

	// Tier 3: actions relayed to TUI.
	CreateInstance(repoPath, instanceID string, params brain.CreateInstanceParams) (*brain.CreateInstanceResult, error)
	InjectMessage(repoPath, instanceID string, params brain.InjectMessageParams) error
	PauseInstance(repoPath, instanceID, target string) error
	ResumeInstance(repoPath, instanceID, target string) error
	KillInstance(repoPath, instanceID, target string) error

	// Tier 3: workflow DAG.
	DefineWorkflow(repoPath, instanceID string, tasks []*brain.WorkflowTask) (*brain.WorkflowResult, error)
	CompleteTask(repoPath, instanceID, taskID, status, errMsg string) (*brain.WorkflowResult, error)
	GetWorkflow(repoPath, instanceID string) (*brain.Workflow, error)

	// Event subscription.
	Subscribe(repoPath string, filter brain.EventFilter) (string, error)
	PollEvents(subscriberID string, timeoutSec int) ([]brain.Event, error)
	Unsubscribe(subscriberID string) error
}

// fileBrainClient is the fallback implementation that reads/writes brain JSON files directly.
// Used when the hivemind socket is not available.
type fileBrainClient struct {
	hivemindDir string
}

// NewFileBrainClient creates a file-based BrainClient.
func NewFileBrainClient(hivemindDir string) BrainClient {
	return &fileBrainClient{hivemindDir: hivemindDir}
}

func (c *fileBrainClient) GetBrain(repoPath, instanceID string) (*brain.BrainState, error) {
	bf, err := readBrain(c.hivemindDir, repoPath)
	if err != nil {
		return nil, err
	}

	pruneStaleAgents(bf, staleAgentAge)

	agents := make(map[string]*brain.AgentStatus, len(bf.Agents))
	for id, a := range bf.Agents {
		agents[id] = &brain.AgentStatus{
			Feature:   a.Feature,
			Files:     a.Files,
			UpdatedAt: a.UpdatedAt,
		}
	}

	var msgs []brain.BrainMessage
	for _, m := range bf.Messages {
		if m.To == instanceID || m.To == "" {
			msgs = append(msgs, brain.BrainMessage{
				From:      m.From,
				To:        m.To,
				Content:   m.Content,
				Timestamp: m.Timestamp,
			})
		}
	}

	return &brain.BrainState{
		Agents:   agents,
		Messages: msgs,
	}, nil
}

func (c *fileBrainClient) UpdateStatus(repoPath, instanceID, feature string, files []string) (*brain.UpdateStatusResult, error) {
	bf, err := readBrain(c.hivemindDir, repoPath)
	if err != nil {
		return nil, err
	}

	bf.Agents[instanceID] = &agentStatus{
		Feature:   feature,
		Files:     files,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := writeBrain(c.hivemindDir, repoPath, bf); err != nil {
		return nil, err
	}

	conflicts := fileConflicts(bf, instanceID)
	var warnings []string
	for _, f := range files {
		if agents, ok := conflicts[f]; ok {
			warnings = append(warnings, fmt.Sprintf("%s is also being worked on by: %s", f, strings.Join(agents, ", ")))
		}
	}

	return &brain.UpdateStatusResult{Conflicts: warnings}, nil
}

func (c *fileBrainClient) SendMessage(repoPath, from, to, content string) error {
	bf, err := readBrain(c.hivemindDir, repoPath)
	if err != nil {
		return err
	}

	bf.Messages = append(bf.Messages, brainMessage{
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})

	if len(bf.Messages) > maxMessages {
		bf.Messages = bf.Messages[len(bf.Messages)-maxMessages:]
	}

	return writeBrain(c.hivemindDir, repoPath, bf)
}

func (c *fileBrainClient) RemoveAgent(repoPath, instanceID string) error {
	bf, err := readBrain(c.hivemindDir, repoPath)
	if err != nil {
		return err
	}

	delete(bf.Agents, instanceID)
	return writeBrain(c.hivemindDir, repoPath, bf)
}

var errRequiresSocket = fmt.Errorf("this operation requires the Hivemind TUI to be running (socket connection)")

func (c *fileBrainClient) CreateInstance(repoPath, instanceID string, params brain.CreateInstanceParams) (*brain.CreateInstanceResult, error) {
	return nil, errRequiresSocket
}

func (c *fileBrainClient) InjectMessage(repoPath, instanceID string, params brain.InjectMessageParams) error {
	return errRequiresSocket
}

func (c *fileBrainClient) PauseInstance(repoPath, instanceID, target string) error {
	return errRequiresSocket
}

func (c *fileBrainClient) ResumeInstance(repoPath, instanceID, target string) error {
	return errRequiresSocket
}

func (c *fileBrainClient) KillInstance(repoPath, instanceID, target string) error {
	return errRequiresSocket
}

func (c *fileBrainClient) DefineWorkflow(repoPath, instanceID string, tasks []*brain.WorkflowTask) (*brain.WorkflowResult, error) {
	return nil, errRequiresSocket
}

func (c *fileBrainClient) CompleteTask(repoPath, instanceID, taskID, status, errMsg string) (*brain.WorkflowResult, error) {
	return nil, errRequiresSocket
}

func (c *fileBrainClient) GetWorkflow(repoPath, instanceID string) (*brain.Workflow, error) {
	return nil, errRequiresSocket
}

func (c *fileBrainClient) Subscribe(repoPath string, filter brain.EventFilter) (string, error) {
	return "", errRequiresSocket
}

func (c *fileBrainClient) PollEvents(subscriberID string, timeoutSec int) ([]brain.Event, error) {
	return nil, errRequiresSocket
}

func (c *fileBrainClient) Unsubscribe(subscriberID string) error {
	return errRequiresSocket
}
