package agent4

import (
	"reflect"
	"testing"
)

// TestAllocateResources_Basic verifies basic resource allocation.
func TestAllocateResources_Basic(t *testing.T) {
	tests := []struct {
		name        string
		agentCount  int
		budget      int
		wantErr     bool
		checkResult func(*testing.T, *Allocation)
	}{
		{
			name:       "even distribution",
			agentCount: 4,
			budget:     100,
			wantErr:    false,
			checkResult: func(t *testing.T, a *Allocation) {
				if a.Remaining != 0 {
					t.Errorf("expected remaining=0, got %d", a.Remaining)
				}
				total := 0
				for _, v := range a.Assignments {
					total += v
				}
				if total != 100 {
					t.Errorf("expected total allocation=100, got %d", total)
				}
			},
		},
		{
			name:       "uneven distribution with remainder",
			agentCount: 3,
			budget:     10,
			wantErr:    false,
			checkResult: func(t *testing.T, a *Allocation) {
				// 10 / 3 = 3 remainder 1
				// First agent gets 4, others get 3
				expected := map[string]int{
					"agent-0": 4,
					"agent-1": 3,
					"agent-2": 3,
				}
				if !reflect.DeepEqual(a.Assignments, expected) {
					t.Errorf("expected assignments=%v, got %v", expected, a.Assignments)
				}
			},
		},
		{
			name:       "zero resources",
			agentCount: 5,
			budget:     0,
			wantErr:    false,
			checkResult: func(t *testing.T, a *Allocation) {
				for agentID, alloc := range a.Assignments {
					if alloc != 0 {
						t.Errorf("expected agent %s to have 0 resources, got %d", agentID, alloc)
					}
				}
			},
		},
		{
			name:       "single agent",
			agentCount: 1,
			budget:     100,
			wantErr:    false,
			checkResult: func(t *testing.T, a *Allocation) {
				if a.Assignments["agent-0"] != 100 {
					t.Errorf("expected agent-0 to get all 100 resources, got %d", a.Assignments["agent-0"])
				}
			},
		},
		{
			name:       "invalid agent count (zero)",
			agentCount: 0,
			budget:     100,
			wantErr:    true,
		},
		{
			name:       "invalid agent count (negative)",
			agentCount: -5,
			budget:     100,
			wantErr:    true,
		},
		{
			name:       "negative budget",
			agentCount: 5,
			budget:     -10,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc, err := AllocateResources(tt.agentCount, tt.budget)
			if (err != nil) != tt.wantErr {
				t.Errorf("AllocateResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, alloc)
			}
		})
	}
}

// TestAllocateResources_Determinism verifies that allocation is deterministic.
func TestAllocateResources_Determinism(t *testing.T) {
	agentCount := 7
	budget := 100

	// Run allocation 10 times
	results := make([]*Allocation, 10)
	for i := 0; i < 10; i++ {
		alloc, err := AllocateResources(agentCount, budget)
		if err != nil {
			t.Fatalf("unexpected error on run %d: %v", i, err)
		}
		results[i] = alloc
	}

	// Verify all results are identical
	first := results[0]
	for i := 1; i < len(results); i++ {
		if !reflect.DeepEqual(first.Assignments, results[i].Assignments) {
			t.Errorf("run %d produced different assignments: want %v, got %v",
				i, first.Assignments, results[i].Assignments)
		}
		if first.Remaining != results[i].Remaining {
			t.Errorf("run %d produced different remaining: want %d, got %d",
				i, first.Remaining, results[i].Remaining)
		}
	}
}

// TestAllocateResources_Fairness verifies fairness property.
func TestAllocateResources_Fairness(t *testing.T) {
	agentCount := 10
	budget := 103 // Intentionally not evenly divisible

	alloc, err := AllocateResources(agentCount, budget)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find min and max allocations
	minAlloc, maxAlloc := alloc.Assignments["agent-0"], alloc.Assignments["agent-0"]
	for _, v := range alloc.Assignments {
		if v < minAlloc {
			minAlloc = v
		}
		if v > maxAlloc {
			maxAlloc = v
		}
	}

	// Fairness: max - min ≤ 1
	if maxAlloc-minAlloc > 1 {
		t.Errorf("fairness violation: max allocation=%d, min allocation=%d, diff=%d (should be ≤ 1)",
			maxAlloc, minAlloc, maxAlloc-minAlloc)
	}
}

