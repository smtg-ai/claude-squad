// Package agent4 implements deterministic resource allocation and capacity management
// for the KGC multi-agent substrate.
//
// All operations are deterministic: same inputs always produce identical outputs.
// Scheduling algorithms guarantee fairness and priority preservation.
package agent4

import (
	"fmt"
	"sort"
)

// Agent represents an agent in the system that requires resource allocation.
type Agent struct {
	ID           string // Unique identifier for the agent
	MinResources int    // Minimum resources needed to function
	MaxResources int    // Maximum resources this agent can utilize
}

// Task represents a unit of work to be scheduled.
type Task struct {
	ID                string // Unique identifier for the task
	RequiredResources int    // Resources required by this task
	Priority          int    // Priority level (higher = more important)
}

// Allocation represents the result of resource allocation.
type Allocation struct {
	Assignments map[string]int // Agent ID -> allocated resources
	Remaining   int            // Unallocated resources
	Fairness    float64        // Gini coefficient (0 = perfect equality, 1 = perfect inequality)
}

// TaskAssignment represents a task assigned to a specific agent.
type TaskAssignment struct {
	TaskID  string // ID of the task
	AgentID string // ID of the agent assigned to execute the task
	Order   int    // Execution order (deterministic, sequential)
}

// Schedule represents an ordered list of task assignments.
type Schedule struct {
	TaskOrder  []TaskAssignment // Deterministically ordered task assignments
	AgentLoads map[string]int   // Agent ID -> number of tasks assigned
}

// FailureReport describes a resource exhaustion scenario.
type FailureReport struct {
	Reason             string // Human-readable description of the failure
	RequestedResources int    // Total resources requested
	AvailableResources int    // Total resources available
	Deficit            int    // Shortfall (requested - available)
}

// AllocateResources distributes resourceBudget fairly among agentCount agents.
//
// Algorithm:
//   1. Compute base quota: floor(resourceBudget / agentCount)
//   2. Distribute remainder: first (resourceBudget mod agentCount) agents get +1
//   3. Result: all agents differ by at most 1 resource unit
//
// Determinism: Pure function, no randomization, stable output for same inputs.
// Fairness: max(allocation) - min(allocation) ≤ 1
func AllocateResources(agentCount int, resourceBudget int) (*Allocation, error) {
	if agentCount <= 0 {
		return nil, fmt.Errorf("invalid agent count: %d (must be > 0)", agentCount)
	}
	if resourceBudget < 0 {
		return nil, fmt.Errorf("invalid resource budget: %d (must be >= 0)", resourceBudget)
	}

	assignments := make(map[string]int)
	baseQuota := resourceBudget / agentCount
	remainder := resourceBudget % agentCount

	// Assign resources to agents deterministically
	// Agents are numbered 0..agentCount-1
	for i := 0; i < agentCount; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		if i < remainder {
			assignments[agentID] = baseQuota + 1
		} else {
			assignments[agentID] = baseQuota
		}
	}

	// Calculate Gini coefficient for fairness metric
	fairness := calculateGini(assignments)

	return &Allocation{
		Assignments: assignments,
		Remaining:   0, // All resources are allocated
		Fairness:    fairness,
	}, nil
}

// RoundRobinSchedule assigns tasks to agents in round-robin fashion.
//
// Algorithm:
//   1. Sort agents by ID (lexicographic, deterministic)
//   2. Sort tasks by ID (lexicographic, deterministic)
//   3. For each task i, assign to agent (i mod len(agents))
//
// Determinism: Stable sorting ensures identical output for same inputs.
// Fairness: max(load) - min(load) ≤ 1 (proven by round-robin property)
func RoundRobinSchedule(agents []Agent, tasks []Task) (*Schedule, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("empty agent list: cannot schedule tasks")
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("empty task list: nothing to schedule")
	}

	// Check for duplicate agent IDs
	agentIDSet := make(map[string]bool)
	for _, agent := range agents {
		if agentIDSet[agent.ID] {
			return nil, fmt.Errorf("duplicate agent ID: %s", agent.ID)
		}
		agentIDSet[agent.ID] = true
	}

	// Check for duplicate task IDs
	taskIDSet := make(map[string]bool)
	for _, task := range tasks {
		if taskIDSet[task.ID] {
			return nil, fmt.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDSet[task.ID] = true
	}

	// Sort agents by ID (deterministic)
	sortedAgents := make([]Agent, len(agents))
	copy(sortedAgents, agents)
	sort.SliceStable(sortedAgents, func(i, j int) bool {
		return sortedAgents[i].ID < sortedAgents[j].ID
	})

	// Sort tasks by ID (deterministic)
	sortedTasks := make([]Task, len(tasks))
	copy(sortedTasks, tasks)
	sort.SliceStable(sortedTasks, func(i, j int) bool {
		return sortedTasks[i].ID < sortedTasks[j].ID
	})

	// Assign tasks in round-robin fashion
	taskOrder := make([]TaskAssignment, len(sortedTasks))
	agentLoads := make(map[string]int)

	for i, task := range sortedTasks {
		agentIndex := i % len(sortedAgents)
		selectedAgent := sortedAgents[agentIndex]

		taskOrder[i] = TaskAssignment{
			TaskID:  task.ID,
			AgentID: selectedAgent.ID,
			Order:   i,
		}

		agentLoads[selectedAgent.ID]++
	}

	return &Schedule{
		TaskOrder:  taskOrder,
		AgentLoads: agentLoads,
	}, nil
}

