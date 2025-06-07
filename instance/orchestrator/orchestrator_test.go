package orchestrator

import (
	"testing"
)

func TestParsePlanOutput(t *testing.T) {
	output := `I'll analyze this goal and break it down into tasks.

<TASK-1>
task1 | Implement feature 1 with details about what to do
</TASK-1>

<TASK-2>
task2 | Implement feature 2 with another set of instructions
</TASK-2>

<TASK-3>
task3 | Implement feature 3 with some more instructions
</TASK-3>

Let me know if you need any adjustments to this plan.`

	tasks := parsePlanOutput(output)

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	expectedNames := []string{"task1", "task2", "task3"}
	for i, task := range tasks {
		if task.Name != expectedNames[i] {
			t.Errorf("Expected task name %s, got %s", expectedNames[i], task.Name)
		}
	}

	// Test with empty output
	emptyTasks := parsePlanOutput("")
	if len(emptyTasks) != 0 {
		t.Errorf("Expected 0 tasks from empty output, got %d", len(emptyTasks))
	}
}
