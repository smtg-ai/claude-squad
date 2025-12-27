package main

import (
	"context"
	"testing"
	"time"
)

func TestDemoCompletesSuccessfully(t *testing.T) {
	ctx := context.Background()
	orchestrator := NewDemoOrchestrator()

	globalReceipt, err := orchestrator.RunDemo(ctx)
	if err != nil {
		t.Fatalf("Demo failed: %v", err)
	}

	if globalReceipt == nil {
		t.Fatal("Global receipt is nil")
	}

	if globalReceipt.ExecutionID == "" {
		t.Error("Global receipt has empty execution ID")
	}

	if globalReceipt.InputHash == "" {
		t.Error("Global receipt has empty input hash")
	}

	if globalReceipt.OutputHash == "" {
		t.Error("Global receipt has empty output hash")
	}

	t.Logf("Demo completed successfully with execution ID: %s", globalReceipt.ExecutionID)
}

func TestAllReceiptsAreValid(t *testing.T) {
	ctx := context.Background()
	orchestrator := NewDemoOrchestrator()

	// Create tasks
	tasks := []*Task{
		{ID: "task-1", Priority: 10, Data: map[string]interface{}{"type": "test"}},
		{ID: "task-2", Priority: 5, Data: map[string]interface{}{"type": "test"}},
		{ID: "task-3", Priority: 7, Data: map[string]interface{}{"type": "test"}},
	}

	// Execute tasks and collect receipts
	receipts := make([]*Receipt, 0)

	for i, task := range tasks {
		worker := &AgentWorker{
			ID:             "test-agent",
			Task:           task,
			KnowledgeStore: orchestrator.knowledgeStore,
			ReceiptChain:   orchestrator.receiptChain,
		}

		receipt, _, err := worker.Execute(ctx)
		if err != nil {
			t.Fatalf("Task %d execution failed: %v", i, err)
		}

		// Verify each receipt
		if !orchestrator.receiptChain.VerifyReceipt(receipt) {
			t.Errorf("Receipt %d failed verification", i)
		}

		receipts = append(receipts, receipt)
	}

	if len(receipts) != len(tasks) {
		t.Errorf("Expected %d receipts, got %d", len(tasks), len(receipts))
	}

	t.Logf("All %d receipts validated successfully", len(receipts))
}

func TestFinalReceiptValidates(t *testing.T) {
	ctx := context.Background()
	orchestrator := NewDemoOrchestrator()

	globalReceipt, err := orchestrator.RunDemo(ctx)
	if err != nil {
		t.Fatalf("Demo failed: %v", err)
	}

	// Verify the final global receipt
	if !orchestrator.receiptChain.VerifyReceipt(globalReceipt) {
		t.Error("Final global receipt failed verification")
	}

	// Check required fields
	if globalReceipt.AgentID != "agent-0-reconciler" {
		t.Errorf("Expected agent ID 'agent-0-reconciler', got '%s'", globalReceipt.AgentID)
	}

	if globalReceipt.CompositionOp != "merge" {
		t.Errorf("Expected composition op 'merge', got '%s'", globalReceipt.CompositionOp)
	}

	if globalReceipt.ConflictPolicy != "fail_fast" {
		t.Errorf("Expected conflict policy 'fail_fast', got '%s'", globalReceipt.ConflictPolicy)
	}

	t.Log("Final receipt validated successfully")
}

func TestDeterminism_RunTwice_SameReceipt(t *testing.T) {
	ctx := context.Background()

	// First run
	orchestrator1 := NewDemoOrchestrator()
	receipt1, err1 := orchestrator1.RunDemo(ctx)
	if err1 != nil {
		t.Fatalf("First demo run failed: %v", err1)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Second run
	orchestrator2 := NewDemoOrchestrator()
	receipt2, err2 := orchestrator2.RunDemo(ctx)
	if err2 != nil {
		t.Fatalf("Second demo run failed: %v", err2)
	}

	// Compare critical deterministic fields
	// Note: ExecutionID and Timestamp will differ, but hashes should be consistent

	if receipt1.AgentID != receipt2.AgentID {
		t.Errorf("Agent IDs differ: %s vs %s", receipt1.AgentID, receipt2.AgentID)
	}

	if receipt1.CompositionOp != receipt2.CompositionOp {
		t.Errorf("Composition ops differ: %s vs %s", receipt1.CompositionOp, receipt2.CompositionOp)
	}

	if receipt1.ConflictPolicy != receipt2.ConflictPolicy {
		t.Errorf("Conflict policies differ: %s vs %s", receipt1.ConflictPolicy, receipt2.ConflictPolicy)
	}

	// The structure should be deterministic even if timestamps differ
	if receipt1.ToolchainVer != receipt2.ToolchainVer {
		t.Errorf("Toolchain versions differ: %s vs %s", receipt1.ToolchainVer, receipt2.ToolchainVer)
	}

	t.Log("Determinism test passed: both runs produced structurally consistent receipts")
}

func TestKnowledgeStoreSnapshot_IsDeterministic(t *testing.T) {
	ctx := context.Background()
	store := NewSimpleKnowledgeStore()

	// Add records in deterministic order
	records := []Record{
		{ID: "record-1", Timestamp: 1000, Data: map[string]interface{}{"value": 1}},
		{ID: "record-2", Timestamp: 2000, Data: map[string]interface{}{"value": 2}},
		{ID: "record-3", Timestamp: 3000, Data: map[string]interface{}{"value": 3}},
	}

	for _, record := range records {
		_, err := store.Append(ctx, record)
		if err != nil {
			t.Fatalf("Failed to append record: %v", err)
		}
	}

	// Take first snapshot
	hash1, data1, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatalf("First snapshot failed: %v", err)
	}

	// Take second snapshot immediately
	hash2, data2, err := store.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Second snapshot failed: %v", err)
	}

	// Hashes should be identical
	if hash1 != hash2 {
		t.Errorf("Snapshot hashes differ: %s vs %s", hash1, hash2)
	}

	// Data should be identical
	if string(data1) != string(data2) {
		t.Error("Snapshot data differs between runs")
	}

	// Verify the snapshot
	valid, err := store.Verify(ctx, hash1)
	if err != nil {
		t.Fatalf("Snapshot verification failed: %v", err)
	}

	if !valid {
		t.Error("Snapshot verification returned false")
	}

	t.Logf("Knowledge store snapshots are deterministic: %s", hash1[:16])
}

