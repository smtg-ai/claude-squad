package brain

import (
	"testing"
)

func TestManagerDefineWorkflow(t *testing.T) {
	m := NewManager()

	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "implement feature"},
		{ID: "task-2", Title: "write tests", DependsOn: []string{"task-1"}},
		{ID: "task-3", Title: "code review", DependsOn: []string{"task-1", "task-2"}},
	}

	result := m.DefineWorkflow("/repo", tasks)
	if result.WorkflowID == "" {
		t.Error("expected non-empty workflow ID")
	}

	wf := m.GetWorkflow("/repo")
	if wf == nil {
		t.Fatal("expected workflow, got nil")
	}
	if len(wf.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(wf.Tasks))
	}
	for _, task := range wf.Tasks {
		if task.Status != TaskPending {
			t.Errorf("task %s: expected pending, got %s", task.ID, task.Status)
		}
	}
}

func TestManagerEvaluateWorkflow(t *testing.T) {
	m := NewManager()

	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "implement feature"},
		{ID: "task-2", Title: "write tests"},
		{ID: "task-3", Title: "code review", DependsOn: []string{"task-1", "task-2"}},
	}

	m.DefineWorkflow("/repo", tasks)

	// First evaluation: task-1 and task-2 have no deps, should be triggered.
	triggered := m.EvaluateWorkflow("/repo")
	if len(triggered) != 2 {
		t.Fatalf("expected 2 triggered tasks, got %d: %v", len(triggered), triggered)
	}

	// Verify they are now running.
	for _, tid := range triggered {
		task := m.GetWorkflowTask("/repo", tid)
		if task.Status != TaskRunning {
			t.Errorf("task %s: expected running, got %s", tid, task.Status)
		}
	}

	// task-3 should still be pending (deps not done).
	task3 := m.GetWorkflowTask("/repo", "task-3")
	if task3.Status != TaskPending {
		t.Errorf("task-3: expected pending, got %s", task3.Status)
	}

	// Complete task-1 and task-2.
	m.CompleteTask("/repo", "task-1", TaskDone, "")
	m.CompleteTask("/repo", "task-2", TaskDone, "")

	// Second evaluation: task-3 should now be triggered.
	triggered = m.EvaluateWorkflow("/repo")
	if len(triggered) != 1 {
		t.Fatalf("expected 1 triggered task, got %d: %v", len(triggered), triggered)
	}
	if triggered[0] != "task-3" {
		t.Errorf("expected task-3, got %s", triggered[0])
	}
}

func TestManagerCompleteTask(t *testing.T) {
	m := NewManager()

	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "work"},
	}
	m.DefineWorkflow("/repo", tasks)

	// Complete with success.
	err := m.CompleteTask("/repo", "task-1", TaskDone, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := m.GetWorkflowTask("/repo", "task-1")
	if task.Status != TaskDone {
		t.Errorf("expected done, got %s", task.Status)
	}
}

func TestManagerCompleteTaskFailed(t *testing.T) {
	m := NewManager()

	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "work"},
		{ID: "task-2", Title: "depends on 1", DependsOn: []string{"task-1"}},
	}
	m.DefineWorkflow("/repo", tasks)

	// Fail task-1.
	err := m.CompleteTask("/repo", "task-1", TaskFailed, "compilation error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := m.GetWorkflowTask("/repo", "task-1")
	if task.Status != TaskFailed {
		t.Errorf("expected failed, got %s", task.Status)
	}
	if task.Error != "compilation error" {
		t.Errorf("expected error message, got %q", task.Error)
	}

	// task-2 should NOT be triggered (dependency failed, not done).
	triggered := m.EvaluateWorkflow("/repo")
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered tasks (dep failed), got %d: %v", len(triggered), triggered)
	}
}

func TestManagerCompleteTaskNotFound(t *testing.T) {
	m := NewManager()

	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "work"},
	}
	m.DefineWorkflow("/repo", tasks)

	err := m.CompleteTask("/repo", "nonexistent", TaskDone, "")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestManagerCompleteTaskNoWorkflow(t *testing.T) {
	m := NewManager()

	err := m.CompleteTask("/repo", "task-1", TaskDone, "")
	if err == nil {
		t.Error("expected error when no workflow defined")
	}
}

func TestManagerGetWorkflowNil(t *testing.T) {
	m := NewManager()

	wf := m.GetWorkflow("/repo")
	if wf != nil {
		t.Errorf("expected nil workflow, got %v", wf)
	}
}

func TestManagerGetWorkflowTask(t *testing.T) {
	m := NewManager()

	tasks := []*WorkflowTask{
		{ID: "task-1", Title: "work", Prompt: "do the thing", Role: "coder"},
	}
	m.DefineWorkflow("/repo", tasks)

	task := m.GetWorkflowTask("/repo", "task-1")
	if task == nil {
		t.Fatal("expected task, got nil")
	}
	if task.Title != "work" {
		t.Errorf("Title = %q, want %q", task.Title, "work")
	}
	if task.Prompt != "do the thing" {
		t.Errorf("Prompt = %q, want %q", task.Prompt, "do the thing")
	}
	if task.Role != "coder" {
		t.Errorf("Role = %q, want %q", task.Role, "coder")
	}

	// Nonexistent task.
	if m.GetWorkflowTask("/repo", "nonexistent") != nil {
		t.Error("expected nil for nonexistent task")
	}
}

func TestManagerUpdateStatusWithRole(t *testing.T) {
	m := NewManager()

	result := m.UpdateStatusWithRole("/repo", "agent-1", "implement auth", []string{"auth.go"}, "coder")
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %v", result.Conflicts)
	}

	state := m.GetBrain("/repo", "agent-1")
	agent := state.Agents["agent-1"]
	if agent.Role != "coder" {
		t.Errorf("Role = %q, want %q", agent.Role, "coder")
	}
	if agent.Feature != "implement auth" {
		t.Errorf("Feature = %q, want %q", agent.Feature, "implement auth")
	}
}

func TestManagerWorkflowRepoIsolation(t *testing.T) {
	m := NewManager()

	m.DefineWorkflow("/repo-a", []*WorkflowTask{{ID: "task-a", Title: "work a"}})
	m.DefineWorkflow("/repo-b", []*WorkflowTask{{ID: "task-b", Title: "work b"}})

	wfA := m.GetWorkflow("/repo-a")
	wfB := m.GetWorkflow("/repo-b")

	if wfA.Tasks[0].ID != "task-a" {
		t.Errorf("repo-a should have task-a, got %s", wfA.Tasks[0].ID)
	}
	if wfB.Tasks[0].ID != "task-b" {
		t.Errorf("repo-b should have task-b, got %s", wfB.Tasks[0].ID)
	}
}
