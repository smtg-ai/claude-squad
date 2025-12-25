package main

import (
	"claude-squad/log"
	"claude-squad/ollama"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TaskRouter is a custom routing strategy that directs tasks to appropriate models
// based on task characteristics, load balancing, and availability.
type TaskRouter struct {
	registry     *ollama.ModelRegistry
	taskCounts   map[string]int32
	mu           sync.RWMutex
	routingRules map[string]string // task type -> model mapping
}

// NewTaskRouter creates a new task router with custom routing logic
func NewTaskRouter(registry *ollama.ModelRegistry) *TaskRouter {
	return &TaskRouter{
		registry:   registry,
		taskCounts: make(map[string]int32),
		routingRules: map[string]string{
			"generation":   "llama2:7b",      // Use faster model for generation
			"analysis":     "mistral:7b",     // Use instruction-following model for analysis
			"optimization": "neural-chat:7b", // Use specialized model for optimization
			"default":      "llama2:7b",
		},
	}
}

// RouteTask determines which model should handle a specific task
func (tr *TaskRouter) RouteTask(task *ollama.Task) string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	// Extract task type from payload
	payload, ok := task.Payload.(map[string]interface{})
	if !ok {
		return tr.registry.GetDefaultModel()
	}

	taskType, ok := payload["type"].(string)
	if !ok {
		return tr.registry.GetDefaultModel()
	}

	// Check if there's a routing rule for this task type
	if model, exists := tr.routingRules[taskType]; exists {
		return model
	}

	return tr.registry.GetDefaultModel()
}

// SelectBestModel implements load-balancing aware model selection
func (tr *TaskRouter) SelectBestModel(taskType string) string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	// Get candidate models based on routing rules
	var candidates []string
	if model, exists := tr.routingRules[taskType]; exists {
		candidates = append(candidates, model)
	}

	// Add fallback options
	for _, model := range tr.registry.ListEnabledModels() {
		candidates = append(candidates, model.Name)
	}

	if len(candidates) == 0 {
		return tr.registry.GetDefaultModel()
	}

	// Select model with lowest current load
	bestModel := candidates[0]
	minLoad := atomic.LoadInt32(&tr.taskCounts[candidates[0]])

	for _, model := range candidates[1:] {
		currentLoad := atomic.LoadInt32(&tr.taskCounts[model])
		if currentLoad < minLoad {
			minLoad = currentLoad
			bestModel = model
		}
	}

	return bestModel
}

// RecordTaskStart increments the load counter for a model
func (tr *TaskRouter) RecordTaskStart(modelName string) {
	atomic.AddInt32(&tr.taskCounts[modelName], 1)
}

// RecordTaskEnd decrements the load counter for a model
func (tr *TaskRouter) RecordTaskEnd(modelName string) {
	atomic.AddInt32(&tr.taskCounts[modelName], -1)
}

// GetModelLoad returns the current load for a model
func (tr *TaskRouter) GetModelLoad(modelName string) int32 {
	return atomic.LoadInt32(&tr.taskCounts[modelName])
}

