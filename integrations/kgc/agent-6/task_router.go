// Package agent6 implements deterministic task routing and task graph evaluation
// with support for XOR, AND, OR routing combinators and replay semantics.
package agent6

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Task represents a unit of work to be routed
type Task struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Priority int                    `json:"priority"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Predicate is a pure function that evaluates a task
type Predicate func(task *Task) bool

// RoutingCombinator defines how predicates are combined
type RoutingCombinator int

const (
	XOR RoutingCombinator = iota // Exactly one predicate must match
	AND                          // All predicates must match
	OR                           // At least one predicate must match
)

func (rc RoutingCombinator) String() string {
	switch rc {
	case XOR:
		return "XOR"
	case AND:
		return "AND"
	case OR:
		return "OR"
	default:
		return "UNKNOWN"
	}
}

// Route defines a routing rule
type Route struct {
	Name        string            `json:"name"`
	Predicates  []Predicate       `json:"-"` // Not serializable
	Combinator  RoutingCombinator `json:"combinator"`
	TargetAgent string            `json:"target_agent"`
}

// Router manages routing decisions
type Router struct {
	routes []Route
}

// NewRouter creates a new router with the given routes
func NewRouter(routes []Route) *Router {
	return &Router{routes: routes}
}

// Route evaluates predicates and returns the target agent ID
// This is a deterministic operation - same task + predicates → same result
func (r *Router) Route(task *Task, predicates []Predicate) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task cannot be nil")
	}
	if task.ID == "" {
		return "", fmt.Errorf("task ID cannot be empty")
	}

	// Evaluate each route in order
	for i, route := range r.routes {
		// Use predicates from route or passed predicates
		preds := route.Predicates
		if len(predicates) > 0 && i == 0 {
			// Use passed predicates for first route (for testing)
			preds = predicates
		}

		matched, err := evaluateRoute(task, preds, route.Combinator)
		if err != nil {
			return "", fmt.Errorf("route %d: %w", i, err)
		}

		if matched {
			return route.TargetAgent, nil
		}
	}

	return "", fmt.Errorf("no route matched for task %s (type=%s)", task.ID, task.Type)
}

// evaluateRoute evaluates predicates against a task using the specified combinator
func evaluateRoute(task *Task, predicates []Predicate, combinator RoutingCombinator) (bool, error) {
	if len(predicates) == 0 {
		return false, fmt.Errorf("no predicates to evaluate")
	}

	// Evaluate all predicates
	results := make([]bool, len(predicates))
	for i, pred := range predicates {
		results[i] = pred(task)
	}

	// Apply combinator logic
	switch combinator {
	case XOR:
		// Exactly one must be true
		trueCount := 0
		for _, result := range results {
			if result {
				trueCount++
			}
		}
		return trueCount == 1, nil

	case AND:
		// All must be true
		for _, result := range results {
			if !result {
				return false, nil
			}
		}
		return true, nil

	case OR:
		// At least one must be true
		for _, result := range results {
			if result {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("unknown combinator: %v", combinator)
	}
}

// TaskGraph represents a DAG of tasks with dependencies
type TaskGraph struct {
	Tasks        []*Task           `json:"tasks"`
	Dependencies map[string][]string `json:"dependencies"` // TaskID → []DependsOnTaskID
}

// NewTaskGraph creates a new task graph
func NewTaskGraph(tasks []*Task, dependencies map[string][]string) *TaskGraph {
	if dependencies == nil {
		dependencies = make(map[string][]string)
	}
	return &TaskGraph{
		Tasks:        tasks,
		Dependencies: dependencies,
	}
}

// EvaluateTaskGraph performs topological sort and returns execution order
// Returns error if cycle detected
func (r *Router) EvaluateTaskGraph(graph *TaskGraph) ([]string, error) {
	if graph == nil {
		return nil, fmt.Errorf("task graph cannot be nil")
	}
	if len(graph.Tasks) == 0 {
		return []string{}, nil
	}

	// Build task ID to task map
	taskMap := make(map[string]*Task)
	for _, task := range graph.Tasks {
		if task.ID == "" {
			return nil, fmt.Errorf("task ID cannot be empty")
		}
		taskMap[task.ID] = task
	}

	// Validate dependencies reference valid tasks
	for taskID, deps := range graph.Dependencies {
		if _, ok := taskMap[taskID]; !ok {
			return nil, fmt.Errorf("dependency references unknown task: %s", taskID)
		}
		for _, depID := range deps {
			if _, ok := taskMap[depID]; !ok {
				return nil, fmt.Errorf("task %s depends on unknown task: %s", taskID, depID)
			}
		}
	}

	// Perform topological sort using Kahn's algorithm
	return kahnTopologicalSort(graph)
}

// kahnTopologicalSort implements Kahn's algorithm for topological sorting
// Returns error if cycle detected
// Dependencies map: TaskID → []DependsOnTaskID (task depends on these tasks)
func kahnTopologicalSort(graph *TaskGraph) ([]string, error) {
	// Calculate in-degree for each node
	// in-degree = number of tasks this task depends on
	inDegree := make(map[string]int)
	for _, task := range graph.Tasks {
		inDegree[task.ID] = 0
	}

	// Count incoming edges (dependencies)
	// If "B": {"A"}, then B depends on A, so B has an incoming edge from A
	for taskID, deps := range graph.Dependencies {
		inDegree[taskID] = len(deps)
	}

	// Find all nodes with in-degree 0 (no dependencies)
	var queue []string
	for taskID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, taskID)
		}
	}

	// Sort queue for deterministic ordering (stable sort)
	sort.Strings(queue)

	var result []string

	for len(queue) > 0 {
		// Pop first element (deterministic order)
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// When we process a task, we "remove" it from the graph
		// Find all tasks that depend on current task and decrement their in-degree
		for taskID, deps := range graph.Dependencies {
			for _, depID := range deps {
				if depID == current {
					inDegree[taskID]--
					if inDegree[taskID] == 0 {
						queue = append(queue, taskID)
						// Keep queue sorted for determinism
						sort.Strings(queue)
					}
					break // Found the dependency, move to next task
				}
			}
		}
	}

	// Check if all nodes were processed (no cycle)
	if len(result) != len(graph.Tasks) {
		return nil, fmt.Errorf("cycle detected in task graph: processed %d of %d tasks",
			len(result), len(graph.Tasks))
	}

	return result, nil
}

// ReplayScript represents a routing decision replay script
type ReplayScript struct {
	ExecutionID   string                 `json:"execution_id"`
	Timestamp     int64                  `json:"timestamp"`
	TaskID        string                 `json:"task_id"`
	TaskType      string                 `json:"task_type"`
	TaskPriority  int                    `json:"task_priority"`
	RoutedToAgent string                 `json:"routed_to_agent"`
	InputHash     string                 `json:"input_hash"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// GenerateReplayScript creates a replay script for a routing decision