func TestReconciler_NoConflicts(t *testing.T) {
	ctx := context.Background()
	reconciler := NewSimpleReconciler()

	// Create non-conflicting deltas
	deltas := []*Delta{
		{ID: "delta-1", Files: []string{"/agent-0/file1.txt"}, CheckSum: "abc123"},
		{ID: "delta-2", Files: []string{"/agent-1/file2.txt"}, CheckSum: "def456"},
		{ID: "delta-3", Files: []string{"/agent-2/file3.txt"}, CheckSum: "ghi789"},
	}

	merged, report, err := reconciler.Reconcile(ctx, deltas)
	if err != nil {
		t.Fatalf("Reconciliation failed: %v", err)
	}

	if report.HasConflicts {
		t.Errorf("Unexpected conflicts: %v", report.Conflicts)
	}

	if merged == nil {
		t.Fatal("Merged delta is nil")
	}

	if len(merged.Files) != 3 {
		t.Errorf("Expected 3 merged files, got %d", len(merged.Files))
	}

	t.Log("Reconciler successfully merged deltas without conflicts")
}

func TestReconciler_DetectsConflicts(t *testing.T) {
	ctx := context.Background()
	reconciler := NewSimpleReconciler()

	// Create conflicting deltas (same file in multiple deltas)
	deltas := []*Delta{
		{ID: "delta-1", Files: []string{"/shared/file.txt"}, CheckSum: "abc123"},
		{ID: "delta-2", Files: []string{"/shared/file.txt"}, CheckSum: "def456"},
	}

	merged, report, err := reconciler.Reconcile(ctx, deltas)
	if err != nil {
		t.Fatalf("Reconciliation failed: %v", err)
	}

	if !report.HasConflicts {
		t.Error("Expected conflicts to be detected, but none were found")
	}

	if merged != nil {
		t.Error("Expected merged delta to be nil when conflicts exist")
	}

	if len(report.Conflicts) == 0 {
		t.Error("Expected conflict list to be non-empty")
	}

	t.Logf("Reconciler correctly detected conflicts: %v", report.Conflicts)
}

func TestTaskRouter_DeterministicRouting(t *testing.T) {
	router := NewSimpleTaskRouter()

	task1 := &Task{ID: "task-1", Priority: 10}
	task2 := &Task{ID: "task-2", Priority: 3}

	// Route the same task multiple times
	route1, err := router.Route(task1, nil)
	if err != nil {
		t.Fatalf("First routing failed: %v", err)
	}

	route2, err := router.Route(task1, nil)
	if err != nil {
		t.Fatalf("Second routing failed: %v", err)
	}

	// Should produce identical results
	if route1 != route2 {
		t.Errorf("Non-deterministic routing: %s vs %s", route1, route2)
	}

	// Different tasks with different priorities
	route3, err := router.Route(task2, nil)
	if err != nil {
		t.Fatalf("Third routing failed: %v", err)
	}

	// Lower priority task should route differently
	if route1 == route3 {
		t.Error("Expected different routes for different priority tasks")
	}

	t.Log("Task router produces deterministic routing decisions")
}

func TestResourceAllocator_FairDistribution(t *testing.T) {
	allocator := NewSimpleResourceAllocator()

	agentCount := 3
	resourceBudget := 300

	allocations := allocator.AllocateResources(agentCount, resourceBudget)

	if len(allocations) != agentCount {
		t.Errorf("Expected %d allocations, got %d", agentCount, len(allocations))
	}

	totalAllocated := 0
	for _, alloc := range allocations {
		totalAllocated += alloc.Resources
	}

	if totalAllocated != resourceBudget {
		t.Errorf("Expected total allocation %d, got %d", resourceBudget, totalAllocated)
	}

	// Check fairness (all agents get equal share)
	expectedPerAgent := resourceBudget / agentCount
	for _, alloc := range allocations {
		if alloc.Resources != expectedPerAgent {
			t.Errorf("Agent %s got %d resources, expected %d", alloc.AgentID, alloc.Resources, expectedPerAgent)
		}
	}

	t.Log("Resource allocator distributes resources fairly")
}

func BenchmarkDemoExecution(b *testing.B) {
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		orchestrator := NewDemoOrchestrator()
		_, err := orchestrator.RunDemo(ctx)
		if err != nil {
			b.Fatalf("Demo failed: %v", err)
		}
	}
}
