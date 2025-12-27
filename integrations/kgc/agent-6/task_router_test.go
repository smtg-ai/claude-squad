package agent6

import (
	"encoding/json"
	"testing"
)

// Test XOR routing: exactly one predicate must match
func TestRoute_XOR(t *testing.T) {
	tests := []struct {
		name        string
		predicates  []Predicate
		task        *Task
		expectMatch bool
		expectError bool
	}{
		{
			name: "XOR: one true - should match",
			predicates: []Predicate{
				func(t *Task) bool { return false },
				func(t *Task) bool { return true },
				func(t *Task) bool { return false },
			},
			task:        &Task{ID: "task1", Type: "build"},
			expectMatch: true,
			expectError: false,
		},
		{
			name: "XOR: all false - should not match",
			predicates: []Predicate{
				func(t *Task) bool { return false },
				func(t *Task) bool { return false },
			},
			task:        &Task{ID: "task2", Type: "test"},
			expectMatch: false,
			expectError: false,
		},
		{
			name: "XOR: two true - should not match",
			predicates: []Predicate{
				func(t *Task) bool { return true },
				func(t *Task) bool { return true },
			},
			task:        &Task{ID: "task3", Type: "deploy"},
			expectMatch: false,
			expectError: false,
		},
		{
			name: "XOR: all true - should not match",
			predicates: []Predicate{
				func(t *Task) bool { return true },
				func(t *Task) bool { return true },
				func(t *Task) bool { return true },
			},
			task:        &Task{ID: "task4", Type: "clean"},
			expectMatch: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := evaluateRoute(tt.task, tt.predicates, XOR)
			if (err != nil) != tt.expectError {
				t.Errorf("evaluateRoute() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if matched != tt.expectMatch {
				t.Errorf("evaluateRoute() matched = %v, expectMatch %v", matched, tt.expectMatch)
			}
		})
	}
}

// Test AND routing: all predicates must match
func TestRoute_AND(t *testing.T) {
	tests := []struct {
		name        string
		predicates  []Predicate
		task        *Task
		expectMatch bool
	}{
		{
			name: "AND: all true - should match",
			predicates: []Predicate{
				func(t *Task) bool { return true },
				func(t *Task) bool { return true },
				func(t *Task) bool { return true },
			},
			task:        &Task{ID: "task1", Type: "build", Priority: 10},
			expectMatch: true,
		},
		{
			name: "AND: one false - should not match",
			predicates: []Predicate{
				func(t *Task) bool { return true },
				func(t *Task) bool { return false },
				func(t *Task) bool { return true },
			},
			task:        &Task{ID: "task2", Type: "test"},
			expectMatch: false,
		},
		{
			name: "AND: all false - should not match",
			predicates: []Predicate{
				func(t *Task) bool { return false },
				func(t *Task) bool { return false },
			},
			task:        &Task{ID: "task3", Type: "deploy"},
			expectMatch: false,
		},
		{
			name: "AND: complex conditions",
			predicates: []Predicate{
				func(t *Task) bool { return t.Type == "build" },
				func(t *Task) bool { return t.Priority > 5 },
				func(t *Task) bool { return t.ID != "" },
			},
			task:        &Task{ID: "task4", Type: "build", Priority: 10},
			expectMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := evaluateRoute(tt.task, tt.predicates, AND)
			if err != nil {
				t.Errorf("evaluateRoute() unexpected error: %v", err)
				return
			}
			if matched != tt.expectMatch {
				t.Errorf("evaluateRoute() matched = %v, expectMatch %v", matched, tt.expectMatch)
			}
		})
	}
}

