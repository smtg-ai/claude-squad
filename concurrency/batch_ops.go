package concurrency

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"claude-squad/session"
)

// Operation defines the interface for a batch operation that can be executed, validated, and rolled back
type Operation interface {
	// Execute performs the operation and returns an error if it fails
	Execute(ctx context.Context, instance *session.Instance) error
	// Rollback reverses the operation (best effort)
	Rollback(ctx context.Context, instance *session.Instance) error
	// Validate checks if the operation can be performed on the instance
	Validate(instance *session.Instance) error
	// Name returns a human-readable name for the operation
	Name() string
}

// OperationResult represents the result of an operation on a single instance
type OperationResult struct {
	Instance  *session.Instance
	Operation Operation
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Rolled    bool
}

// Duration returns the duration of the operation
func (r *OperationResult) Duration() time.Duration {
	if r.EndTime.IsZero() {
		return time.Since(r.StartTime)
	}
	return r.EndTime.Sub(r.StartTime)
}

// Success returns true if the operation succeeded
func (r *OperationResult) Success() bool {
	return r.Error == nil
}

// PartialResult represents the results of a batch operation with partial failures
type PartialResult struct {
	Successes []*OperationResult
	Failures  []*OperationResult
	Total     int
	StartTime time.Time
	EndTime   time.Time
}

// NewPartialResult creates a new PartialResult
func NewPartialResult() *PartialResult {
	return &PartialResult{
		Successes: make([]*OperationResult, 0),
		Failures:  make([]*OperationResult, 0),
		StartTime: time.Now(),
	}
}

// AddResult adds an operation result to the appropriate list
func (pr *PartialResult) AddResult(result *OperationResult) {
	pr.Total++
	if result.Success() {
		pr.Successes = append(pr.Successes, result)
	} else {
		pr.Failures = append(pr.Failures, result)
	}
}

// Complete marks the batch operation as complete
func (pr *PartialResult) Complete() {
	pr.EndTime = time.Now()
}

// Duration returns the total duration of the batch operation
func (pr *PartialResult) Duration() time.Duration {
	if pr.EndTime.IsZero() {
		return time.Since(pr.StartTime)
	}
	return pr.EndTime.Sub(pr.StartTime)
}

// AllSucceeded returns true if all operations succeeded
func (pr *PartialResult) AllSucceeded() bool {
	return len(pr.Failures) == 0 && pr.Total > 0
}

// SuccessRate returns the success rate as a percentage
func (pr *PartialResult) SuccessRate() float64 {
	if pr.Total == 0 {
		return 0
	}
	return float64(len(pr.Successes)) / float64(pr.Total) * 100
}

// Error returns a combined error for all failures, or nil if all succeeded
func (pr *PartialResult) Error() error {
	if len(pr.Failures) == 0 {
		return nil
	}

	if len(pr.Failures) == 1 {
		return pr.Failures[0].Error
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "%d operations failed:", len(pr.Failures))
	for _, failure := range pr.Failures {
		fmt.Fprintf(&builder, "\n  - %s on instance '%s': %v",
			failure.Operation.Name(),
			failure.Instance.Title,
			failure.Error)
	}
	return fmt.Errorf("%s", builder.String())
}

// ProgressCallback is called when an operation completes (success or failure)
type ProgressCallback func(result *OperationResult, completed, total int)

// ProgressTracker tracks the progress of batch operations
type ProgressTracker struct {
	mu        sync.RWMutex
	callbacks []ProgressCallback
	completed int
	total     int
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(total int) *ProgressTracker {
	return &ProgressTracker{
		callbacks: make([]ProgressCallback, 0),
		total:     total,
		completed: 0,
	}
}

// OnProgress registers a callback to be invoked on progress updates
func (pt *ProgressTracker) OnProgress(callback ProgressCallback) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.callbacks = append(pt.callbacks, callback)
}

// Update increments the completed counter and invokes all callbacks
func (pt *ProgressTracker) Update(result *OperationResult) {
	pt.mu.Lock()
	pt.completed++
	completed := pt.completed
	total := pt.total
	callbacks := append([]ProgressCallback(nil), pt.callbacks...)
	pt.mu.Unlock()

	// Call callbacks outside the lock
	for _, callback := range callbacks {
		callback(result, completed, total)
	}
}

// Progress returns the current progress (completed, total)
func (pt *ProgressTracker) Progress() (int, int) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.completed, pt.total
}

// IsComplete returns true if all operations have completed
func (pt *ProgressTracker) IsComplete() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.completed >= pt.total
}

// BatchExecutor executes operations on multiple instances with configurable concurrency
type BatchExecutor struct {
	maxConcurrency int
	timeout        time.Duration
}