// TestRoundRobinSchedule_Basic verifies basic round-robin scheduling.
func TestRoundRobinSchedule_Basic(t *testing.T) {
	agents := []Agent{
		{ID: "agent-alpha", MinResources: 1, MaxResources: 10},
		{ID: "agent-beta", MinResources: 1, MaxResources: 10},
		{ID: "agent-gamma", MinResources: 1, MaxResources: 10},
	}

	tasks := []Task{
		{ID: "task-1", RequiredResources: 5, Priority: 1},
		{ID: "task-2", RequiredResources: 3, Priority: 1},
		{ID: "task-3", RequiredResources: 4, Priority: 1},
		{ID: "task-4", RequiredResources: 2, Priority: 1},
		{ID: "task-5", RequiredResources: 6, Priority: 1},
		{ID: "task-6", RequiredResources: 1, Priority: 1},
	}

	schedule, err := RoundRobinSchedule(agents, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all tasks are scheduled
	if len(schedule.TaskOrder) != len(tasks) {
		t.Errorf("expected %d tasks in schedule, got %d", len(tasks), len(schedule.TaskOrder))
	}

	// Verify task IDs are present
	taskIDMap := make(map[string]bool)
	for _, assignment := range schedule.TaskOrder {
		taskIDMap[assignment.TaskID] = true
	}

	for _, task := range tasks {
		if !taskIDMap[task.ID] {
			t.Errorf("task %s not found in schedule", task.ID)
		}
	}
}

// TestRoundRobinSchedule_Fairness verifies round-robin fairness property.
func TestRoundRobinSchedule_Fairness(t *testing.T) {
	agents := []Agent{
		{ID: "agent-1", MinResources: 1, MaxResources: 10},
		{ID: "agent-2", MinResources: 1, MaxResources: 10},
		{ID: "agent-3", MinResources: 1, MaxResources: 10},
	}

	tasks := []Task{
		{ID: "task-a", RequiredResources: 1, Priority: 1},
		{ID: "task-b", RequiredResources: 1, Priority: 1},
		{ID: "task-c", RequiredResources: 1, Priority: 1},
		{ID: "task-d", RequiredResources: 1, Priority: 1},
		{ID: "task-e", RequiredResources: 1, Priority: 1},
		{ID: "task-f", RequiredResources: 1, Priority: 1},
		{ID: "task-g", RequiredResources: 1, Priority: 1},
	}

	schedule, err := RoundRobinSchedule(agents, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check fairness: max_load - min_load ≤ 1
	minLoad, maxLoad := schedule.AgentLoads["agent-1"], schedule.AgentLoads["agent-1"]
	for _, load := range schedule.AgentLoads {
		if load < minLoad {
			minLoad = load
		}
		if load > maxLoad {
			maxLoad = load
		}
	}

	if maxLoad-minLoad > 1 {
		t.Errorf("fairness violation: max load=%d, min load=%d, diff=%d (should be ≤ 1)",
			maxLoad, minLoad, maxLoad-minLoad)
	}
}

// TestRoundRobinSchedule_Determinism verifies deterministic scheduling.
func TestRoundRobinSchedule_Determinism(t *testing.T) {
	agents := []Agent{
		{ID: "agent-z", MinResources: 1, MaxResources: 10},
		{ID: "agent-a", MinResources: 1, MaxResources: 10},
		{ID: "agent-m", MinResources: 1, MaxResources: 10},
	}

	tasks := []Task{
		{ID: "task-9", RequiredResources: 1, Priority: 1},
		{ID: "task-1", RequiredResources: 1, Priority: 1},
		{ID: "task-5", RequiredResources: 1, Priority: 1},
	}

	// Run scheduling 10 times
	schedules := make([]*Schedule, 10)
	for i := 0; i < 10; i++ {
		schedule, err := RoundRobinSchedule(agents, tasks)
		if err != nil {
			t.Fatalf("unexpected error on run %d: %v", i, err)
		}
		schedules[i] = schedule
	}

	// Verify all schedules are identical
	first := schedules[0]
	for i := 1; i < len(schedules); i++ {
		if !reflect.DeepEqual(first.TaskOrder, schedules[i].TaskOrder) {
			t.Errorf("run %d produced different task order:\nwant: %+v\ngot:  %+v",
				i, first.TaskOrder, schedules[i].TaskOrder)
		}
		if !reflect.DeepEqual(first.AgentLoads, schedules[i].AgentLoads) {
			t.Errorf("run %d produced different agent loads:\nwant: %v\ngot:  %v",
				i, first.AgentLoads, schedules[i].AgentLoads)
		}
	}
}

// TestRoundRobinSchedule_Errors verifies error handling.
func TestRoundRobinSchedule_Errors(t *testing.T) {
	validAgents := []Agent{{ID: "agent-1"}}
	validTasks := []Task{{ID: "task-1"}}

	tests := []struct {
		name    string
		agents  []Agent
		tasks   []Task
		wantErr bool
	}{
		{
			name:    "empty agent list",
			agents:  []Agent{},
			tasks:   validTasks,
			wantErr: true,
		},
		{
			name:    "empty task list",
			agents:  validAgents,
			tasks:   []Task{},
			wantErr: true,
		},
		{
			name: "duplicate agent IDs",
			agents: []Agent{
				{ID: "agent-1"},
				{ID: "agent-1"},
			},
			tasks:   validTasks,
			wantErr: true,
		},
		{
			name:   "duplicate task IDs",
			agents: validAgents,
			tasks: []Task{
				{ID: "task-1"},
				{ID: "task-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RoundRobinSchedule(tt.agents, tt.tasks)
			if (err != nil) != tt.wantErr {
				t.Errorf("RoundRobinSchedule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPrioritySchedule_Basic verifies basic priority scheduling.
func TestPrioritySchedule_Basic(t *testing.T) {
	agents := []Agent{
		{ID: "agent-1", MinResources: 1, MaxResources: 10},
		{ID: "agent-2", MinResources: 1, MaxResources: 10},
	}

	tasks := []Task{
		{ID: "task-low", RequiredResources: 1, Priority: 1},
		{ID: "task-high", RequiredResources: 1, Priority: 10},
		{ID: "task-medium", RequiredResources: 1, Priority: 5},
	}

	schedule, err := PrioritySchedule(agents, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify higher priority tasks come first
	if len(schedule.TaskOrder) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(schedule.TaskOrder))
	}

	if schedule.TaskOrder[0].TaskID != "task-high" {
		t.Errorf("expected highest priority task first, got %s", schedule.TaskOrder[0].TaskID)
	}
	if schedule.TaskOrder[1].TaskID != "task-medium" {
		t.Errorf("expected medium priority task second, got %s", schedule.TaskOrder[1].TaskID)
	}
	if schedule.TaskOrder[2].TaskID != "task-low" {
		t.Errorf("expected lowest priority task last, got %s", schedule.TaskOrder[2].TaskID)
	}
}

// TestPrioritySchedule_PriorityOrdering verifies priority preservation.
func TestPrioritySchedule_PriorityOrdering(t *testing.T) {
	agents := []Agent{
		{ID: "agent-1"},
		{ID: "agent-2"},
	}

	tasks := []Task{
		{ID: "task-p9", Priority: 9},
		{ID: "task-p1", Priority: 1},
		{ID: "task-p5", Priority: 5},
		{ID: "task-p10", Priority: 10},
		{ID: "task-p3", Priority: 3},
	}

	schedule, err := PrioritySchedule(agents, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tasks are ordered by priority (descending)
	for i := 1; i < len(schedule.TaskOrder); i++ {
		prevTaskID := schedule.TaskOrder[i-1].TaskID
		currTaskID := schedule.TaskOrder[i].TaskID

		// Find original priorities
		var prevPriority, currPriority int
		for _, task := range tasks {
			if task.ID == prevTaskID {
				prevPriority = task.Priority
			}
			if task.ID == currTaskID {
				currPriority = task.Priority
			}
		}

		if prevPriority < currPriority {
			t.Errorf("priority ordering violation: task %s (priority %d) scheduled before task %s (priority %d)",
				prevTaskID, prevPriority, currTaskID, currPriority)
		}
	}
}

// TestPrioritySchedule_LoadBalancing verifies load balancing with priority.
func TestPrioritySchedule_LoadBalancing(t *testing.T) {
	agents := []Agent{
		{ID: "agent-1"},
		{ID: "agent-2"},
		{ID: "agent-3"},
	}

	// All tasks have same priority - should balance load
	tasks := []Task{
		{ID: "task-1", Priority: 5},
		{ID: "task-2", Priority: 5},
		{ID: "task-3", Priority: 5},
		{ID: "task-4", Priority: 5},
		{ID: "task-5", Priority: 5},
		{ID: "task-6", Priority: 5},
		{ID: "task-7", Priority: 5},
	}

	schedule, err := PrioritySchedule(agents, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify load balancing: max_load - min_load ≤ 1
	minLoad, maxLoad := schedule.AgentLoads["agent-1"], schedule.AgentLoads["agent-1"]
	for _, load := range schedule.AgentLoads {
		if load < minLoad {
			minLoad = load
		}
		if load > maxLoad {
			maxLoad = load
		}
	}

	if maxLoad-minLoad > 1 {
		t.Errorf("load balancing violation: max load=%d, min load=%d, diff=%d (should be ≤ 1)",
			maxLoad, minLoad, maxLoad-minLoad)
	}
}

// TestPrioritySchedule_Determinism verifies deterministic priority scheduling.
func TestPrioritySchedule_Determinism(t *testing.T) {
	agents := []Agent{
		{ID: "agent-z"},
		{ID: "agent-a"},
		{ID: "agent-m"},
	}

	tasks := []Task{
		{ID: "task-9", Priority: 3},
		{ID: "task-1", Priority: 7},
		{ID: "task-5", Priority: 5},
		{ID: "task-2", Priority: 7}, // Same priority as task-1, should tie-break by ID
	}

	// Run scheduling 10 times
	schedules := make([]*Schedule, 10)
	for i := 0; i < 10; i++ {
		schedule, err := PrioritySchedule(agents, tasks)
		if err != nil {
			t.Fatalf("unexpected error on run %d: %v", i, err)
		}
		schedules[i] = schedule
	}

	// Verify all schedules are identical
	first := schedules[0]
	for i := 1; i < len(schedules); i++ {
		if !reflect.DeepEqual(first.TaskOrder, schedules[i].TaskOrder) {
			t.Errorf("run %d produced different task order:\nwant: %+v\ngot:  %+v",
				i, first.TaskOrder, schedules[i].TaskOrder)
		}
	}
}

// TestPrioritySchedule_TieBreaking verifies deterministic tie-breaking.
func TestPrioritySchedule_TieBreaking(t *testing.T) {
	agents := []Agent{
		{ID: "agent-1"},
	}

	tasks := []Task{
		{ID: "task-z", Priority: 5},
		{ID: "task-a", Priority: 5},
		{ID: "task-m", Priority: 5},
	}

	schedule, err := PrioritySchedule(agents, tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With same priority, tasks should be ordered by ID (lexicographic)
	expectedOrder := []string{"task-a", "task-m", "task-z"}
	for i, assignment := range schedule.TaskOrder {
		if assignment.TaskID != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], assignment.TaskID)
		}
	}
}

// TestPrioritySchedule_Errors verifies error handling.
func TestPrioritySchedule_Errors(t *testing.T) {
	validAgents := []Agent{{ID: "agent-1"}}
	validTasks := []Task{{ID: "task-1", Priority: 1}}

	tests := []struct {
		name    string
		agents  []Agent
		tasks   []Task
		wantErr bool
	}{
		{
			name:    "empty agent list",
			agents:  []Agent{},
			tasks:   validTasks,
			wantErr: true,
		},
		{
			name:    "empty task list",
			agents:  validAgents,
			tasks:   []Task{},
			wantErr: true,
		},
		{
			name: "duplicate agent IDs",
			agents: []Agent{
				{ID: "agent-1"},
				{ID: "agent-1"},
			},
			tasks:   validTasks,
			wantErr: true,
		},
		{
			name:   "duplicate task IDs",
			agents: validAgents,
			tasks: []Task{
				{ID: "task-1", Priority: 1},
				{ID: "task-1", Priority: 2},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PrioritySchedule(tt.agents, tt.tasks)
			if (err != nil) != tt.wantErr {
				t.Errorf("PrioritySchedule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestExhaustionTest verifies exhaustion scenario detection.
func TestExhaustionTest(t *testing.T) {
	tests := []struct {
		name          string
		resourceLimit int
		wantDeficit   int
	}{
		{
			name:          "limit 100",
			resourceLimit: 100,
			wantDeficit:   1,
		},
		{
			name:          "limit 0",
			resourceLimit: 0,
			wantDeficit:   1,
		},
		{
			name:          "limit 1000",
			resourceLimit: 1000,
			wantDeficit:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := ExhaustionTest(tt.resourceLimit)

			if report.AvailableResources != tt.resourceLimit {
				t.Errorf("expected available=%d, got %d", tt.resourceLimit, report.AvailableResources)
			}

			if report.RequestedResources != tt.resourceLimit+1 {
				t.Errorf("expected requested=%d, got %d", tt.resourceLimit+1, report.RequestedResources)
			}

			if report.Deficit != tt.wantDeficit {
				t.Errorf("expected deficit=%d, got %d", tt.wantDeficit, report.Deficit)
			}

			if report.Reason == "" {
				t.Error("expected non-empty reason")
			}
		})
	}
}

// TestExhaustionTest_Determinism verifies exhaustion test determinism.
func TestExhaustionTest_Determinism(t *testing.T) {
	resourceLimit := 50

	reports := make([]*FailureReport, 10)
	for i := 0; i < 10; i++ {
		reports[i] = ExhaustionTest(resourceLimit)
	}

	first := reports[0]
	for i := 1; i < len(reports); i++ {
		if !reflect.DeepEqual(first, reports[i]) {
			t.Errorf("run %d produced different report:\nwant: %+v\ngot:  %+v",
				i, first, reports[i])
		}
	}
}

// TestResourceConservation verifies no over-allocation occurs.
func TestResourceConservation(t *testing.T) {
	testCases := []struct {
		agentCount int
		budget     int
	}{
		{3, 10},
		{5, 17},
		{10, 100},
		{7, 49},
	}

	for _, tc := range testCases {
		alloc, err := AllocateResources(tc.agentCount, tc.budget)
		if err != nil {
			t.Fatalf("unexpected error for agentCount=%d, budget=%d: %v",
				tc.agentCount, tc.budget, err)
		}

		total := 0
		for _, v := range alloc.Assignments {
			total += v
		}

		if total != tc.budget {
			t.Errorf("resource conservation violated: allocated %d, budget %d",
				total, tc.budget)
		}
	}
}

// TestGiniCoefficient verifies Gini calculation.
func TestGiniCoefficient(t *testing.T) {
	tests := []struct {
		name        string
		allocations map[string]int
		wantGini    float64
		tolerance   float64
	}{
		{
			name: "perfect equality",
			allocations: map[string]int{
				"agent-0": 10,
				"agent-1": 10,
				"agent-2": 10,
			},
			wantGini:  0.0,
			tolerance: 0.01,
		},
		{
			name: "perfect inequality",
			allocations: map[string]int{
				"agent-0": 0,
				"agent-1": 0,
				"agent-2": 30,
			},
			wantGini:  0.66, // Approximately 2/3
			tolerance: 0.01,
		},
		{
			name: "moderate inequality",
			allocations: map[string]int{
				"agent-0": 5,
				"agent-1": 10,
				"agent-2": 15,
			},
			wantGini:  0.22, // Moderate Gini
			tolerance: 0.02,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gini := calculateGini(tt.allocations)
			if gini < tt.wantGini-tt.tolerance || gini > tt.wantGini+tt.tolerance {
				t.Errorf("Gini coefficient = %f, want %f ± %f", gini, tt.wantGini, tt.tolerance)
			}
		})
	}
}