// Test OR routing: at least one predicate must match
func TestRoute_OR(t *testing.T) {
	tests := []struct {
		name        string
		predicates  []Predicate
		task        *Task
		expectMatch bool
	}{
		{
			name: "OR: one true - should match",
			predicates: []Predicate{
				func(t *Task) bool { return false },
				func(t *Task) bool { return true },
				func(t *Task) bool { return false },
			},
			task:        &Task{ID: "task1", Type: "build"},
			expectMatch: true,
		},
		{
			name: "OR: all true - should match",
			predicates: []Predicate{
				func(t *Task) bool { return true },
				func(t *Task) bool { return true },
			},
			task:        &Task{ID: "task2", Type: "test"},
			expectMatch: true,
		},
		{
			name: "OR: all false - should not match",
			predicates: []Predicate{
				func(t *Task) bool { return false },
				func(t *Task) bool { return false },
				func(t *Task) bool { return false },
			},
			task:        &Task{ID: "task3", Type: "deploy"},
			expectMatch: false,
		},
		{
			name: "OR: complex conditions",
			predicates: []Predicate{
				func(t *Task) bool { return t.Type == "test" },
				func(t *Task) bool { return t.Priority > 100 },
			},
			task:        &Task{ID: "task4", Type: "build", Priority: 5},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := evaluateRoute(tt.task, tt.predicates, OR)
			if err != nil {
				t.Errorf("evaluateRoute() unexpected error: %v", err)
				return
			}
			if matched != tt.expectMatch {
				t.Errorf("evaluateRoute() matched = %v, expectMatch %v", matched, tt.expectMatch)
			}
		})
	}
}