// NewBatchExecutor creates a new batch executor with the specified maximum concurrency
func NewBatchExecutor(maxConcurrency int) *BatchExecutor {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	return &BatchExecutor{
		maxConcurrency: maxConcurrency,
		timeout:        5 * time.Minute, // default timeout
	}
}

// SetTimeout sets the timeout for individual operations
func (be *BatchExecutor) SetTimeout(timeout time.Duration) {
	be.timeout = timeout
}

// Execute executes an operation on multiple instances with progress tracking
func (be *BatchExecutor) Execute(ctx context.Context, instances []*session.Instance, op Operation, tracker *ProgressTracker) *PartialResult {
	result := NewPartialResult()

	if len(instances) == 0 {
		result.Complete()
		return result
	}

	// Create a semaphore to limit concurrency
	semaphore := make(chan struct{}, be.maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, instance := range instances {
		// Check context cancellation before starting new work
		select {
		case <-ctx.Done():
			// Context cancelled, stop processing new instances
			break
		default:
		}

		wg.Add(1)
		go func(inst *session.Instance) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				opResult := &OperationResult{
					Instance:  inst,
					Operation: op,
					Error:     ctx.Err(),
					StartTime: time.Now(),
					EndTime:   time.Now(),
				}
				mu.Lock()
				result.AddResult(opResult)
				mu.Unlock()
				if tracker != nil {
					tracker.Update(opResult)
				}
				return
			}

			// Execute operation with timeout
			opCtx, cancel := context.WithTimeout(ctx, be.timeout)
			defer cancel()

			opResult := &OperationResult{
				Instance:  inst,
				Operation: op,
				StartTime: time.Now(),
			}

			// Validate before executing
			if err := op.Validate(inst); err != nil {
				opResult.Error = fmt.Errorf("validation failed: %w", err)
				opResult.EndTime = time.Now()
			} else {
				// Execute the operation
				opResult.Error = op.Execute(opCtx, inst)
				opResult.EndTime = time.Now()
			}

			mu.Lock()
			result.AddResult(opResult)
			mu.Unlock()

			if tracker != nil {
				tracker.Update(opResult)
			}
		}(instance)
	}

	wg.Wait()
	result.Complete()
	return result
}

// ExecuteWithRollback executes an operation with transaction-like semantics
// If any operation fails, all successful operations are rolled back
func (be *BatchExecutor) ExecuteWithRollback(ctx context.Context, instances []*session.Instance, op Operation, tracker *ProgressTracker) (*PartialResult, error) {
	result := be.Execute(ctx, instances, op, tracker)

	if !result.AllSucceeded() {
		// Rollback all successful operations
		rollbackResult := be.rollback(ctx, result.Successes, tracker)

		// Mark rolled back operations
		for _, success := range result.Successes {
			success.Rolled = true
		}

		if len(rollbackResult.Failures) > 0 {
			return result, fmt.Errorf("operation failed and rollback partially failed: original error: %w, rollback failures: %d",
				result.Error(), len(rollbackResult.Failures))
		}

		return result, fmt.Errorf("operation failed and was rolled back: %w", result.Error())
	}

	return result, nil
}

// rollback attempts to rollback successful operations
func (be *BatchExecutor) rollback(ctx context.Context, successes []*OperationResult, tracker *ProgressTracker) *PartialResult {
	result := NewPartialResult()

	if len(successes) == 0 {
		result.Complete()
		return result
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, be.maxConcurrency)

	for _, success := range successes {
		wg.Add(1)
		go func(opResult *OperationResult) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				rollbackResult := &OperationResult{
					Instance:  opResult.Instance,
					Operation: opResult.Operation,
					Error:     ctx.Err(),
					StartTime: time.Now(),
					EndTime:   time.Now(),
				}
				mu.Lock()
				result.AddResult(rollbackResult)
				mu.Unlock()
				if tracker != nil {
					tracker.Update(rollbackResult)
				}
				return
			}

			rollbackCtx, cancel := context.WithTimeout(ctx, be.timeout)
			defer cancel()

			rollbackResult := &OperationResult{
				Instance:  opResult.Instance,
				Operation: opResult.Operation,
				StartTime: time.Now(),
			}

			rollbackResult.Error = opResult.Operation.Rollback(rollbackCtx, opResult.Instance)
			rollbackResult.EndTime = time.Now()

			mu.Lock()
			result.AddResult(rollbackResult)
			mu.Unlock()

			if tracker != nil {
				tracker.Update(rollbackResult)
			}
		}(success)
	}

	wg.Wait()
	result.Complete()
	return result
}