// PrioritySchedule assigns tasks to agents based on priority and load balancing.
//
// Algorithm:
//   1. Sort agents by ID (deterministic)
//   2. Sort tasks by (Priority DESC, ID ASC) - higher priority first
//   3. For each task, assign to agent with minimum current load
//   4. Tie-breaking: agent with lexicographically smallest ID
//
// Determinism: Stable sort + deterministic tie-breaking ensures reproducibility.
// Priority Property: Higher priority tasks are scheduled earlier.
func PrioritySchedule(agents []Agent, prioritizedTasks []Task) (*Schedule, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("empty agent list: cannot schedule tasks")
	}
	if len(prioritizedTasks) == 0 {
		return nil, fmt.Errorf("empty task list: nothing to schedule")
	}

	// Check for duplicate agent IDs
	agentIDSet := make(map[string]bool)
	for _, agent := range agents {
		if agentIDSet[agent.ID] {
			return nil, fmt.Errorf("duplicate agent ID: %s", agent.ID)
		}
		agentIDSet[agent.ID] = true
	}

	// Check for duplicate task IDs
	taskIDSet := make(map[string]bool)
	for _, task := range prioritizedTasks {
		if taskIDSet[task.ID] {
			return nil, fmt.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDSet[task.ID] = true
	}

	// Sort agents by ID (deterministic)
	sortedAgents := make([]Agent, len(agents))
	copy(sortedAgents, agents)
	sort.SliceStable(sortedAgents, func(i, j int) bool {
		return sortedAgents[i].ID < sortedAgents[j].ID
	})

	// Sort tasks by priority (descending), then by ID (ascending) for determinism
	sortedTasks := make([]Task, len(prioritizedTasks))
	copy(sortedTasks, prioritizedTasks)
	sort.SliceStable(sortedTasks, func(i, j int) bool {
		if sortedTasks[i].Priority != sortedTasks[j].Priority {
			return sortedTasks[i].Priority > sortedTasks[j].Priority // Higher priority first
		}
		return sortedTasks[i].ID < sortedTasks[j].ID // Tie-break by ID
	})

	// Initialize agent loads
	agentLoads := make(map[string]int)
	for _, agent := range sortedAgents {
		agentLoads[agent.ID] = 0
	}

	// Assign tasks to agents with minimum load (tie-break by ID)
	taskOrder := make([]TaskAssignment, len(sortedTasks))

	for i, task := range sortedTasks {
		// Find agent with minimum load (deterministic tie-breaking by ID)
		selectedAgent := sortedAgents[0]
		minLoad := agentLoads[selectedAgent.ID]

		for _, agent := range sortedAgents[1:] {
			currentLoad := agentLoads[agent.ID]
			if currentLoad < minLoad || (currentLoad == minLoad && agent.ID < selectedAgent.ID) {
				selectedAgent = agent
				minLoad = currentLoad
			}
		}

		taskOrder[i] = TaskAssignment{
			TaskID:  task.ID,
			AgentID: selectedAgent.ID,
			Order:   i,
		}

		agentLoads[selectedAgent.ID]++
	}

	return &Schedule{
		TaskOrder:  taskOrder,
		AgentLoads: agentLoads,
	}, nil
}

// ExhaustionTest simulates resource exhaustion by requesting more than available.
//
// This is a deterministic test function that demonstrates system behavior when
// resource demand exceeds capacity. Used for verification and testing.
//
// Algorithm:
//   1. Set demand = resourceLimit + 1
//   2. Report deficit and reason
//
// Determinism: Pure function, no side effects.
func ExhaustionTest(resourceLimit int) *FailureReport {
	requestedResources := resourceLimit + 1
	availableResources := resourceLimit
	deficit := requestedResources - availableResources

	return &FailureReport{
		Reason:             "Resource exhaustion: insufficient capacity",
		RequestedResources: requestedResources,
		AvailableResources: availableResources,
		Deficit:            deficit,
	}
}

// calculateGini computes the Gini coefficient for resource allocation fairness.
//
// Gini coefficient ranges from 0 (perfect equality) to 1 (perfect inequality).
// For resource allocation:
//   - 0.0 = all agents have identical allocations
//   - 1.0 = one agent has all resources, others have none
//
// Algorithm (simplified for small datasets):
//   1. Sort allocations
//   2. Compute cumulative differences
//   3. Normalize by total resources and agent count
func calculateGini(allocations map[string]int) float64 {
	if len(allocations) == 0 {
		return 0.0
	}

	// Extract values and sort them
	values := make([]int, 0, len(allocations))
	totalResources := 0
	for _, v := range allocations {
		values = append(values, v)
		totalResources += v
	}

	if totalResources == 0 {
		return 0.0 // Perfect equality when no resources
	}

	sort.Ints(values)

	// Calculate Gini coefficient
	n := len(values)
	numerator := 0.0
	for i := 0; i < n; i++ {
		numerator += float64((i + 1)) * float64(values[i])
	}

	gini := (2.0*numerator)/float64(n*totalResources) - float64(n+1)/float64(n)

	return gini
}
