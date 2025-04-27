package orchestrator

import (
	"claude-squad/session"
	"claude-squad/session/git"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Orchestrator manages the orchestration of multiple worker instances to achieve a goal.
type Orchestrator struct {
	Prompt    string
	Workers   map[string]*session.Instance
	AutoYes   bool
	mu        sync.Mutex
	Plan      []Task
	Completed map[string]bool
	Program   string // The program to run for workers and merge (defaults to "claude")
}

// Task represents a subdivided work item for a worker.
type Task struct {
	Name   string
	Prompt string
}

// NewOrchestrator creates a new orchestrator with the given prompt and autoyes mode.
func NewOrchestrator(prompt string, autoyes bool) *Orchestrator {
	return &Orchestrator{
		Prompt:    prompt,
		Workers:   make(map[string]*session.Instance),
		AutoYes:   autoyes,
		Completed: make(map[string]bool),
		Program:   "claude", // Default program
	}
}

// SetProgram sets the program to use for workers and merge.
func (o *Orchestrator) SetProgram(program string) {
	o.Program = program
}

// DividePrompt splits the orchestrator's prompt into manageable tasks.
func (o *Orchestrator) DividePrompt() []Task {
	// We'll create a planner instance to analyze the prompt and break it down
	plannerPrompt := `You will analyze this goal: "` + o.Prompt + `"
	
Break this goal down into 2-5 separate distinct tasks that would be appropriate to delegate to different workers. 
Each task should be independent enough that it can be worked on separately.

For each task, provide:
1. A short, descriptive task name (use kebab-case, like "create-login-api")
2. A detailed prompt for the worker that will implement this task

Respond in the following format, with each task on its own line:
TASK: task-name | Detailed instructions for the worker to complete this specific task...
`

	// Create a planning instance to divide the work
	program := o.Program
	if program == "" {
		program = "claude" // Default fallback
	}

	plannerOpts := session.InstanceOptions{
		Title:   "orchestrator-planner",
		Path:    ".", // This will be overridden when the instance is created
		Program: program,
		AutoYes: o.AutoYes,
	}

	planner, err := session.NewInstance(plannerOpts)
	if err != nil {
		// Log the error but continue with a fallback
		fmt.Printf("Failed to create planner instance: %v\n", err)
		// Fallback to a single task
		return []Task{{Name: "main-task", Prompt: o.Prompt}}
	}

	// Start the planner instance
	err = planner.Start(true)
	if err != nil {
		fmt.Printf("Failed to start planner instance: %v\n", err)
		// Fallback to a single task
		return []Task{{Name: "main-task", Prompt: o.Prompt}}
	}

	// Send the planning prompt
	err = planner.SendPrompt(plannerPrompt)
	if err != nil {
		fmt.Printf("Failed to send prompt to planner: %v\n", err)
		// Fallback to a single task
		return []Task{{Name: "main-task", Prompt: o.Prompt}}
	}

	// Wait for the planner to respond (simplistic approach - in a real implementation we'd monitor for completion)
	time.Sleep(30 * time.Second)

	// Capture the planner's output
	output, err := planner.Preview()
	if err != nil {
		fmt.Printf("Failed to get preview: %v\n", err)
		// Fallback to a single task
		return []Task{{Name: "main-task", Prompt: o.Prompt}}
	}

	// Cleanup the planner
	if err := planner.Close(); err != nil {
		fmt.Printf("Failed to close planner: %v\n", err)
	}

	// Parse the output to extract tasks
	tasks := parsePlanOutput(output, o.Prompt)

	// If no tasks were parsed, fallback to a single task
	if len(tasks) == 0 {
		return []Task{{Name: "main-task", Prompt: o.Prompt}}
	}

	return tasks
}

// parsePlanOutput parses the output from the planner to extract tasks
func parsePlanOutput(output string, defaultPrompt string) []Task {
	var tasks []Task

	// Split by lines
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for lines that start with "TASK:"
		if strings.HasPrefix(line, "TASK:") {
			parts := strings.SplitN(line[5:], "|", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				prompt := strings.TrimSpace(parts[1])

				tasks = append(tasks, Task{
					Name:   name,
					Prompt: prompt,
				})
			}
		}
	}

	return tasks
}

