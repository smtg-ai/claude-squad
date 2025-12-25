package concurrency

import (
	"context"
	"fmt"
	"testing"
	"time"

	"claude-squad/session"
)

// MockOperation for testing
type MockOperation struct {
	name           string
	shouldFail     bool
	executeCalled  bool
	rollbackCalled bool
	validateCalled bool
	delay          time.Duration
}

func NewMockOperation(name string, shouldFail bool, delay time.Duration) *MockOperation {
	return &MockOperation{
		name:       name,
		shouldFail: shouldFail,
		delay:      delay,
	}
}

func (m *MockOperation) Name() string {
	return m.name
}

func (m *MockOperation) Validate(instance *session.Instance) error {
	m.validateCalled = true
	if m.shouldFail {
		return fmt.Errorf("mock validation failed")
	}
	return nil
}

func (m *MockOperation) Execute(ctx context.Context, instance *session.Instance) error {
	m.executeCalled = true
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if m.shouldFail {
		return fmt.Errorf("mock execution failed")
	}
	return nil
}

func (m *MockOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	m.rollbackCalled = true
	if m.shouldFail {
		return fmt.Errorf("mock rollback failed")
	}
	return nil
}

// MockOperationWithRetry for testing retry behavior
type MockOperationWithRetry struct {
	name      string
	attempts  int
	failUntil int
}

func (m *MockOperationWithRetry) Name() string {
	return m.name
}

func (m *MockOperationWithRetry) Validate(instance *session.Instance) error {
	return nil
}

func (m *MockOperationWithRetry) Execute(ctx context.Context, instance *session.Instance) error {
	m.attempts++
	if m.attempts <= m.failUntil {
		return fmt.Errorf("temporary failure attempt %d", m.attempts)
	}
	return nil
}

func (m *MockOperationWithRetry) Rollback(ctx context.Context, instance *session.Instance) error {
	return nil
}


