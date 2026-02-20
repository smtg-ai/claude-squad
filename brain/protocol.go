package brain

import "encoding/json"

// IPC method constants shared between client and server.
const (
	MethodGetBrain     = "get_brain"
	MethodUpdateStatus = "update_status"
	MethodSendMessage  = "send_message"
	MethodRemoveAgent  = "remove_agent"
	MethodPing         = "ping"

	// Tier 3 methods — relayed to the TUI via action channel.
	MethodCreateInstance  = "create_instance"
	MethodInjectMessage   = "inject_message"
	MethodPauseInstance   = "pause_instance"
	MethodResumeInstance  = "resume_instance"
	MethodKillInstance    = "kill_instance"
	MethodDefineWorkflow  = "define_workflow"
	MethodCompleteTask    = "complete_task"
	MethodGetWorkflow     = "get_workflow"

	// Event subscription methods.
	MethodSubscribe   = "subscribe"
	MethodPollEvents  = "poll_events"
	MethodUnsubscribe = "unsubscribe"
)

// Request is the JSON envelope sent from client to server over the Unix socket.
type Request struct {
	Method     string         `json:"method"`
	InstanceID string         `json:"instance_id"`
	RepoPath   string         `json:"repo_path"`
	Params     map[string]any `json:"params,omitempty"`
}

// Response is the JSON envelope sent from server to client.
type Response struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// AgentStatus tracks what an agent is currently working on.
type AgentStatus struct {
	Feature   string   `json:"feature"`
	Files     []string `json:"files"`
	Role      string   `json:"role,omitempty"`
	UpdatedAt string   `json:"updated_at"`
}

// BrainMessage is a directed message between agents.
type BrainMessage struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// BrainState is the coordination state for a single repository.
type BrainState struct {
	Agents   map[string]*AgentStatus `json:"agents"`
	Messages []BrainMessage          `json:"messages"`
}

// UpdateStatusResult is returned by UpdateStatus with optional conflict warnings.
type UpdateStatusResult struct {
	Conflicts []string `json:"conflicts,omitempty"`
}

// --- Tier 3: Action channel types ---
// These are used for requests that must be relayed from the brain server to the TUI.

// ActionType identifies the kind of action request.
type ActionType string

const (
	ActionCreateInstance ActionType = "create_instance"
	ActionInjectMessage  ActionType = "inject_message"
	ActionPauseInstance  ActionType = "pause_instance"
	ActionResumeInstance ActionType = "resume_instance"
	ActionKillInstance   ActionType = "kill_instance"
)

// ActionRequest is sent from the brain server to the TUI via a channel.
// The server blocks on ResponseCh until the TUI processes the action.
type ActionRequest struct {
	Type       ActionType
	Params     map[string]any
	ResponseCh chan ActionResponse
}

// ActionResponse is sent back from the TUI to the brain server.
type ActionResponse struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data,omitempty"`
	Error string         `json:"error,omitempty"`
}

// CreateInstanceParams holds parameters for spawning a new agent instance.
type CreateInstanceParams struct {
	Title           string `json:"title"`
	Program         string `json:"program,omitempty"`
	Prompt          string `json:"prompt,omitempty"`
	Role            string `json:"role,omitempty"`
	Topic           string `json:"topic,omitempty"`
	SkipPermissions *bool  `json:"skip_permissions,omitempty"` // defaults to true for programmatic creation
}

// CreateInstanceResult is returned after an instance is created.
type CreateInstanceResult struct {
	Title  string `json:"title"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// InjectMessageParams holds parameters for direct terminal injection.
type InjectMessageParams struct {
	To      string `json:"to"`
	Content string `json:"content"`
	Format  string `json:"format,omitempty"` // "plain" or "hivemind" (default)
}

// --- Tier 3: Workflow DAG types ---

// TaskStatus tracks the state of a workflow task.
type TaskStatus string

const (
	TaskPending  TaskStatus = "pending"
	TaskRunning  TaskStatus = "running"
	TaskDone     TaskStatus = "done"
	TaskFailed   TaskStatus = "failed"
)

// WorkflowTask represents a single task in a workflow DAG.
type WorkflowTask struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     TaskStatus `json:"status"`
	DependsOn  []string   `json:"depends_on,omitempty"`
	AssignedTo string     `json:"assigned_to,omitempty"`
	Prompt     string     `json:"prompt,omitempty"`
	Role       string     `json:"role,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// Workflow is a DAG of tasks for a repository.
type Workflow struct {
	ID    string          `json:"id"`
	Tasks []*WorkflowTask `json:"tasks"`
}

// WorkflowResult is returned by workflow operations.
type WorkflowResult struct {
	WorkflowID   string   `json:"workflow_id"`
	Triggered    []string `json:"triggered,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// --- Event subscription types ---

// SubscribeResult is returned by the subscribe method.
type SubscribeResult struct {
	SubscriberID string `json:"subscriber_id"`
}

// PollEventsResult is returned by the poll_events method.
type PollEventsResult struct {
	SubscriberID string  `json:"subscriber_id"`
	Events       []Event `json:"events"`
}