// CreateWorkers creates worker instances for each task.
func (o *Orchestrator) CreateWorkers(basePath string) error {
	tasks := o.Plan
	if len(tasks) == 0 {
		tasks = o.DividePrompt()
		o.Plan = tasks
	}

	fmt.Printf("Creating %d worker instances...\n", len(tasks))

	for i, task := range tasks {
		fmt.Printf("Creating worker %d/%d: %s\n", i+1, len(tasks), task.Name)

		// Get the program to use - use the orchestrator's Program field
		program := o.Program
		if program == "" {
			program = "claude" // Default fallback
		}

		opts := session.InstanceOptions{
			Title:   task.Name,
			Path:    basePath,
			Program: program,
			AutoYes: o.AutoYes,
		}

		inst, err := session.NewInstance(opts)
		if err != nil {
			return fmt.Errorf("failed to create worker instance '%s': %w", task.Name, err)
		}

		// Start the instance
		if err := inst.Start(true); err != nil {
			return fmt.Errorf("failed to start worker instance '%s': %w", task.Name, err)
		}

		fmt.Printf("Sending task prompt to worker '%s'...\n", task.Name)

		// Send the task prompt to the worker
		if err := inst.SendPrompt(task.Prompt); err != nil {
			// Attempt to clean up the instance before returning error
			_ = inst.Close()
			return fmt.Errorf("failed to send prompt to worker '%s': %w", task.Name, err)
		}

		o.mu.Lock()
		o.Workers[task.Name] = inst
		o.mu.Unlock()

		fmt.Printf("Worker '%s' initialized successfully\n", task.Name)
	}

	fmt.Printf("All %d workers initialized successfully\n", len(tasks))
	return nil
}

// MonitorWorkers waits for all workers to complete and collects their diffs.
func (o *Orchestrator) MonitorWorkers() (map[string]*git.DiffStats, error) {
	results := make(map[string]*git.DiffStats)

	fmt.Println("Monitoring worker progress...")

	// Define maximum wait time for each worker
	maxWaitTime := 10 * time.Minute
	checkInterval := 5 * time.Second
	timeoutTicker := time.NewTicker(maxWaitTime)
	defer timeoutTicker.Stop()

	// Create a channel to signal when all workers are done
	allDone := make(chan bool)

	// Start a goroutine to check worker progress
	go func() {
		for {
			allCompleted := true

			o.mu.Lock()
			numWorkers := len(o.Workers)
			numCompleted := 0

			// Check status of all workers
			for name, inst := range o.Workers {
				if _, ok := o.Completed[name]; ok {
					numCompleted++
					continue
				}

				// Check if worker is still active
				updated, hasPrompt := inst.HasUpdated()
				if !updated && !hasPrompt {
					// Worker might be done, check its status
					o.Completed[name] = true
					numCompleted++
					fmt.Printf("Worker %s completed task\n", name)
				} else {
					allCompleted = false
				}

				// Update diff stats for the worker
				if err := inst.UpdateDiffStats(); err != nil {
					fmt.Printf("Warning: could not update diff stats for %s: %v\n", name, err)
				}
			}

			// Print progress
			fmt.Printf("Progress: %d/%d workers completed\n", numCompleted, numWorkers)

			o.mu.Unlock()

			if allCompleted {
				allDone <- true
				return
			}

			// Wait before checking again
			time.Sleep(checkInterval)
		}
	}()

	// Wait for all workers to complete or for timeout
	select {
	case <-allDone:
		fmt.Println("All workers have completed their tasks")
	case <-timeoutTicker.C:
		fmt.Println("WARNING: Maximum wait time reached, proceeding with available results")
	}

	// Collect the results
	o.mu.Lock()
	defer o.mu.Unlock()

	for name, inst := range o.Workers {
		if err := inst.UpdateDiffStats(); err != nil {
			fmt.Printf("Warning: could not update final diff stats for %s: %v\n", name, err)
		}

		stats := inst.GetDiffStats()
		results[name] = stats
		o.Completed[name] = true

		fmt.Printf("Collected diff stats from worker %s: +%d, -%d lines\n",
			name,
			stats.Added,
			stats.Removed)
	}

	return results, nil
}

