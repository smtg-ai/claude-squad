package brain

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	maxMessages   = 50
	staleAgentAge = time.Hour
)

// repoState holds brain state for a single repository.
type repoState struct {
	mu       sync.RWMutex
	agents   map[string]*AgentStatus
	messages []BrainMessage
	workflow *Workflow
}

// Manager holds per-repo brain state in memory with mutex protection.
// All mutations are serialized through this struct.
type Manager struct {
	mu      sync.RWMutex
	repos   map[string]*repoState
	onEvent func(Event)
}

// NewManager creates a new empty Manager.
func NewManager() *Manager {
	return &Manager{
		repos: make(map[string]*repoState),
	}
}

// SetEventCallback sets the function called when the manager emits an event.
func (m *Manager) SetEventCallback(fn func(Event)) {
	m.onEvent = fn
}

func (m *Manager) emitEvent(eventType EventType, repoPath, source string, data map[string]any) {
	if m.onEvent == nil {
		return
	}
	m.onEvent(Event{
		Type:      eventType,
		Timestamp: time.Now(),
		RepoPath:  repoPath,
		Source:    source,
		Data:      data,
	})
}

// getOrCreateRepo returns the repoState for the given path, creating it if needed.
func (m *Manager) getOrCreateRepo(repoPath string) *repoState {
	m.mu.RLock()
	rs, ok := m.repos[repoPath]
	m.mu.RUnlock()
	if ok {
		return rs
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after acquiring write lock.
	if rs, ok = m.repos[repoPath]; ok {
		return rs
	}
	rs = &repoState{
		agents: make(map[string]*AgentStatus),
	}
	m.repos[repoPath] = rs
	return rs
}

// GetBrain returns the brain state for a repo, filtered for the requesting agent.
// Stale agents are pruned. Messages are filtered to those addressed to instanceID or broadcast.
func (m *Manager) GetBrain(repoPath, instanceID string) *BrainState {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()
	defer rs.mu.Unlock()

	pruneStale(rs)

	// Copy agents map for the response.
	agents := make(map[string]*AgentStatus, len(rs.agents))
	for id, a := range rs.agents {
		cp := *a
		cp.Files = append([]string(nil), a.Files...)
		agents[id] = &cp
	}

	// Filter messages: only those addressed to this agent or broadcast.
	var msgs []BrainMessage
	for _, msg := range rs.messages {
		if msg.To == instanceID || msg.To == "" {
			msgs = append(msgs, msg)
		}
	}

	return &BrainState{
		Agents:   agents,
		Messages: msgs,
	}
}

// UpdateStatus sets the agent's feature and files, returning conflict warnings.
func (m *Manager) UpdateStatus(repoPath, instanceID, feature string, files []string) *UpdateStatusResult {
	return m.UpdateStatusWithRole(repoPath, instanceID, feature, files, "")
}

// SendMessage appends a message to the repo's message list, capping at maxMessages.
func (m *Manager) SendMessage(repoPath, from, to, content string) {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()

	rs.messages = append(rs.messages, BrainMessage{
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})

	if len(rs.messages) > maxMessages {
		rs.messages = rs.messages[len(rs.messages)-maxMessages:]
	}

	rs.mu.Unlock()

	m.emitEvent(EventMessageReceived, repoPath, from, map[string]any{
		"to":      to,
		"content": content,
	})
}

// RemoveAgent removes an agent from the repo's state.
func (m *Manager) RemoveAgent(repoPath, instanceID string) {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()
	delete(rs.agents, instanceID)
	rs.mu.Unlock()

	m.emitEvent(EventAgentRemoved, repoPath, instanceID, nil)
}

// pruneStale removes agents that haven't updated within staleAgentAge.
// Caller must hold rs.mu write lock.
func pruneStale(rs *repoState) {
	cutoff := time.Now().Add(-staleAgentAge)
	for id, agent := range rs.agents {
		t, err := time.Parse(time.RFC3339, agent.UpdatedAt)
		if err != nil || t.Before(cutoff) {
			delete(rs.agents, id)
		}
	}
}

// fileConflicts returns files claimed by other agents (excluding excludeAgent).
// Caller must hold rs.mu at least read lock.
func fileConflicts(rs *repoState, excludeAgent string) map[string][]string {
	filesToAgents := make(map[string][]string)
	for id, agent := range rs.agents {
		if id == excludeAgent {
			continue
		}
		for _, f := range agent.Files {
			filesToAgents[f] = append(filesToAgents[f], id)
		}
	}
	return filesToAgents
}

// UpdateStatusWithRole sets the agent's feature, files, and optional role.
func (m *Manager) UpdateStatusWithRole(repoPath, instanceID, feature string, files []string, role string) *UpdateStatusResult {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()

	rs.agents[instanceID] = &AgentStatus{
		Feature:   feature,
		Files:     files,
		Role:      role,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	conflicts := fileConflicts(rs, instanceID)
	var warnings []string
	for _, f := range files {
		if agents, ok := conflicts[f]; ok {
			warnings = append(warnings, fmt.Sprintf("%s is also being worked on by: %s", f, strings.Join(agents, ", ")))
		}
	}

	rs.mu.Unlock()

	eventData := map[string]any{
		"feature": feature,
		"files":   files,
	}
	if role != "" {
		eventData["role"] = role
	}
	m.emitEvent(EventStatusChanged, repoPath, instanceID, eventData)

	return &UpdateStatusResult{Conflicts: warnings}
}

// DefineWorkflow creates or replaces a workflow for a repo.
func (m *Manager) DefineWorkflow(repoPath string, tasks []*WorkflowTask) *WorkflowResult {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()

	wfID := fmt.Sprintf("wf-%d", time.Now().UnixMilli())
	// Ensure all tasks start as pending.
	for _, t := range tasks {
		if t.Status == "" {
			t.Status = TaskPending
		}
	}
	rs.workflow = &Workflow{
		ID:    wfID,
		Tasks: tasks,
	}

	rs.mu.Unlock()

	m.emitEvent(EventWorkflowDefined, repoPath, "", map[string]any{
		"workflow_id": wfID,
		"task_count":  len(tasks),
	})

	return &WorkflowResult{WorkflowID: wfID}
}

// GetWorkflow returns the current workflow for a repo, or nil if none exists.
func (m *Manager) GetWorkflow(repoPath string) *Workflow {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.workflow
}

// GetWorkflowTask returns a single task from the workflow by ID.
func (m *Manager) GetWorkflowTask(repoPath, taskID string) *WorkflowTask {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.workflow == nil {
		return nil
	}
	for _, t := range rs.workflow.Tasks {
		if t.ID == taskID {
			return t
		}
	}
	return nil
}

// CompleteTask marks a workflow task as done or failed.
func (m *Manager) CompleteTask(repoPath, taskID string, status TaskStatus, errMsg string) error {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()

	if rs.workflow == nil {
		rs.mu.Unlock()
		return fmt.Errorf("no workflow defined for repo")
	}

	found := false
	for _, t := range rs.workflow.Tasks {
		if t.ID == taskID {
			t.Status = status
			t.Error = errMsg
			found = true
			break
		}
	}

	rs.mu.Unlock()

	if !found {
		return fmt.Errorf("task %q not found in workflow", taskID)
	}

	m.emitEvent(EventTaskCompleted, repoPath, taskID, map[string]any{
		"task_id": taskID,
		"status":  string(status),
	})

	return nil
}

// EvaluateWorkflow checks for pending tasks whose dependencies are all done,
// marks them as running, and returns their IDs.
func (m *Manager) EvaluateWorkflow(repoPath string) []string {
	rs := m.getOrCreateRepo(repoPath)
	rs.mu.Lock()

	if rs.workflow == nil {
		rs.mu.Unlock()
		return nil
	}

	// Build status index.
	statusMap := make(map[string]TaskStatus, len(rs.workflow.Tasks))
	for _, t := range rs.workflow.Tasks {
		statusMap[t.ID] = t.Status
	}

	var triggered []string
	for _, t := range rs.workflow.Tasks {
		if t.Status != TaskPending {
			continue
		}

		// Check all dependencies are done.
		allDone := true
		for _, dep := range t.DependsOn {
			if statusMap[dep] != TaskDone {
				allDone = false
				break
			}
		}
		if allDone {
			t.Status = TaskRunning
			triggered = append(triggered, t.ID)
		}
	}

	rs.mu.Unlock()

	for _, taskID := range triggered {
		m.emitEvent(EventTaskTriggered, repoPath, taskID, map[string]any{
			"task_id": taskID,
		})
	}

	return triggered
}
