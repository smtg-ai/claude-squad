package orchestrator

import (
	"testing"
)

func TestParsePlanOutput(t *testing.T) {
	output := `I'll analyze this goal and break it down into tasks.

TASK: task1 | Implement feature 1 with details about what to do
TASK: task2 | Implement feature 2 with another set of instructions
TASK: task3 | Implement feature 3 with some more instructions

Let me know if you need any adjustments to this plan.`

	tasks := parsePlanOutput(output, "default prompt")

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
	emptyTasks := parsePlanOutput("", "default prompt")
	if len(emptyTasks) != 0 {
		t.Errorf("Expected 0 tasks from empty output, got %d", len(emptyTasks))
	}
}