func GenerateReplayScript(task *Task, routedTo string) (*ReplayScript, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	// Generate execution ID (deterministic based on task content)
	executionID := generateExecutionID(task)

	// Calculate input hash
	inputHash, err := hashTask(task)
	if err != nil {
		return nil, fmt.Errorf("failed to hash task: %w", err)
	}

	script := &ReplayScript{
		ExecutionID:   executionID,
		Timestamp:     time.Now().UnixNano(),
		TaskID:        task.ID,
		TaskType:      task.Type,
		TaskPriority:  task.Priority,
		RoutedToAgent: routedTo,
		InputHash:     inputHash,
		Metadata:      task.Metadata,
	}

	return script, nil
}

// ReplayRoute re-executes routing and verifies it matches the replay script
func (r *Router) ReplayRoute(task *Task, predicates []Predicate, script *ReplayScript) (string, error) {
	if script == nil {
		return "", fmt.Errorf("replay script cannot be nil")
	}

	// Verify task matches script
	if task.ID != script.TaskID {
		return "", fmt.Errorf("task ID mismatch: got %s, expected %s", task.ID, script.TaskID)
	}

	// Calculate current input hash
	currentHash, err := hashTask(task)
	if err != nil {
		return "", fmt.Errorf("failed to hash task: %w", err)
	}

	if currentHash != script.InputHash {
		return "", fmt.Errorf("input hash mismatch: task has been modified")
	}

	// Re-execute routing
	routedTo, err := r.Route(task, predicates)
	if err != nil {
		return "", fmt.Errorf("replay routing failed: %w", err)
	}

	// Verify result matches script
	if routedTo != script.RoutedToAgent {
		return "", fmt.Errorf("non-deterministic routing detected: got %s, expected %s",
			routedTo, script.RoutedToAgent)
	}

	return routedTo, nil
}

