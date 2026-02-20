package brain

import (
	"encoding/json"
	"testing"
)

func TestServerActionChannel(t *testing.T) {
	srv := startTestServer(t)

	// Start a goroutine to handle actions from the TUI side.
	go func() {
		for action := range srv.Actions() {
			switch action.Type {
			case ActionCreateInstance:
				title, _ := action.Params["title"].(string)
				action.ResponseCh <- ActionResponse{
					OK: true,
					Data: map[string]any{
						"title":  title,
						"status": "created",
					},
				}
			case ActionKillInstance:
				action.ResponseCh <- ActionResponse{OK: true}
			default:
				action.ResponseCh <- ActionResponse{Error: "unhandled action"}
			}
		}
	}()

	client := NewClient(srv.SocketPath())

	// Test create_instance via socket.
	result, err := client.CreateInstance("/repo", "agent-1", CreateInstanceParams{
		Title:   "new-agent",
		Program: "claude",
		Prompt:  "implement auth",
		Role:    "coder",
	})
	if err != nil {
		t.Fatalf("CreateInstance error: %v", err)
	}
	if result.Title != "new-agent" {
		t.Errorf("Title = %q, want %q", result.Title, "new-agent")
	}
	if result.Status != "created" {
		t.Errorf("Status = %q, want %q", result.Status, "created")
	}

	// Test kill_instance via socket.
	err = client.KillInstance("/repo", "agent-1", "new-agent")
	if err != nil {
		t.Fatalf("KillInstance error: %v", err)
	}
}

func TestServerInjectMessageAlsoStoresInBrain(t *testing.T) {
	srv := startTestServer(t)

	// Handle the inject_message action.
	go func() {
		for action := range srv.Actions() {
			action.ResponseCh <- ActionResponse{OK: true}
		}
	}()

	client := NewClient(srv.SocketPath())

	// First register agents.
	client.UpdateStatus("/repo", "agent-1", "work", nil)
	client.UpdateStatus("/repo", "agent-2", "work", nil)

	// Inject a message.
	err := client.InjectMessage("/repo", "agent-1", InjectMessageParams{
		To:      "agent-2",
		Content: "stop working on auth.go",
	})
	if err != nil {
		t.Fatalf("InjectMessage error: %v", err)
	}

	// Verify the message is stored in brain state too.
	state, err := client.GetBrain("/repo", "agent-2")
	if err != nil {
		t.Fatalf("GetBrain error: %v", err)
	}
	if len(state.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "stop working on auth.go" {
		t.Errorf("message content = %q, want %q", state.Messages[0].Content, "stop working on auth.go")
	}
}

func TestServerDefineWorkflowAndComplete(t *testing.T) {
	srv := startTestServer(t)

	// Handle create_instance actions triggered by workflow evaluation.
	go func() {
		for action := range srv.Actions() {
			action.ResponseCh <- ActionResponse{OK: true}
		}
	}()

	client := NewClient(srv.SocketPath())

	// Define a workflow.
	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "implement feature"},
		{ID: "task-2", Title: "write tests"},
		{ID: "task-3", Title: "review", DependsOn: []string{"task-1", "task-2"}},
	}
	result, err := client.DefineWorkflow("/repo", "architect", tasks)
	if err != nil {
		t.Fatalf("DefineWorkflow error: %v", err)
	}
	if result.WorkflowID == "" {
		t.Error("expected non-empty workflow ID")
	}
	// task-1 and task-2 should be triggered immediately (no deps).
	if len(result.Triggered) != 2 {
		t.Errorf("expected 2 triggered, got %d: %v", len(result.Triggered), result.Triggered)
	}

	// Get the workflow state.
	wf, err := client.GetWorkflow("/repo", "architect")
	if err != nil {
		t.Fatalf("GetWorkflow error: %v", err)
	}
	if len(wf.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(wf.Tasks))
	}

	// Complete task-1 and task-2.
	completeResult, err := client.CompleteTask("/repo", "agent-1", "task-1", "done", "")
	if err != nil {
		t.Fatalf("CompleteTask error: %v", err)
	}
	// task-3 still blocked (task-2 not done).
	if len(completeResult.Triggered) != 0 {
		t.Errorf("expected 0 triggered (task-2 not done), got %d", len(completeResult.Triggered))
	}

	completeResult, err = client.CompleteTask("/repo", "agent-2", "task-2", "done", "")
	if err != nil {
		t.Fatalf("CompleteTask error: %v", err)
	}
	// Now task-3 should be triggered.
	if len(completeResult.Triggered) != 1 {
		t.Errorf("expected 1 triggered, got %d: %v", len(completeResult.Triggered), completeResult.Triggered)
	}
}

func TestServerGetWorkflowEmpty(t *testing.T) {
	srv := startTestServer(t)

	client := NewClient(srv.SocketPath())

	wf, err := client.GetWorkflow("/repo", "agent-1")
	if err != nil {
		t.Fatalf("GetWorkflow error: %v", err)
	}
	if len(wf.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(wf.Tasks))
	}
}

func TestClientUpdateStatusWithRole(t *testing.T) {
	srv := startTestServer(t)

	client := NewClient(srv.SocketPath())

	_, err := client.UpdateStatusWithRole("/repo", "agent-1", "implement auth", []string{"auth.go"}, "coder")
	if err != nil {
		t.Fatalf("UpdateStatusWithRole error: %v", err)
	}

	state, err := client.GetBrain("/repo", "agent-1")
	if err != nil {
		t.Fatalf("GetBrain error: %v", err)
	}
	agent := state.Agents["agent-1"]
	if agent.Role != "coder" {
		t.Errorf("Role = %q, want %q", agent.Role, "coder")
	}
}

// Ensure json.RawMessage can decode the response data for new types.
func TestCreateInstanceResultJSON(t *testing.T) {
	result := CreateInstanceResult{
		Title:  "test-agent",
		Status: "created",
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CreateInstanceResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Title != "test-agent" {
		t.Errorf("Title = %q, want %q", decoded.Title, "test-agent")
	}
}