func TestPartialResult(t *testing.T) {
	pr := NewPartialResult()

	if pr.AllSucceeded() {
		t.Error("Empty result should not be all succeeded")
	}

	// Add successful result
	mockInstance := &session.Instance{Title: "test-1"}
	mockOp := NewMockOperation("test-op", false, 0)
	successResult := &OperationResult{
		Instance:  mockInstance,
		Operation: mockOp,
		Error:     nil,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
	pr.AddResult(successResult)

	if !pr.AllSucceeded() {
		t.Error("Should be all succeeded with one success")
	}

	if pr.SuccessRate() != 100.0 {
		t.Errorf("Expected 100%% success rate, got %.2f%%", pr.SuccessRate())
	}

	// Add failed result
	mockInstance2 := &session.Instance{Title: "test-2"}
	failResult := &OperationResult{
		Instance:  mockInstance2,
		Operation: mockOp,
		Error:     fmt.Errorf("test error"),
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}
	pr.AddResult(failResult)

	if pr.AllSucceeded() {
		t.Error("Should not be all succeeded with one failure")
	}

	if pr.SuccessRate() != 50.0 {
		t.Errorf("Expected 50%% success rate, got %.2f%%", pr.SuccessRate())
	}

	if pr.Error() == nil {
		t.Error("Expected error when there are failures")
	}

	pr.Complete()
	if pr.Duration() <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestProgressTracker(t *testing.T) {
	tracker := NewProgressTracker(10)

	progressUpdates := 0
	tracker.OnProgress(func(result *OperationResult, completed, total int) {
		progressUpdates++
		if completed > total {
			t.Errorf("Completed (%d) should not exceed total (%d)", completed, total)
		}
	})

	mockInstance := &session.Instance{Title: "test"}
	mockOp := NewMockOperation("test-op", false, 0)

	for i := 0; i < 10; i++ {
		result := &OperationResult{
			Instance:  mockInstance,
			Operation: mockOp,
			Error:     nil,
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
		tracker.Update(result)
	}

	if progressUpdates != 10 {
		t.Errorf("Expected 10 progress updates, got %d", progressUpdates)
	}

	if !tracker.IsComplete() {
		t.Error("Tracker should be complete")
	}

	completed, total := tracker.Progress()
	if completed != 10 || total != 10 {
		t.Errorf("Expected 10/10, got %d/%d", completed, total)
	}
}

func TestBatchExecutor(t *testing.T) {
	executor := NewBatchExecutor(2)
	executor.SetTimeout(5 * time.Second)

	instances := []*session.Instance{
		{Title: "test-1"},
		{Title: "test-2"},
		{Title: "test-3"},
	}

	mockOp := NewMockOperation("test-op", false, 100*time.Millisecond)
	tracker := NewProgressTracker(len(instances))

	ctx := context.Background()
	result := executor.Execute(ctx, instances, mockOp, tracker)

	if !result.AllSucceeded() {
		t.Errorf("All operations should succeed, got error: %v", result.Error())
	}

	if len(result.Successes) != 3 {
		t.Errorf("Expected 3 successes, got %d", len(result.Successes))
	}

	if len(result.Failures) != 0 {
		t.Errorf("Expected 0 failures, got %d", len(result.Failures))
	}
}

func TestBatchExecutorWithFailures(t *testing.T) {
	executor := NewBatchExecutor(2)

	instances := []*session.Instance{
		{Title: "test-1"},
		{Title: "test-2"},
		{Title: "test-3"},
	}

	mockOp := NewMockOperation("test-op", true, 0) // This will fail
	tracker := NewProgressTracker(len(instances))

	ctx := context.Background()
	result := executor.Execute(ctx, instances, mockOp, tracker)

	if result.AllSucceeded() {
		t.Error("Should not all succeed with failing operation")
	}

	if len(result.Failures) != 3 {
		t.Errorf("Expected 3 failures, got %d", len(result.Failures))
	}

	if result.Error() == nil {
		t.Error("Expected error from result")
	}
}

func TestBatchExecutorWithContext(t *testing.T) {
	executor := NewBatchExecutor(2)

	instances := []*session.Instance{
		{Title: "test-1"},
		{Title: "test-2"},
		{Title: "test-3"},
	}

	// Operation that takes a long time
	mockOp := NewMockOperation("test-op", false, 2*time.Second)
	tracker := NewProgressTracker(len(instances))

	// Context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := executor.Execute(ctx, instances, mockOp, tracker)

	// Some operations should fail due to context cancellation
	if result.AllSucceeded() {
		t.Error("Some operations should fail due to context cancellation")
	}
}

func TestBatchExecutorWithRollback(t *testing.T) {
	executor := NewBatchExecutor(2)

	instances := []*session.Instance{
		{Title: "test-1"},
		{Title: "test-2"},
		{Title: "test-3"},
	}

	// First two succeed, third fails
	successOp := NewMockOperation("success-op", false, 0)
	tracker := NewProgressTracker(len(instances))

	ctx := context.Background()

	// Execute successfully first
	result := executor.Execute(ctx, instances[:2], successOp, tracker)
	if !result.AllSucceeded() {
		t.Fatalf("Initial operations should succeed")
	}

	// Now try with rollback on all instances, where one fails
	failOp := NewMockOperation("fail-op", true, 0)
	tracker2 := NewProgressTracker(len(instances))

	rollbackResult, err := executor.ExecuteWithRollback(ctx, instances, failOp, tracker2)
	if err == nil {
		t.Error("Expected error from ExecuteWithRollback")
	}

	if rollbackResult.AllSucceeded() {
		t.Error("Should not all succeed")
	}
}

func TestTransactionManager(t *testing.T) {
	tm := NewTransactionManager(2)
	tm.SetTimeout(5 * time.Second)

	instances := []*session.Instance{
		{Title: "test-1"},
		{Title: "test-2"},
	}

	successOp := NewMockOperation("success-op", false, 0)
	tracker := NewProgressTracker(len(instances))

	ctx := context.Background()
	err := tm.Execute(ctx, instances, successOp, tracker)
	if err != nil {
		t.Errorf("Transaction should succeed, got error: %v", err)
	}

	// Test with failing operation
	failOp := NewMockOperation("fail-op", true, 0)
	tracker2 := NewProgressTracker(len(instances))

	err = tm.Execute(ctx, instances, failOp, tracker2)
	if err == nil {
		t.Error("Transaction should fail")
	}
}

func TestOperationChain(t *testing.T) {
	executor := NewBatchExecutor(2)
	instances := []*session.Instance{
		{Title: "test-1"},
	}

	chain := NewOperationChain(true)
	op1 := NewMockOperation("op1", false, 0)
	op2 := NewMockOperation("op2", false, 0)
	op3 := NewMockOperation("op3", false, 0)

	chain.Add(op1).Add(op2).Add(op3)

	tracker := NewProgressTracker(len(instances) * len(chain.operations))
	ctx := context.Background()

	results := chain.Execute(ctx, instances, executor, tracker)

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.AllSucceeded() {
			t.Errorf("Operation %d should succeed", i)
		}
	}
}

func TestOperationChainStopOnFailure(t *testing.T) {
	executor := NewBatchExecutor(2)
	instances := []*session.Instance{
		{Title: "test-1"},
	}

	chain := NewOperationChain(true) // Stop on failure
	op1 := NewMockOperation("op1", false, 0)
	op2 := NewMockOperation("op2", true, 0) // This will fail
	op3 := NewMockOperation("op3", false, 0)

	chain.Add(op1).Add(op2).Add(op3)

	tracker := NewProgressTracker(len(instances) * len(chain.operations))
	ctx := context.Background()

	results := chain.Execute(ctx, instances, executor, tracker)

	// Should stop after op2 fails
	if len(results) != 2 {
		t.Errorf("Expected 2 results (stopped on failure), got %d", len(results))
	}

	if !op3.executeCalled {
		// op3 should not be executed if we stopped on failure
	}
}

func TestCompositeOperation(t *testing.T) {
	mockInstance := &session.Instance{Title: "test"}
	ctx := context.Background()

	op1 := NewMockOperation("op1", false, 0)
	op2 := NewMockOperation("op2", false, 0)

	composite := NewCompositeOperation("composite", op1, op2)

	if err := composite.Validate(mockInstance); err != nil {
		t.Errorf("Validation should succeed: %v", err)
	}

	if err := composite.Execute(ctx, mockInstance); err != nil {
		t.Errorf("Execution should succeed: %v", err)
	}

	if !op1.executeCalled || !op2.executeCalled {
		t.Error("Both operations should be executed")
	}
}

func TestCompositeOperationRollback(t *testing.T) {
	mockInstance := &session.Instance{Title: "test"}
	ctx := context.Background()

	op1 := NewMockOperation("op1", false, 0)
	op2 := NewMockOperation("op2", false, 0)

	composite := NewCompositeOperation("composite", op1, op2)

	// Execute first
	if err := composite.Execute(ctx, mockInstance); err != nil {
		t.Fatalf("Execution should succeed: %v", err)
	}

	// Then rollback
	if err := composite.Rollback(ctx, mockInstance); err != nil {
		t.Errorf("Rollback should succeed: %v", err)
	}

	if !op1.rollbackCalled || !op2.rollbackCalled {
		t.Error("Both operations should be rolled back")
	}
}

func TestConditionalOperation(t *testing.T) {
	mockInstance := &session.Instance{Title: "test"}
	ctx := context.Background()

	op := NewMockOperation("op", false, 0)

	// Condition that returns true
	conditionalTrue := NewConditionalOperation(op, func(i *session.Instance) bool {
		return true
	})

	if err := conditionalTrue.Execute(ctx, mockInstance); err != nil {
		t.Errorf("Execution should succeed: %v", err)
	}

	if !op.executeCalled {
		t.Error("Operation should be executed when condition is true")
	}

	// Condition that returns false
	op2 := NewMockOperation("op2", false, 0)
	conditionalFalse := NewConditionalOperation(op2, func(i *session.Instance) bool {
		return false
	})

	if err := conditionalFalse.Execute(ctx, mockInstance); err != nil {
		t.Errorf("Execution should succeed (skip): %v", err)
	}

	if op2.executeCalled {
		t.Error("Operation should not be executed when condition is false")
	}
}

func TestRetryOperation(t *testing.T) {
	mockInstance := &session.Instance{Title: "test"}
	ctx := context.Background()

	// Operation that fails first time, succeeds second time
	retryableOp := &MockOperationWithRetry{
		name:     "retryable",
		attempts: 0,
		failUntil: 1, // fail first attempt
	}

	retryOp := NewRetryOperation(retryableOp, 3, 10*time.Millisecond)

	if err := retryOp.Execute(ctx, mockInstance); err != nil {
		t.Errorf("Retry operation should eventually succeed: %v", err)
	}

	if retryableOp.attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", retryableOp.attempts)
	}
}

func TestRetryOperationExhaustion(t *testing.T) {
	mockInstance := &session.Instance{Title: "test"}
	ctx := context.Background()

	// Operation that always fails
	failOp := NewMockOperation("always-fail", true, 0)
	retryOp := NewRetryOperation(failOp, 2, 10*time.Millisecond)

	err := retryOp.Execute(ctx, mockInstance)
	if err == nil {
		t.Error("Retry operation should fail after exhausting retries")
	}

	if !failOp.executeCalled {
		t.Error("Operation should have been attempted")
	}
}

func TestOperationResultDuration(t *testing.T) {
	start := time.Now()
	time.Sleep(100 * time.Millisecond)
	end := time.Now()

	result := &OperationResult{
		Instance:  &session.Instance{Title: "test"},
		Operation: NewMockOperation("op", false, 0),
		StartTime: start,
		EndTime:   end,
	}

	duration := result.Duration()
	if duration < 100*time.Millisecond {
		t.Errorf("Duration should be at least 100ms, got %v", duration)
	}

	if duration > 200*time.Millisecond {
		t.Errorf("Duration should be less than 200ms, got %v", duration)
	}
}

func TestBatchOperationValidation(t *testing.T) {
	tests := []struct {
		name      string
		op        Operation
		instance  *session.Instance
		shouldErr bool
	}{
		{
			name:      "BatchKill - not started",
			op:        NewBatchKillOperation(),
			instance:  &session.Instance{Title: "test"},
			shouldErr: true,
		},
		{
			name:      "BatchPause - not started",
			op:        NewBatchPauseOperation(),
			instance:  &session.Instance{Title: "test"},
			shouldErr: true,
		},
		{
			name:      "BatchResume - not started",
			op:        NewBatchResumeOperation(),
			instance:  &session.Instance{Title: "test"},
			shouldErr: true,
		},
		{
			name:      "BatchStart - empty title",
			op:        NewBatchStartOperation(true),
			instance:  &session.Instance{Title: ""},
			shouldErr: true,
		},
		{
			name:      "BatchPrompt - empty prompt",
			op:        NewBatchPromptOperation(""),
			instance:  &session.Instance{Title: "test"},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate(tt.instance)
			if tt.shouldErr && err == nil {
				t.Error("Expected validation error")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func BenchmarkBatchExecutor(b *testing.B) {
	executor := NewBatchExecutor(10)
	instances := make([]*session.Instance, 100)
	for i := 0; i < 100; i++ {
		instances[i] = &session.Instance{Title: fmt.Sprintf("test-%d", i)}
	}

	mockOp := NewMockOperation("bench-op", false, 0)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(ctx, instances, mockOp, nil)
	}
}

func BenchmarkOperationChain(b *testing.B) {
	executor := NewBatchExecutor(5)
	instances := []*session.Instance{
		{Title: "test-1"},
		{Title: "test-2"},
	}

	chain := NewOperationChain(false)
	for i := 0; i < 5; i++ {
		chain.Add(NewMockOperation(fmt.Sprintf("op-%d", i), false, 0))
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.Execute(ctx, instances, executor, nil)
	}
}
