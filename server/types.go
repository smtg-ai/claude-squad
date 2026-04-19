// Package server exposes claude-squad's session model as an HTTP +
// Server-Sent-Events API so an external orchestrator (e.g. Paperclip)
// can drive it without the TUI.
//
// Fork-only — does not exist in upstream smtg-ai/claude-squad.
package server

import "time"

// InstanceDTO is the JSON-serializable representation of a session
// returned by the HTTP API. The in-memory session.Instance type from
// the parent package is not marshaled directly because it contains
// unexported live fields (tmuxSession, gitWorktree).
type InstanceDTO struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Status         string    `json:"status"`
	Program        string    `json:"program"`
	Branch         string    `json:"branch"`
	RepoPath       string    `json:"repoPath,omitempty"`
	WorktreePath   string    `json:"worktreePath,omitempty"`
	AutoYes        bool      `json:"autoYes"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Paused         bool      `json:"paused"`
	TmuxAlive      bool      `json:"tmuxAlive"`
	DiffAdded      int       `json:"diffAdded"`
	DiffRemoved    int       `json:"diffRemoved"`
	TraceID        string    `json:"traceId,omitempty"`
	ParentSpanID   string    `json:"parentSpanId,omitempty"`
	InitialPrompt  string    `json:"initialPrompt,omitempty"`
}

// CreateInstanceRequest is the body accepted by POST /v1/instances.
type CreateInstanceRequest struct {
	Title   string `json:"title"`
	Program string `json:"program,omitempty"`
	Branch  string `json:"branch,omitempty"`
	AutoYes bool   `json:"autoYes,omitempty"`
	// Prompt is sent to the program's input once the session is live.
	// Agent harnesses typically use this to deliver the task.
	Prompt string `json:"prompt,omitempty"`
	// WorkspaceBasePath overrides the default (current working directory
	// of the server). Must be inside a git repo.
	WorkspaceBasePath string `json:"workspaceBasePath,omitempty"`
}

// InputRequest is the body accepted by POST /v1/instances/:id/input.
type InputRequest struct {
	// Prompt, if set, is sent as text plus a terminating Enter.
	Prompt string `json:"prompt,omitempty"`
	// Keys, if set, is sent as raw keystrokes (Prompt takes precedence).
	Keys string `json:"keys,omitempty"`
	// TapEnter, if true, sends a single Enter and nothing else.
	TapEnter bool `json:"tapEnter,omitempty"`
}

// PaneResponse is the body returned by GET /v1/instances/:id/pane.
type PaneResponse struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	WithAnsi bool   `json:"withAnsi"`
}

// DiffResponse is the body returned by GET /v1/instances/:id/diff.
type DiffResponse struct {
	ID      string `json:"id"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Content string `json:"content"`
}

// HealthResponse is the body returned by GET /v1/health.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
	Version string `json:"version"`
}

// ErrorResponse is returned for any non-2xx code.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Event is a lifecycle record broadcast on /v1/events (SSE stream).
// The schema is intended to be wire-stable — Paperclip's harness code
// consumes these verbatim.
type Event struct {
	Seq        int64             `json:"seq"`
	Type       string            `json:"type"`
	InstanceID string            `json:"instanceId,omitempty"`
	Timestamp  time.Time         `json:"ts"`
	Data       map[string]string `json:"data,omitempty"`
}

// Known event type names. Listed here so consumers can depend on them.
const (
	EventInstanceCreated         = "instance.created"
	EventInstanceStarted         = "instance.started"
	EventInstanceStatusChanged   = "instance.status_changed"
	EventInstanceDiffUpdated     = "instance.diff_updated"
	EventInstancePaneAppended    = "instance.pane_appended"
	EventInstancePromptDetected  = "instance.prompt_detected"
	EventInstancePromptAutoAck   = "instance.prompt_auto_accepted"
	EventInstancePaused          = "instance.paused"
	EventInstanceResumed         = "instance.resumed"
	EventInstanceKilled          = "instance.killed"
	EventInstanceExited          = "instance.exited"
)
