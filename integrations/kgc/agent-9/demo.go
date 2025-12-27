package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// DemoOrchestrator coordinates the end-to-end demo
type DemoOrchestrator struct {
	knowledgeStore    KnowledgeStore
	receiptChain      ReceiptChain
	resourceAllocator ResourceAllocator
	taskRouter        TaskRouter
	reconciler        Reconciler
}

// NewDemoOrchestrator creates a new orchestrator
func NewDemoOrchestrator() *DemoOrchestrator {
	return &DemoOrchestrator{
		knowledgeStore:    NewSimpleKnowledgeStore(),
		receiptChain:      NewSimpleReceiptChain(),
		resourceAllocator: NewSimpleResourceAllocator(),
		taskRouter:        NewSimpleTaskRouter(),
		reconciler:        NewSimpleReconciler(),
	}
}

// AgentWorker simulates an agent executing a task
type AgentWorker struct {
	ID            string
	Task          *Task
	KnowledgeStore KnowledgeStore
	ReceiptChain   ReceiptChain
}

// Execute runs the agent task and produces a receipt
func (w *AgentWorker) Execute(ctx context.Context) (*Receipt, *Delta, error) {
	// 1. Record task initiation in knowledge store
	inputRecord := Record{
		ID:        fmt.Sprintf("%s-input-%s", w.ID, w.Task.ID),
		Timestamp: time.Now().UnixNano(),
		Data: map[string]interface{}{
			"agent_id": w.ID,
			"task_id":  w.Task.ID,
			"phase":    "input",
		},
	}

	inputHash, err := w.KnowledgeStore.Append(ctx, inputRecord)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to append input record: %w", err)
	}

	// 2. Simulate task execution (deterministic work)
	time.Sleep(10 * time.Millisecond) // Simulate work

	// 3. Record task completion in knowledge store
	outputRecord := Record{
		ID:        fmt.Sprintf("%s-output-%s", w.ID, w.Task.ID),
		Timestamp: time.Now().UnixNano(),
		Data: map[string]interface{}{
			"agent_id": w.ID,
			"task_id":  w.Task.ID,
			"phase":    "output",
			"result":   fmt.Sprintf("completed-task-%s", w.Task.ID),
		},
	}

	outputHash, err := w.KnowledgeStore.Append(ctx, outputRecord)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to append output record: %w", err)
	}

	// 4. Create receipt for this execution
	replayScript := fmt.Sprintf(`#!/bin/bash
# Replay script for %s executing task %s
echo "Task: %s"
echo "Agent: %s"
echo "Priority: %d"
`, w.ID, w.Task.ID, w.Task.ID, w.ID, w.Task.Priority)

	receipt := w.ReceiptChain.CreateReceipt(
		fmt.Sprintf("exec-%s-%s", w.ID, w.Task.ID),
		w.ID,
		inputHash,
		outputHash,
		replayScript,
	)

	// 5. Create delta representing changes
	delta := &Delta{
		ID:       fmt.Sprintf("delta-%s-%s", w.ID, w.Task.ID),
		Files:    []string{fmt.Sprintf("/agent-%s/%s.result", w.ID, w.Task.ID)},
		Receipt:  receipt,
		CheckSum: fmt.Sprintf("%x", sha256.Sum256([]byte(w.ID+w.Task.ID))),
	}

	return receipt, delta, nil
}