// generateExecutionID creates a deterministic execution ID based on task content
func generateExecutionID(task *Task) string {
	// Use task content to generate deterministic ID
	content := fmt.Sprintf("%s:%s:%d", task.ID, task.Type, task.Priority)
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("exec-%x", hash[:8])
}

// hashTask creates a SHA256 hash of the task for tamper detection
func hashTask(task *Task) (string, error) {
	// Create a deterministic representation
	// Sort metadata keys for consistency
	var metadataKeys []string
	for key := range task.Metadata {
		metadataKeys = append(metadataKeys, key)
	}
	sort.Strings(metadataKeys)

	var metadataParts []string
	for _, key := range metadataKeys {
		value := task.Metadata[key]
		metadataParts = append(metadataParts, fmt.Sprintf("%s=%v", key, value))
	}

	content := fmt.Sprintf("id=%s,type=%s,priority=%d,metadata={%s}",
		task.ID, task.Type, task.Priority, strings.Join(metadataParts, ","))

	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash), nil
}

// MarshalReplayScript converts a replay script to JSON
func MarshalReplayScript(script *ReplayScript) ([]byte, error) {
	return json.MarshalIndent(script, "", "  ")
}

// UnmarshalReplayScript parses a replay script from JSON
func UnmarshalReplayScript(data []byte) (*ReplayScript, error) {
	var script ReplayScript
	if err := json.Unmarshal(data, &script); err != nil {
		return nil, fmt.Errorf("failed to unmarshal replay script: %w", err)
	}
	return &script, nil
}

// ValidateTaskGraph checks if a task graph is valid (no cycles, valid references)
func ValidateTaskGraph(graph *TaskGraph) error {
	if graph == nil {
		return fmt.Errorf("task graph cannot be nil")
	}

	// Build task ID set
	taskIDs := make(map[string]bool)
	for _, task := range graph.Tasks {
		if task.ID == "" {
			return fmt.Errorf("task ID cannot be empty")
		}
		if taskIDs[task.ID] {
			return fmt.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDs[task.ID] = true
	}

	// Validate all dependency references
	for taskID, deps := range graph.Dependencies {
		if !taskIDs[taskID] {
			return fmt.Errorf("dependency references unknown task: %s", taskID)
		}
		for _, depID := range deps {
			if !taskIDs[depID] {
				return fmt.Errorf("task %s depends on unknown task: %s", taskID, depID)
			}
		}
	}

	// Check for cycles using DFS
	return detectCycle(graph)
}

// detectCycle uses DFS to detect cycles in the task graph
func detectCycle(graph *TaskGraph) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(taskID string) error
	dfs = func(taskID string) error {
		visited[taskID] = true
		recStack[taskID] = true

		// Visit all tasks that depend on this task
		for id, deps := range graph.Dependencies {
			for _, depID := range deps {
				if depID == taskID {
					if !visited[id] {
						if err := dfs(id); err != nil {
							return err
						}
					} else if recStack[id] {
						return fmt.Errorf("cycle detected: %s → %s", taskID, id)
					}
				}
			}
		}

		recStack[taskID] = false
		return nil
	}

	for _, task := range graph.Tasks {
		if !visited[task.ID] {
			if err := dfs(task.ID); err != nil {
				return err
			}
		}
	}

	return nil
}