// TransactionManager manages transactional batch operations
type TransactionManager struct {
	executor *BatchExecutor
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(maxConcurrency int) *TransactionManager {
	return &TransactionManager{
		executor: NewBatchExecutor(maxConcurrency),
	}
}

// Execute executes an operation with transaction semantics
func (tm *TransactionManager) Execute(ctx context.Context, instances []*session.Instance, op Operation, tracker *ProgressTracker) error {
	_, err := tm.executor.ExecuteWithRollback(ctx, instances, op, tracker)
	return err
}

// SetTimeout sets the timeout for operations
func (tm *TransactionManager) SetTimeout(timeout time.Duration) {
	tm.executor.SetTimeout(timeout)
}

// OperationChain represents a sequence of operations to be executed in order
type OperationChain struct {
	operations    []Operation
	stopOnFailure bool
}

// NewOperationChain creates a new operation chain
func NewOperationChain(stopOnFailure bool) *OperationChain {
	return &OperationChain{
		operations:    make([]Operation, 0),
		stopOnFailure: stopOnFailure,
	}
}

// Add adds an operation to the chain
func (oc *OperationChain) Add(op Operation) *OperationChain {
	oc.operations = append(oc.operations, op)
	return oc
}

// Execute executes all operations in the chain on the given instances
func (oc *OperationChain) Execute(ctx context.Context, instances []*session.Instance, executor *BatchExecutor, tracker *ProgressTracker) []*PartialResult {
	results := make([]*PartialResult, 0, len(oc.operations))

	for _, op := range oc.operations {
		result := executor.Execute(ctx, instances, op, tracker)
		results = append(results, result)

		if oc.stopOnFailure && !result.AllSucceeded() {
			break
		}
	}

	return results
}

// BatchKillOperation kills multiple instances
type BatchKillOperation struct{}

func NewBatchKillOperation() *BatchKillOperation {
	return &BatchKillOperation{}
}

func (op *BatchKillOperation) Name() string {
	return "Kill"
}

func (op *BatchKillOperation) Validate(instance *session.Instance) error {
	if !instance.Started() {
		return fmt.Errorf("instance '%s' has not been started", instance.Title)
	}
	return nil
}

func (op *BatchKillOperation) Execute(ctx context.Context, instance *session.Instance) error {
	// Check context before executing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Kill()
}

func (op *BatchKillOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Kill operation cannot be rolled back - it's a terminal operation
	// We could potentially restart the instance, but that would create a new state
	return fmt.Errorf("kill operation cannot be rolled back")
}

// BatchPauseOperation pauses multiple instances
type BatchPauseOperation struct{}

func NewBatchPauseOperation() *BatchPauseOperation {
	return &BatchPauseOperation{}
}

func (op *BatchPauseOperation) Name() string {
	return "Pause"
}

func (op *BatchPauseOperation) Validate(instance *session.Instance) error {
	if !instance.Started() {
		return fmt.Errorf("instance '%s' has not been started", instance.Title)
	}
	if instance.Paused() {
		return fmt.Errorf("instance '%s' is already paused", instance.Title)
	}
	return nil
}

func (op *BatchPauseOperation) Execute(ctx context.Context, instance *session.Instance) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Pause()
}

func (op *BatchPauseOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Rollback pause by resuming
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Resume()
}

// BatchResumeOperation resumes multiple instances
type BatchResumeOperation struct{}

func NewBatchResumeOperation() *BatchResumeOperation {
	return &BatchResumeOperation{}
}

func (op *BatchResumeOperation) Name() string {
	return "Resume"
}

func (op *BatchResumeOperation) Validate(instance *session.Instance) error {
	if !instance.Started() {
		return fmt.Errorf("instance '%s' has not been started", instance.Title)
	}
	if !instance.Paused() {
		return fmt.Errorf("instance '%s' is not paused", instance.Title)
	}
	return nil
}

func (op *BatchResumeOperation) Execute(ctx context.Context, instance *session.Instance) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Resume()
}

func (op *BatchResumeOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Rollback resume by pausing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Pause()
}

// BatchStartOperation starts multiple instances
type BatchStartOperation struct {
	firstTimeSetup bool
}

func NewBatchStartOperation(firstTimeSetup bool) *BatchStartOperation {
	return &BatchStartOperation{
		firstTimeSetup: firstTimeSetup,
	}
}

func (op *BatchStartOperation) Name() string {
	return "Start"
}

func (op *BatchStartOperation) Validate(instance *session.Instance) error {
	if instance.Started() && !instance.Paused() {
		return fmt.Errorf("instance '%s' is already started", instance.Title)
	}
	if instance.Title == "" {
		return fmt.Errorf("instance title cannot be empty")
	}
	return nil
}

func (op *BatchStartOperation) Execute(ctx context.Context, instance *session.Instance) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Start(op.firstTimeSetup)
}