// RunDemo executes the complete end-to-end demonstration
func (d *DemoOrchestrator) RunDemo(ctx context.Context) (*Receipt, error) {
	fmt.Println("=== KGC Multi-Agent Demo Starting ===")
	startTime := time.Now()

	// Step 1: Initialize knowledge store
	fmt.Println("\n[Step 1] Initializing knowledge store...")
	initRecord := Record{
		ID:        "init-record",
		Timestamp: time.Now().UnixNano(),
		Data: map[string]interface{}{
			"demo":    "kgc-multi-agent",
			"version": "v0.1.0",
		},
	}
	_, err := d.knowledgeStore.Append(ctx, initRecord)
	if err != nil {
		return nil, fmt.Errorf("knowledge store init failed: %w", err)
	}
	snapshotHash, _, _ := d.knowledgeStore.Snapshot(ctx)
	fmt.Printf("  ✓ Knowledge store initialized (snapshot: %s...)\n", snapshotHash[:16])

	// Step 2: Create 3+ concurrent tasks
	fmt.Println("\n[Step 2] Creating 3+ concurrent tasks...")
	tasks := []*Task{
		{ID: "task-1", Priority: 10, Data: map[string]interface{}{"type": "analysis"}, Predicate: "high"},
		{ID: "task-2", Priority: 5, Data: map[string]interface{}{"type": "synthesis"}, Predicate: "normal"},
		{ID: "task-3", Priority: 7, Data: map[string]interface{}{"type": "validation"}, Predicate: "normal"},
		{ID: "task-4", Priority: 3, Data: map[string]interface{}{"type": "reporting"}, Predicate: "low"},
	}
	fmt.Printf("  ✓ Created %d tasks\n", len(tasks))

	// Step 3: Route tasks deterministically
	fmt.Println("\n[Step 3] Routing tasks deterministically...")
	sortedTasks, err := d.taskRouter.EvaluateTaskGraph(tasks)
	if err != nil {
		return nil, fmt.Errorf("task routing failed: %w", err)
	}
	fmt.Printf("  ✓ Tasks sorted by priority: ")
	for _, t := range sortedTasks {
		fmt.Printf("%s(P%d) ", t.ID, t.Priority)
	}
	fmt.Println()

	// Step 4: Allocate resources
	fmt.Println("\n[Step 4] Allocating resources...")
	agentCount := 3
	resourceBudget := 300
	allocations := d.resourceAllocator.AllocateResources(agentCount, resourceBudget)
	for _, alloc := range allocations {
		fmt.Printf("  ✓ %s allocated %d resources\n", alloc.AgentID, alloc.Resources)
	}

	// Create agent pool
	agents := []string{"agent-0", "agent-1", "agent-2"}
	schedule := d.resourceAllocator.RoundRobinSchedule(agents, sortedTasks)

	// Step 5: Execute tasks concurrently, each producing a receipt
	fmt.Println("\n[Step 5] Executing tasks concurrently...")

	var wg sync.WaitGroup
	receiptChan := make(chan *Receipt, len(tasks))
	deltaChan := make(chan *Delta, len(tasks))
	errorChan := make(chan error, len(tasks))

	for agentID, agentTasks := range schedule {
		for _, task := range agentTasks {
			wg.Add(1)
			go func(aid string, t *Task) {
				defer wg.Done()

				worker := &AgentWorker{
					ID:             aid,
					Task:           t,
					KnowledgeStore: d.knowledgeStore,
					ReceiptChain:   d.receiptChain,
				}

				receipt, delta, err := worker.Execute(ctx)
				if err != nil {
					errorChan <- err
					return
				}

				receiptChan <- receipt
				deltaChan <- delta
				execIDPreview := receipt.ExecutionID
				if len(execIDPreview) > 20 {
					execIDPreview = execIDPreview[:20] + "..."
				}
				fmt.Printf("  ✓ %s completed %s (receipt: %s)\n", aid, t.ID, execIDPreview)
			}(agentID, task)
		}
	}

	// Wait for all tasks to complete
	wg.Wait()
	close(receiptChan)
	close(deltaChan)
	close(errorChan)

	// Check for errors
	if len(errorChan) > 0 {
		return nil, <-errorChan
	}

	// Collect all receipts and deltas
	receipts := make([]*Receipt, 0)
	deltas := make([]*Delta, 0)

	for receipt := range receiptChan {
		receipts = append(receipts, receipt)
	}

	for delta := range deltaChan {
		deltas = append(deltas, delta)
	}

	fmt.Printf("  ✓ All %d tasks completed successfully\n", len(receipts))

	// Step 6: Reconciler validates all
	fmt.Println("\n[Step 6] Reconciler validating all deltas...")
	mergedDelta, conflictReport, err := d.reconciler.Reconcile(ctx, deltas)
	if err != nil {
		return nil, fmt.Errorf("reconciliation failed: %w", err)
	}

	if conflictReport.HasConflicts {
		fmt.Println("  ✗ Conflicts detected:")
		for _, conflict := range conflictReport.Conflicts {
			fmt.Printf("    - %s\n", conflict)
		}
		return nil, fmt.Errorf("reconciliation conflicts detected")
	}

	fmt.Printf("  ✓ No conflicts detected, merged %d deltas\n", len(deltas))
	fmt.Printf("  ✓ Merged delta checksum: %s...\n", mergedDelta.CheckSum[:16])

	// Step 7: Create and print final global receipt
	fmt.Println("\n[Step 7] Creating final global receipt...")
	globalReceipt, err := d.receiptChain.ChainReceipts(receipts)
	if err != nil {
		return nil, fmt.Errorf("receipt chaining failed: %w", err)
	}

	// Verify the global receipt
	if !d.receiptChain.VerifyReceipt(globalReceipt) {
		return nil, fmt.Errorf("global receipt verification failed")
	}

	fmt.Printf("  ✓ Global receipt created and verified\n")
	fmt.Printf("  ✓ Global execution ID: %s\n", globalReceipt.ExecutionID)
	fmt.Printf("  ✓ Global input hash: %s...\n", globalReceipt.InputHash[:16])
	fmt.Printf("  ✓ Global output hash: %s...\n", globalReceipt.OutputHash[:16])

	// Final snapshot
	finalSnapshotHash, _, _ := d.knowledgeStore.Snapshot(ctx)
	fmt.Printf("  ✓ Final knowledge store snapshot: %s...\n", finalSnapshotHash[:16])

	duration := time.Since(startTime)
	fmt.Printf("\n=== Demo Completed Successfully in %s ===\n", duration)

	return globalReceipt, nil
}

func main() {
	ctx := context.Background()

	orchestrator := NewDemoOrchestrator()
	globalReceipt, err := orchestrator.RunDemo(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Demo failed: %v\n", err)
		os.Exit(1)
	}

	// Pretty print the global receipt
	fmt.Println("\n=== Final Global Receipt ===")
	receiptJSON, _ := json.MarshalIndent(globalReceipt, "", "  ")
	fmt.Println(string(receiptJSON))

	os.Exit(0)
}
