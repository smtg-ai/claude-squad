package main

import (
	"claude-squad/log"
	"claude-squad/ollama"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ExampleAiderIntegration demonstrates using Aider with multiple Ollama models.
// This example shows:
// 1. Configuring multiple Ollama models for Aider
// 2. Routing different task types to different models
// 3. Integration with Aider's command-line interface
// 4. Handling file-based interactions
//
// Prerequisites:
// - Ollama running locally (http://localhost:11434)
// - Aider installed: pip install aider-chat
// - One or more Ollama models available: ollama list
//
// Run with: go run aider_integration.go
func main() {
	// Initialize logging
	log.Initialize(false)
	defer log.Close()

	ctx := context.Background()

	// Initialize model registry with multiple Ollama models
	registry := ollama.NewModelRegistry()

	// Register different Ollama models with varying capabilities
	models := []struct {
		name        string
		displayName string
		description string
		contextSize int
		useCase     string
	}{
		{
			name:        "llama2:7b",
			displayName: "Llama 2 7B",
			description: "Fast, general-purpose model",
			contextSize: 4096,
			useCase:     "code_generation",
		},
		{
			name:        "mistral:7b",
			displayName: "Mistral 7B",
			description: "Efficient instruction-following model",
			contextSize: 8192,
			useCase:     "code_review",
		},
		{
			name:        "neural-chat:7b",
			displayName: "Neural Chat 7B",
			description: "Optimized for conversational tasks",
			contextSize: 4096,
			useCase:     "documentation",
		},
	}

	// Register models with specific configurations
	for _, m := range models {
		metadata := &ollama.ModelMetadata{
			Name:        m.name,
			FullName:    m.name,
			DisplayName: m.displayName,
			Description: m.description,
			Status:      ollama.ModelStatusAvailable,
			Attributes: map[string]interface{}{
				"context_window": m.contextSize,
				"use_case":       m.useCase,
				"provider":       "ollama",
			},
		}

		config := &ollama.ModelConfig{
			Enabled:               true,
			Priority:              0,
			MaxConcurrentRequests: 2,
			TimeoutSeconds:        120,
			RequestOptions: ollama.RequestOptions{
				Stream:      true,
				Temperature: 0.7,
				TopK:        40,
				TopP:        0.9,
			},
			Labels: []string{"aider-compatible", m.useCase},
		}

		if err := registry.RegisterModel(metadata, config); err != nil {
			log.ErrorLog.Printf("Failed to register model %s: %v", m.name, err)
		}
	}

	// Set default model
	registry.SetDefaultModel("llama2:7b")

	fmt.Println("=== Aider Integration Example ===")
	fmt.Printf("Registered %d Ollama models\n\n", len(models))

	// Display registered models
	fmt.Println("Available models:")
	for _, model := range registry.ListEnabledModels() {
		fmt.Printf("  - %s (%s)\n", model.DisplayName, model.Name)
		if useCase, ok := model.Attributes["use_case"]; ok {
			fmt.Printf("    Use case: %v\n", useCase)
		}
	}
	fmt.Println()

	// Create agent function that invokes Aider with specific models
	aiderFunc := func(ctx context.Context, task *ollama.Task) error {
		payload := task.Payload.(map[string]interface{})
		filePath := payload["file"].(string)
		instruction := payload["instruction"].(string)
		modelName := payload["model"].(string)

		fmt.Printf("[Aider] Processing file %s with model %s\n", filePath, modelName)
		fmt.Printf("[Aider] Instruction: %s\n", instruction)

		// Build Aider command
		// Format: aider --model ollama/model-name [options] file
		cmd := exec.CommandContext(ctx,
			"aider",
			"--model", fmt.Sprintf("ollama_chat/%s", modelName),
			"--yes",             // Auto-accept changes
			"--check-update=no", // Disable update checks
			"--no-git",          // Don't auto-git for this example
			filePath,
		)

		// For this example, we'll simulate the Aider execution
		// In production, you would capture the output and handle results
		fmt.Printf("[Aider] Executing: %v\n", cmd.Args)

		// Simulate execution (actual Aider would process the file here)
		select {
		case <-time.After(3 * time.Second):
			task.Result = map[string]interface{}{
				"model":        modelName,
				"file":         filePath,
				"status":       "completed",
				"changes_made": true,
				"instruction":  instruction,
				"timestamp":    time.Now(),
				"simulated":    true, // Indicates this is simulated output
			}
			fmt.Printf("[Aider] Completed processing of %s\n", filePath)
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Create dispatcher with 3 workers for parallel Aider invocations
	dispatcher, err := ollama.NewTaskDispatcher(ctx, aiderFunc, 3)
	if err != nil {
		log.ErrorLog.Printf("Failed to create dispatcher: %v", err)
		return
	}

	dispatcher.SetProgressCallback(func(taskID string, status ollama.TaskStatus, progress int, message string) {
		if progress == 0 || progress == 100 {
			fmt.Printf("[Status] %s: %s\n", taskID, message)
		}
	})

	if err := dispatcher.Start(); err != nil {
		log.ErrorLog.Printf("Failed to start dispatcher: %v", err)
		return
	}

	// Create sample tasks for different code files with different models
	fmt.Println("\nSubmitting tasks...\n")

	sampleTasks := []struct {
		id          string
		file        string
		instruction string
		model       string
	}{
		{
			id:          "task-001",
			file:        "main.go",
			instruction: "Add error handling and improve function documentation",
			model:       "llama2:7b",
		},
		{
			id:          "task-002",
			file:        "utils.go",
			instruction: "Review code for performance issues and optimize",
			model:       "mistral:7b",
		},
		{
			id:          "task-003",
			file:        "README.md",
			instruction: "Improve documentation clarity and add examples",
			model:       "neural-chat:7b",
		},
		{
			id:          "task-004",
			file:        "config.go",
			instruction: "Add validation and improve configuration handling",
			model:       "llama2:7b",
		},
	}

	// Submit tasks
	for _, st := range sampleTasks {
		task := &ollama.Task{
			ID:       st.id,
			Priority: ollama.PriorityNormal,
			Payload: map[string]interface{}{
				"file":        st.file,
				"instruction": st.instruction,
				"model":       st.model,
			},
		}

		if err := dispatcher.SubmitTask(task); err != nil {
			log.ErrorLog.Printf("Failed to submit task: %v", err)
		}
	}

	// Wait for completion
	time.Sleep(15 * time.Second)

	// Display results
	fmt.Println("\n=== Task Results ===")
	for _, st := range sampleTasks {
		task, err := dispatcher.GetTask(st.id)
		if err != nil {
			fmt.Printf("Error retrieving task %s: %v\n", st.id, err)
			continue
		}

		fmt.Printf("\nTask: %s\n", task.ID)
		fmt.Printf("  File: %s\n", st.file)
		fmt.Printf("  Model: %s\n", st.model)
		fmt.Printf("  Status: %s\n", task.Status.String())
		fmt.Printf("  Duration: %v\n", task.CompletedAt.Sub(task.StartedAt))

		if task.Result != nil {
			result := task.Result.(map[string]interface{})
			if changes, ok := result["changes_made"].(bool); ok {
				fmt.Printf("  Changes Made: %v\n", changes)
			}
		}

		if task.Error != nil {
			fmt.Printf("  Error: %v\n", task.Error)
		}
	}

	// Display metrics
	metrics := dispatcher.GetMetrics()
	fmt.Println("\n=== Dispatcher Metrics ===")
	fmt.Printf("Total Tasks: %d\n", metrics.TotalTasks)
	fmt.Printf("Completed: %d\n", metrics.CompletedTasks)
	fmt.Printf("Failed: %d\n", metrics.FailedTasks)
	fmt.Printf("Success Rate: %.1f%%\n",
		float64(metrics.CompletedTasks)/float64(metrics.TotalTasks)*100)

	// Display model registry stats
	fmt.Println("\n=== Model Registry ===")
	enabledModels := registry.ListEnabledModels()
	fmt.Printf("Enabled Models: %d\n", len(enabledModels))
	for _, model := range enabledModels {
		fmt.Printf("  - %s\n", model.Name)
	}

	// Shutdown
	fmt.Println("\nShutting down...")
	if err := dispatcher.Shutdown(10 * time.Second); err != nil {
		log.ErrorLog.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Done!")
}