// ExampleCustomRouter demonstrates implementing custom routing strategy.
// This example shows:
// 1. Creating a TaskRouter with intelligent routing logic
// 2. Load-balancing aware task distribution
// 3. Dynamic model selection based on task characteristics
// 4. Task type-based routing rules
//
// Run with: go run custom_router.go
func main() {
	// Initialize logging
	log.Initialize(false)
	defer log.Close()

	ctx := context.Background()

	// Set up model registry
	registry := ollama.NewModelRegistry()

	models := []struct {
		name        string
		description string
	}{
		{"llama2:7b", "Fast general-purpose"},
		{"mistral:7b", "Efficient instruction-following"},
		{"neural-chat:7b", "Conversational"},
	}

	for _, m := range models {
		metadata := &ollama.ModelMetadata{
			Name:        m.name,
			DisplayName: m.name,
			Description: m.description,
			Status:      ollama.ModelStatusAvailable,
		}
		config := &ollama.ModelConfig{
			Enabled:               true,
			MaxConcurrentRequests: 5,
			TimeoutSeconds:        60,
		}
		registry.RegisterModel(metadata, config)
	}

	registry.SetDefaultModel("llama2:7b")

	// Create custom task router
	router := NewTaskRouter(registry)

	fmt.Println("=== Custom Router Example ===")
	fmt.Println("Router configured with task-type routing rules:")
	fmt.Println("  - 'generation' → llama2:7b")
	fmt.Println("  - 'analysis' → mistral:7b")
	fmt.Println("  - 'optimization' → neural-chat:7b\n")

	// Create agent function that uses the router
	routingAgentFunc := func(ctx context.Context, task *ollama.Task) error {
		// Select model using router
		selectedModel := router.SelectBestModel(
			func() string {
				if payload, ok := task.Payload.(map[string]interface{}); ok {
					if taskType, ok := payload["type"].(string); ok {
						return taskType
					}
				}
				return "unknown"
			}(),
		)

		router.RecordTaskStart(selectedModel)
		defer router.RecordTaskEnd(selectedModel)

		payload := task.Payload.(map[string]interface{})
		taskType := payload["type"].(string)

		fmt.Printf("[Router] Task %s (type: %s) → model: %s\n",
			task.ID, taskType, selectedModel)
		fmt.Printf("[Router] Current model loads: ", selectedModel)
		for _, m := range registry.ListEnabledModels() {
			fmt.Printf("%s=%d ", m.Name, router.GetModelLoad(m.Name))
		}
		fmt.Println()

		// Simulate processing
		select {
		case <-time.After(time.Duration(500+int64(task.Priority)*100) * time.Millisecond):
			task.Result = map[string]interface{}{
				"routed_to": selectedModel,
				"task_type": taskType,
				"status":    "completed",
			}
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Create dispatcher with 3 workers
	dispatcher, err := ollama.NewTaskDispatcher(ctx, routingAgentFunc, 3)
	if err != nil {
		log.ErrorLog.Printf("Failed to create dispatcher: %v", err)
		return
	}

	dispatcher.SetProgressCallback(func(taskID string, status ollama.TaskStatus, progress int, message string) {
		if progress == 0 || progress == 100 {
			fmt.Printf("[Progress] %s: %s\n", taskID, message)
		}
	})

	if err := dispatcher.Start(); err != nil {
		log.ErrorLog.Printf("Failed to start dispatcher: %v", err)
		return
	}

	// Create diverse tasks that will be routed to different models
	fmt.Println("\nSubmitting tasks with different types for intelligent routing...\n")

	taskConfigs := []struct {
		id    string
		ttype string
	}{
		{"task-001", "generation"},
		{"task-002", "analysis"},
		{"task-003", "optimization"},
		{"task-004", "generation"},
		{"task-005", "analysis"},
		{"task-006", "generation"},
		{"task-007", "optimization"},
		{"task-008", "analysis"},
		{"task-009", "generation"},
		{"task-010", "optimization"},
	}

	for _, tc := range taskConfigs {
		task := &ollama.Task{
			ID:       tc.id,
			Priority: ollama.PriorityNormal,
			Payload: map[string]interface{}{
				"type": tc.ttype,
			},
		}
		dispatcher.SubmitTask(task)
	}

	fmt.Println("Monitoring execution...\n")

	// Monitor in background
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		metrics := dispatcher.GetMetrics()
		if metrics.CompletedTasks+metrics.FailedTasks == metrics.TotalTasks &&
			metrics.TotalTasks > 0 {
			break
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Display results
	fmt.Println("\n=== Routing Results ===")
	routingStats := make(map[string]int)

	for _, tc := range taskConfigs {
		task, _ := dispatcher.GetTask(tc.id)
		if task.Result != nil {
			result := task.Result.(map[string]interface{})
			model := result["routed_to"].(string)
			routingStats[model]++
			fmt.Printf("Task %s (type: %s) → %s\n", tc.id, tc.ttype, model)
		}
	}

	fmt.Println("\n=== Router Statistics ===")
	fmt.Println("Tasks routed to each model:")
	for model, count := range routingStats {
		fmt.Printf("  %s: %d tasks\n", model, count)
	}

	// Overall metrics
	metrics := dispatcher.GetMetrics()
	fmt.Println("\n=== Overall Metrics ===")
	fmt.Printf("Total Tasks: %d\n", metrics.TotalTasks)
	fmt.Printf("Completed: %d\n", metrics.CompletedTasks)
	fmt.Printf("Success Rate: %.1f%%\n",
		float64(metrics.CompletedTasks)/float64(metrics.TotalTasks)*100)

	// Shutdown
	fmt.Println("\nShutting down...")
	if err := dispatcher.Shutdown(10 * time.Second); err != nil {
		log.ErrorLog.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Done!")
}