func (op *BatchStartOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Rollback start by killing
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.Kill()
}

// BatchPromptOperation sends a prompt to multiple instances
type BatchPromptOperation struct {
	prompt string
}

func NewBatchPromptOperation(prompt string) *BatchPromptOperation {
	return &BatchPromptOperation{
		prompt: prompt,
	}
}

func (op *BatchPromptOperation) Name() string {
	return "SendPrompt"
}

func (op *BatchPromptOperation) Validate(instance *session.Instance) error {
	if !instance.Started() {
		return fmt.Errorf("instance '%s' has not been started", instance.Title)
	}
	if instance.Paused() {
		return fmt.Errorf("instance '%s' is paused", instance.Title)
	}
	if op.prompt == "" {
		return fmt.Errorf("prompt cannot be empty")
	}
	return nil
}

func (op *BatchPromptOperation) Execute(ctx context.Context, instance *session.Instance) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return instance.SendPrompt(op.prompt)
}

func (op *BatchPromptOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Prompts cannot be rolled back
	return fmt.Errorf("prompt operation cannot be rolled back")
}

// CompositeOperation combines multiple operations into a single operation
type CompositeOperation struct {
	name       string
	operations []Operation
}

func NewCompositeOperation(name string, operations ...Operation) *CompositeOperation {
	return &CompositeOperation{
		name:       name,
		operations: operations,
	}
}

func (op *CompositeOperation) Name() string {
	return op.name
}

func (op *CompositeOperation) Validate(instance *session.Instance) error {
	for _, operation := range op.operations {
		if err := operation.Validate(instance); err != nil {
			return fmt.Errorf("validation failed for %s: %w", operation.Name(), err)
		}
	}
	return nil
}

func (op *CompositeOperation) Execute(ctx context.Context, instance *session.Instance) error {
	for i, operation := range op.operations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := operation.Execute(ctx, instance); err != nil {
			return fmt.Errorf("operation %d (%s) failed: %w", i, operation.Name(), err)
		}
	}
	return nil
}

func (op *CompositeOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Rollback operations in reverse order
	var errors []error
	for i := len(op.operations) - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			errors = append(errors, ctx.Err())
			break
		default:
		}

		if err := op.operations[i].Rollback(ctx, instance); err != nil {
			errors = append(errors, fmt.Errorf("rollback of operation %d (%s) failed: %w",
				i, op.operations[i].Name(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback had %d errors: %v", len(errors), errors)
	}
	return nil
}

// ConditionalOperation executes an operation only if a condition is met
type ConditionalOperation struct {
	operation Operation
	condition func(*session.Instance) bool
}

func NewConditionalOperation(operation Operation, condition func(*session.Instance) bool) *ConditionalOperation {
	return &ConditionalOperation{
		operation: operation,
		condition: condition,
	}
}

func (op *ConditionalOperation) Name() string {
	return fmt.Sprintf("Conditional(%s)", op.operation.Name())
}

func (op *ConditionalOperation) Validate(instance *session.Instance) error {
	if !op.condition(instance) {
		return nil // Skip validation if condition is not met
	}
	return op.operation.Validate(instance)
}

func (op *ConditionalOperation) Execute(ctx context.Context, instance *session.Instance) error {
	if !op.condition(instance) {
		return nil // Skip execution if condition is not met
	}
	return op.operation.Execute(ctx, instance)
}

func (op *ConditionalOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	if !op.condition(instance) {
		return nil // Skip rollback if condition is not met
	}
	return op.operation.Rollback(ctx, instance)
}

// RetryOperation wraps an operation with retry logic
type RetryOperation struct {
	operation  Operation
	maxRetries int
	retryDelay time.Duration
}

func NewRetryOperation(operation Operation, maxRetries int, retryDelay time.Duration) *RetryOperation {
	return &RetryOperation{
		operation:  operation,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

func (op *RetryOperation) Name() string {
	return fmt.Sprintf("Retry(%s)", op.operation.Name())
}

func (op *RetryOperation) Validate(instance *session.Instance) error {
	return op.operation.Validate(instance)
}

func (op *RetryOperation) Execute(ctx context.Context, instance *session.Instance) error {
	var lastErr error
	for i := 0; i <= op.maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := op.operation.Execute(ctx, instance); err == nil {
			return nil
		} else {
			lastErr = err
			if i < op.maxRetries {
				// Wait before retry
				select {
				case <-time.After(op.retryDelay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
	return fmt.Errorf("operation failed after %d retries: %w", op.maxRetries, lastErr)
}

func (op *RetryOperation) Rollback(ctx context.Context, instance *session.Instance) error {
	// Don't retry rollback operations - they should be fast
	return op.operation.Rollback(ctx, instance)
}