// Test determinism: same task → same route across runs
func TestRoute_Determinism(t *testing.T) {
	task := &Task{
		ID:       "determinism-test",
		Type:     "build",
		Priority: 10,
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	routes := []Route{
		{
			Name: "build-route",
			Predicates: []Predicate{
				func(t *Task) bool { return t.Type == "build" },
			},
			Combinator:  AND,
			TargetAgent: "agent-1",
		},
	}

	router := NewRouter(routes)

	// Run routing 1000 times and verify identical results
	var results []string
	for i := 0; i < 1000; i++ {
		result, err := router.Route(task, nil)
		if err != nil {
			t.Fatalf("Run %d: routing failed: %v", i, err)
		}
		results = append(results, result)
	}

	// Verify all results are identical
	firstResult := results[0]
	for i, result := range results {
		if result != firstResult {
			t.Errorf("Run %d: non-deterministic result: got %s, expected %s", i, result, firstResult)
		}
	}
}

// Test replay: ReplayRoute(same inputs) == Route(same inputs)
func TestReplayRoute(t *testing.T) {
	task := &Task{
		ID:       "replay-test",
		Type:     "test",
		Priority: 5,
		Metadata: map[string]interface{}{
			"env": "staging",
		},
	}

	predicates := []Predicate{
		func(t *Task) bool { return t.Type == "test" },
	}

	routes := []Route{
		{
			Name:        "test-route",
			Predicates:  predicates,
			Combinator:  AND,
			TargetAgent: "agent-2",
		},
	}

	router := NewRouter(routes)

	// Original routing
	originalResult, err := router.Route(task, nil)
	if err != nil {
		t.Fatalf("Original routing failed: %v", err)
	}

	// Generate replay script
	script, err := GenerateReplayScript(task, originalResult)
	if err != nil {
		t.Fatalf("Failed to generate replay script: %v", err)
	}

	// Replay routing
	replayResult, err := router.ReplayRoute(task, predicates, script)
	if err != nil {
		t.Fatalf("Replay routing failed: %v", err)
	}

	// Verify results match
	if replayResult != originalResult {
		t.Errorf("Replay mismatch: got %s, expected %s", replayResult, originalResult)
	}

	// Verify script is serializable
	scriptJSON, err := MarshalReplayScript(script)
	if err != nil {
		t.Fatalf("Failed to marshal replay script: %v", err)
	}

	// Verify we can unmarshal
	unmarshaled, err := UnmarshalReplayScript(scriptJSON)
	if err != nil {
		t.Fatalf("Failed to unmarshal replay script: %v", err)
	}

	if unmarshaled.RoutedToAgent != script.RoutedToAgent {
		t.Errorf("Unmarshal mismatch: got %s, expected %s",
			unmarshaled.RoutedToAgent, script.RoutedToAgent)
	}
}

// Test task graph topological sort
func TestEvaluateTaskGraph(t *testing.T) {
	tests := []struct {
		name         string
		tasks        []*Task
		dependencies map[string][]string
		expectError  bool
		validateFunc func(t *testing.T, order []string)
	}{
		{
			name: "simple linear dependency",
			tasks: []*Task{
				{ID: "task1"},
				{ID: "task2"},
				{ID: "task3"},
			},
			dependencies: map[string][]string{
				"task2": {"task1"},
				"task3": {"task2"},
			},
			expectError: false,
			validateFunc: func(t *testing.T, order []string) {
				// task1 must come before task2, task2 before task3
				if len(order) != 3 {
					t.Errorf("Expected 3 tasks, got %d", len(order))
				}
				task1Idx := indexOf(order, "task1")
				task2Idx := indexOf(order, "task2")
				task3Idx := indexOf(order, "task3")
				if task1Idx >= task2Idx || task2Idx >= task3Idx {
					t.Errorf("Invalid order: %v", order)
				}
			},
		},
		{
			name: "parallel tasks",
			tasks: []*Task{
				{ID: "task1"},
				{ID: "task2"},
				{ID: "task3"},
			},
			dependencies: map[string][]string{},
			expectError:  false,
			validateFunc: func(t *testing.T, order []string) {
				if len(order) != 3 {
					t.Errorf("Expected 3 tasks, got %d", len(order))
				}
				// All tasks should be present (order may vary but should be deterministic)
				if !contains(order, "task1") || !contains(order, "task2") || !contains(order, "task3") {
					t.Errorf("Missing tasks in order: %v", order)
				}
			},
		},
		{
			name: "diamond dependency",
			tasks: []*Task{
				{ID: "A"},
				{ID: "B"},
				{ID: "C"},
				{ID: "D"},
			},
			dependencies: map[string][]string{
				"B": {"A"},
				"C": {"A"},
				"D": {"B", "C"},
			},
			expectError: false,
			validateFunc: func(t *testing.T, order []string) {
				aIdx := indexOf(order, "A")
				bIdx := indexOf(order, "B")
				cIdx := indexOf(order, "C")
				dIdx := indexOf(order, "D")
				// A must come before B, C, and D
				if aIdx >= bIdx || aIdx >= cIdx || aIdx >= dIdx {
					t.Errorf("A should come before B, C, D: %v", order)
				}
				// B and C must come before D
				if bIdx >= dIdx || cIdx >= dIdx {
					t.Errorf("B and C should come before D: %v", order)
				}
			},
		},
		{
			name: "empty graph",
			tasks: []*Task{},
			dependencies: map[string][]string{},
			expectError: false,
			validateFunc: func(t *testing.T, order []string) {
				if len(order) != 0 {
					t.Errorf("Expected empty order, got %v", order)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := NewTaskGraph(tt.tasks, tt.dependencies)
			router := NewRouter(nil)

			order, err := router.EvaluateTaskGraph(graph)
			if (err != nil) != tt.expectError {
				t.Errorf("EvaluateTaskGraph() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError && tt.validateFunc != nil {
				tt.validateFunc(t, order)
			}
		})
	}
}

// Test cycle detection
func TestEvaluateTaskGraph_CycleDetection(t *testing.T) {
	tests := []struct {
		name         string
		tasks        []*Task
		dependencies map[string][]string
		expectCycle  bool
	}{
		{
			name: "simple cycle: A -> B -> A",
			tasks: []*Task{
				{ID: "A"},
				{ID: "B"},
			},
			dependencies: map[string][]string{
				"B": {"A"},
				"A": {"B"},
			},
			expectCycle: true,
		},
		{
			name: "three-node cycle: A -> B -> C -> A",
			tasks: []*Task{
				{ID: "A"},
				{ID: "B"},
				{ID: "C"},
			},
			dependencies: map[string][]string{
				"B": {"A"},
				"C": {"B"},
				"A": {"C"},
			},
			expectCycle: true,
		},
		{
			name: "self-loop: A -> A",
			tasks: []*Task{
				{ID: "A"},
			},
			dependencies: map[string][]string{
				"A": {"A"},
			},
			expectCycle: true,
		},
		{
			name: "no cycle",
			tasks: []*Task{
				{ID: "A"},
				{ID: "B"},
				{ID: "C"},
			},
			dependencies: map[string][]string{
				"B": {"A"},
				"C": {"A"},
			},
			expectCycle: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph := NewTaskGraph(tt.tasks, tt.dependencies)
			router := NewRouter(nil)

			_, err := router.EvaluateTaskGraph(graph)
			hasCycle := err != nil && err.Error() != ""

			if tt.expectCycle && !hasCycle {
				t.Errorf("Expected cycle to be detected, but got no error")
			}
			if !tt.expectCycle && hasCycle {
				t.Errorf("Expected no cycle, but got error: %v", err)
			}
		})
	}
}

// Test hash stability
func TestHashTask_Stability(t *testing.T) {
	task := &Task{
		ID:       "hash-test",
		Type:     "build",
		Priority: 10,
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	// Hash the same task 100 times
	var hashes []string
	for i := 0; i < 100; i++ {
		hash, err := hashTask(task)
		if err != nil {
			t.Fatalf("Hash %d failed: %v", i, err)
		}
		hashes = append(hashes, hash)
	}

	// Verify all hashes are identical
	firstHash := hashes[0]
	for i, hash := range hashes {
		if hash != firstHash {
			t.Errorf("Hash %d mismatch: got %s, expected %s", i, hash, firstHash)
		}
	}
}

// Test no route matched error
func TestRoute_NoMatch(t *testing.T) {
	task := &Task{ID: "nomatch", Type: "unknown"}

	routes := []Route{
		{
			Name: "build-route",
			Predicates: []Predicate{
				func(t *Task) bool { return t.Type == "build" },
			},
			Combinator:  AND,
			TargetAgent: "agent-1",
		},
	}

	router := NewRouter(routes)
	_, err := router.Route(task, nil)

	if err == nil {
		t.Error("Expected error for no matching route, got nil")
	}
}

// Test invalid task
func TestRoute_InvalidTask(t *testing.T) {
	router := NewRouter(nil)

	// Nil task
	_, err := router.Route(nil, nil)
	if err == nil {
		t.Error("Expected error for nil task")
	}

	// Empty task ID
	_, err = router.Route(&Task{ID: ""}, nil)
	if err == nil {
		t.Error("Expected error for empty task ID")
	}
}

// Test task graph validation
func TestValidateTaskGraph(t *testing.T) {
	tests := []struct {
		name        string
		graph       *TaskGraph
		expectError bool
	}{
		{
			name: "valid graph",
			graph: NewTaskGraph(
				[]*Task{{ID: "A"}, {ID: "B"}},
				map[string][]string{"B": {"A"}},
			),
			expectError: false,
		},
		{
			name:        "nil graph",
			graph:       nil,
			expectError: true,
		},
		{
			name: "duplicate task IDs",
			graph: &TaskGraph{
				Tasks: []*Task{{ID: "A"}, {ID: "A"}},
			},
			expectError: true,
		},
		{
			name: "unknown dependency",
			graph: NewTaskGraph(
				[]*Task{{ID: "A"}},
				map[string][]string{"A": {"B"}},
			),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaskGraph(tt.graph)
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateTaskGraph() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

// Test replay script serialization
func TestReplayScript_Serialization(t *testing.T) {
	task := &Task{
		ID:       "serial-test",
		Type:     "deploy",
		Priority: 8,
		Metadata: map[string]interface{}{
			"region": "us-west",
		},
	}

	script, err := GenerateReplayScript(task, "agent-3")
	if err != nil {
		t.Fatalf("Failed to generate replay script: %v", err)
	}

	// Serialize to JSON
	data, err := MarshalReplayScript(script)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify JSON is valid
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Deserialize
	script2, err := UnmarshalReplayScript(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify fields match
	if script2.TaskID != script.TaskID {
		t.Errorf("TaskID mismatch: got %s, expected %s", script2.TaskID, script.TaskID)
	}
	if script2.RoutedToAgent != script.RoutedToAgent {
		t.Errorf("RoutedToAgent mismatch: got %s, expected %s",
			script2.RoutedToAgent, script.RoutedToAgent)
	}
}

// Helper functions

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

func contains(slice []string, item string) bool {
	return indexOf(slice, item) >= 0
}