// MergeDiffs creates a merge instance and uses AI to merge worker diffs, handling conflicts if needed.
func (o *Orchestrator) MergeDiffs(basePath string, diffs map[string]*git.DiffStats) (string, error) {
	// Check if we have any diffs to merge
	hasDiffs := false
	for _, diff := range diffs {
		if diff != nil && diff.Content != "" {
			hasDiffs = true
			break
		}
	}

	if !hasDiffs {
		return "No changes were made by any of the workers.", nil
	}

	// Prepare a merge prompt for the AI instance
	var sb strings.Builder
	sb.WriteString("You are a codebase merge orchestrator. Your task is to carefully analyze and combine the following diffs from multiple workers into a single coherent result.\n\n")
	sb.WriteString("IMPORTANT INSTRUCTIONS:\n")
	sb.WriteString("1. Analyze each worker's changes to understand what they modified\n")
	sb.WriteString("2. Identify any potential conflicts between workers' changes\n")
	sb.WriteString("3. Merge the changes intelligently, preserving the intent of each worker's contribution\n")
	sb.WriteString("4. When conflicts occur, select the most comprehensive solution and provide justification\n")
	sb.WriteString("5. If needed, make minor adjustments to ensure the merged code is cohesive and functional\n")
	sb.WriteString("6. Your output should be a single unified diff that can be applied to the codebase\n\n")
	sb.WriteString("Here are the worker diffs to merge:\n\n")

	// Add worker diffs to the prompt
	for name, diff := range diffs {
		if diff != nil && diff.Content != "" {
			sb.WriteString(fmt.Sprintf("===== WORKER: %s =====\n", name))
			sb.WriteString(fmt.Sprintf("%s\n\n", diff.Content))
		} else {
			sb.WriteString(fmt.Sprintf("===== WORKER: %s =====\n", name))
			sb.WriteString("No diff available\n\n")
		}
	}

	sb.WriteString("Analyze all the diffs and create a final unified diff that correctly combines all changes. For any conflicts, provide a brief comment in your diff explaining your resolution approach.\n")

	mergePrompt := sb.String()

	fmt.Println("Creating merge instance to combine worker changes...")

	// Create a dedicated merge instance
	program := o.Program
	if program == "" {
		program = "claude" // Default fallback
	}

	mergeOpts := session.InstanceOptions{
		Title:   "merge-orchestrator",
		Path:    basePath,
		Program: program,
		AutoYes: o.AutoYes,
	}
	mergeInstance, err := session.NewInstance(mergeOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create merge instance: %w", err)
	}

	// Start the merge instance
	err = mergeInstance.Start(true)
	if err != nil {
		return "", fmt.Errorf("failed to start merge instance: %w", err)
	}

	// Send the merge prompt
	if err := mergeInstance.SendPrompt(mergePrompt); err != nil {
		return "", fmt.Errorf("failed to send merge prompt: %w", err)
	}

	fmt.Println("Waiting for merge to complete...")

	// Wait for merge to complete (up to 5 minutes)
	maxWaitTime := 5 * time.Minute
	checkInterval := 5 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		// Check if worker is still active
		updated, hasPrompt := mergeInstance.HasUpdated()
		if !updated && !hasPrompt {
			// Worker is likely done
			break
		}

		// Update diff stats
		if err := mergeInstance.UpdateDiffStats(); err != nil {
			fmt.Printf("Warning: could not update merge diff stats: %v\n", err)
		}

		// Check if we have a diff yet
		diffStats := mergeInstance.GetDiffStats()
		if diffStats != nil && diffStats.Content != "" {
			fmt.Println("Diff changes detected, waiting for completion...")
		}

		// Wait before checking again
		time.Sleep(checkInterval)
	}

	// Final update of diff stats
	if err := mergeInstance.UpdateDiffStats(); err != nil {
		fmt.Printf("Warning: could not update final merge diff stats: %v\n", err)
	}

	// Get the diff from the merge instance
	mergeDiff := mergeInstance.GetDiffStats()

	// Close the merge instance
	if err := mergeInstance.Close(); err != nil {
		fmt.Printf("Warning: could not close merge instance: %v\n", err)
	}

	if mergeDiff != nil && mergeDiff.Content != "" {
		fmt.Printf("Merge completed successfully: +%d, -%d lines\n", mergeDiff.Added, mergeDiff.Removed)
		return mergeDiff.Content, nil
	}

	return "", fmt.Errorf("merge instance did not produce a diff")
}

// Run executes the orchestration process.
func (o *Orchestrator) Run(basePath string) (string, error) {
	fmt.Println("========= Starting Orchestration =========")
	fmt.Println("1. Creating worker instances...")
	if err := o.CreateWorkers(basePath); err != nil {
		return "", fmt.Errorf("failed to create workers: %w", err)
	}

	fmt.Println("\n2. Monitoring workers and collecting results...")
	diffs, err := o.MonitorWorkers()
	if err != nil {
		return "", fmt.Errorf("error monitoring workers: %w", err)
	}

	fmt.Println("\n3. Merging results from workers...")
	merged, err := o.MergeDiffs(basePath, diffs)
	if err != nil {
		return "", fmt.Errorf("error merging diffs: %w", err)
	}

	fmt.Println("\n========= Orchestration Complete =========")

	// Cleanup workers
	fmt.Println("Cleaning up worker instances...")
	for name, inst := range o.Workers {
		if err := inst.Close(); err != nil {
			fmt.Printf("Warning: could not properly close worker %s: %v\n", name, err)
		} else {
			fmt.Printf("Worker %s cleaned up successfully\n", name)
		}
	}

	return merged, nil
}
