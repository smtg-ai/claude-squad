package concurrency

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Job represents a unit of work to be executed by the worker pool.
// Implementations must be safe for concurrent execution.
type Job interface {
	// Execute performs the job's work and returns a result or error.
	Execute(ctx context.Context) (interface{}, error)
	// Priority returns the job's priority (higher values = higher priority).
	Priority() int
	// ID returns a unique identifier for the job.
	ID() string
}

// JobResult contains the outcome of a job execution.
type JobResult struct {
	JobID     string
	Result    interface{}
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// WorkerStatus represents the health state of a worker.
type WorkerStatus int

const (
	// WorkerIdle indicates the worker is waiting for jobs.
	WorkerIdle WorkerStatus = iota
	// WorkerBusy indicates the worker is processing a job.
	WorkerBusy
	// WorkerFailed indicates the worker encountered an error.
	WorkerFailed
	// WorkerStopped indicates the worker has been shut down.
	WorkerStopped
)

func (s WorkerStatus) String() string {
	switch s {
	case WorkerIdle:
		return "idle"
	case WorkerBusy:
		return "busy"
	case WorkerFailed:
		return "failed"
	case WorkerStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Worker represents a single worker in the pool.
type Worker struct {
	id            int
	status        atomic.Int32 // Stores WorkerStatus
	lastHeartbeat atomic.Int64 // Unix timestamp
	jobsProcessed atomic.Uint64
	lastError     atomic.Value // Stores error
}

// newWorker creates a new worker with the given ID.
func newWorker(id int) *Worker {
	w := &Worker{
		id: id,
	}
	w.status.Store(int32(WorkerIdle))
	w.updateHeartbeat()
	return w
}

// ID returns the worker's unique identifier.
func (w *Worker) ID() int {
	return w.id
}

// Status returns the current status of the worker.
func (w *Worker) Status() WorkerStatus {
	return WorkerStatus(w.status.Load())
}

// setStatus updates the worker's status.
func (w *Worker) setStatus(status WorkerStatus) {
	w.status.Store(int32(status))
}

// LastHeartbeat returns the timestamp of the worker's last heartbeat.
func (w *Worker) LastHeartbeat() time.Time {
	return time.Unix(w.lastHeartbeat.Load(), 0)
}

// updateHeartbeat updates the worker's heartbeat timestamp.
func (w *Worker) updateHeartbeat() {
	w.lastHeartbeat.Store(time.Now().Unix())
}

// JobsProcessed returns the total number of jobs processed by this worker.
func (w *Worker) JobsProcessed() uint64 {
	return w.jobsProcessed.Load()
}

// incrementJobsProcessed increments the jobs processed counter.
func (w *Worker) incrementJobsProcessed() {
	w.jobsProcessed.Add(1)
}

// LastError returns the last error encountered by this worker.
func (w *Worker) LastError() error {
	if err := w.lastError.Load(); err != nil {
		if e, ok := err.(error); ok {
			return e
		}
		return fmt.Errorf("unexpected error type: %T", err)
	}
	return nil
}

// setLastError stores the last error encountered.
func (w *Worker) setLastError(err error) {
	w.lastError.Store(err)
}

// Metrics tracks statistics for the worker pool.
type Metrics struct {
	JobsSubmitted atomic.Uint64
	JobsCompleted atomic.Uint64
	JobsFailed    atomic.Uint64
	TotalLatency  atomic.Int64 // Nanoseconds
	MinLatency    atomic.Int64 // Nanoseconds
	MaxLatency    atomic.Int64 // Nanoseconds
	ActiveWorkers atomic.Int32
	IdleWorkers   atomic.Int32
}

// AverageLatency returns the average job execution time.
func (m *Metrics) AverageLatency() time.Duration {
	completed := m.JobsCompleted.Load()
	if completed == 0 {
		return 0
	}
	avgNanos := m.TotalLatency.Load() / int64(completed)
	return time.Duration(avgNanos)
}

// recordLatency updates latency metrics.
func (m *Metrics) recordLatency(duration time.Duration) {
	nanos := duration.Nanoseconds()
	m.TotalLatency.Add(nanos)

	// Update min latency
	for {
		current := m.MinLatency.Load()
		if current == 0 || nanos < current {
			if m.MinLatency.CompareAndSwap(current, nanos) {
				break
			}
		} else {
			break
		}
	}

	// Update max latency
	for {
		current := m.MaxLatency.Load()
		if nanos > current {
			if m.MaxLatency.CompareAndSwap(current, nanos) {
				break
			}
		} else {
			break
		}
	}
}

// String returns a formatted string representation of the metrics.
func (m *Metrics) String() string {
	return fmt.Sprintf(
		"Jobs: %d submitted, %d completed, %d failed | "+
			"Latency: avg=%v, min=%v, max=%v | "+
			"Workers: %d active, %d idle",
		m.JobsSubmitted.Load(),
		m.JobsCompleted.Load(),
		m.JobsFailed.Load(),
		m.AverageLatency(),
		time.Duration(m.MinLatency.Load()),
		time.Duration(m.MaxLatency.Load()),
		m.ActiveWorkers.Load(),
		m.IdleWorkers.Load(),
	)
}

// priorityQueue implements a priority queue using container/heap.
type priorityQueue struct {
	items []*priorityQueueItem
	mu    sync.Mutex
}

type priorityQueueItem struct {
	job      Job
	priority int
	index    int
}

func (pq *priorityQueue) Len() int {
	return len(pq.items)
}

func (pq *priorityQueue) Less(i, j int) bool {
	// Higher priority values come first
	return pq.items[i].priority > pq.items[j].priority
}

func (pq *priorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item, ok := x.(*priorityQueueItem)
	if !ok {
		// This should never happen if the heap interface is used correctly
		panic(fmt.Sprintf("priorityQueue.Push: unexpected type %T, want *priorityQueueItem", x))
	}
	item.index = n
	pq.items = append(pq.items, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	pq.items = old[0 : n-1]
	return item
}

// WorkerPoolConfig contains configuration options for the worker pool.
type WorkerPoolConfig struct {
	// MaxWorkers is the maximum number of concurrent workers (default: 10).
	MaxWorkers int
	// QueueSize is the maximum size of the job queue (default: 1000).
	QueueSize int
	// WorkerTimeout is the maximum time a job can run before being cancelled (default: 5 minutes).
	WorkerTimeout time.Duration
	// HealthCheckInterval is how often to check worker health (default: 30 seconds).
	HealthCheckInterval time.Duration
}

// DefaultConfig returns a WorkerPoolConfig with default values.
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		MaxWorkers:          10,
		QueueSize:           1000,
		WorkerTimeout:       5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
	}
}

// WorkerPool manages a pool of workers for parallel job execution.
type WorkerPool struct {
	config     WorkerPoolConfig
	workers    []*Worker
	jobQueue   *priorityQueue
	results    chan JobResult
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	metrics    *Metrics
	started    atomic.Bool
	submitChan chan Job // Channel for job submissions
	workChan   chan Job // Channel for dispatching to workers
	mu         sync.RWMutex
}

// NewWorkerPool creates a new worker pool with the given configuration.
func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	// Validate and set defaults
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 10
	}
	if config.MaxWorkers > 1000 {
		config.MaxWorkers = 1000 // Cap at reasonable limit
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}
	if config.QueueSize > 100000 {
		config.QueueSize = 100000 // Cap at reasonable limit
	}
	if config.WorkerTimeout <= 0 {
		config.WorkerTimeout = 5 * time.Minute
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = 30 * time.Second
	}

	wp := &WorkerPool{
		config:     config,
		workers:    make([]*Worker, config.MaxWorkers),
		jobQueue:   &priorityQueue{items: make([]*priorityQueueItem, 0)},
		results:    make(chan JobResult, config.QueueSize),
		ctx:        ctx,
		cancel:     cancel,
		metrics:    &Metrics{},
		submitChan: make(chan Job, config.QueueSize),
		workChan:   make(chan Job, config.MaxWorkers),
	}

	// Initialize workers
	for i := 0; i < config.MaxWorkers; i++ {
		wp.workers[i] = newWorker(i)
	}

	return wp
}

// Start begins processing jobs with the worker pool.
func (wp *WorkerPool) Start() error {
	if !wp.started.CompareAndSwap(false, true) {
		return fmt.Errorf("worker pool already started")
	}

	// Start job dispatcher
	wp.wg.Add(1)
	go wp.dispatcher()

	// Start workers
	for _, worker := range wp.workers {
		wp.wg.Add(1)
		go wp.workerLoop(worker)
	}

	// Start health monitor
	wp.wg.Add(1)
	go wp.healthMonitor()

	return nil
}

// Submit adds a job to the worker pool for execution.
// Returns an error if the pool is shut down or the queue is full.
func (wp *WorkerPool) Submit(ctx context.Context, job Job) error {
	if !wp.started.Load() {
		return fmt.Errorf("worker pool not started")
	}

	select {
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	wp.metrics.JobsSubmitted.Add(1)

	select {
	case wp.submitChan <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Results returns a channel that receives job results.
// The channel will be closed when the worker pool shuts down.
func (wp *WorkerPool) Results() <-chan JobResult {
	return wp.results
}

// Shutdown gracefully stops the worker pool, waiting for all jobs to complete.
// It returns when all workers have finished or the context is cancelled.
func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	if !wp.started.Load() {
		return fmt.Errorf("worker pool not started")
	}

	// Signal shutdown
	wp.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(wp.results)
		return nil
	case <-ctx.Done():
		close(wp.results)
		return fmt.Errorf("shutdown cancelled: %w", ctx.Err())
	}
}

// Metrics returns the current pool metrics.
func (wp *WorkerPool) Metrics() *Metrics {
	return wp.metrics
}

// Workers returns information about all workers in the pool.
func (wp *WorkerPool) Workers() []*Worker {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	workers := make([]*Worker, len(wp.workers))
	copy(workers, wp.workers)
	return workers
}

// dispatcher manages the job queue and assigns jobs to workers.
func (wp *WorkerPool) dispatcher() {
	defer wp.wg.Done()
	defer close(wp.workChan)

	for {
		select {
		case <-wp.ctx.Done():
			// Drain remaining jobs in the submission channel
			close(wp.submitChan)
			for range wp.submitChan {
				// Discard remaining jobs
			}
			return

		case job, ok := <-wp.submitChan:
			if !ok {
				return
			}
			// Add job to priority queue
			wp.jobQueue.mu.Lock()
			heap.Push(wp.jobQueue, &priorityQueueItem{
				job:      job,
				priority: job.Priority(),
			})
			wp.jobQueue.mu.Unlock()

			// Try to dispatch highest priority job immediately
			wp.dispatchNextJob()

		default:
			// If no new jobs, try to dispatch from queue
			if wp.dispatchNextJob() {
				// Successfully dispatched a job, continue
				continue
			}
			// No jobs to dispatch, brief sleep to avoid busy waiting
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// dispatchNextJob attempts to dispatch the next job from the priority queue.
// Returns true if a job was dispatched, false otherwise.
func (wp *WorkerPool) dispatchNextJob() bool {
	wp.jobQueue.mu.Lock()
	if wp.jobQueue.Len() == 0 {
		wp.jobQueue.mu.Unlock()
		return false
	}
	popped := heap.Pop(wp.jobQueue)
	item, ok := popped.(*priorityQueueItem)
	if !ok {
		wp.jobQueue.mu.Unlock()
		// Log error or handle unexpected type - this should never happen
		return false
	}
	wp.jobQueue.mu.Unlock()

	// Try to dispatch job to a worker (non-blocking)
	select {
	case wp.workChan <- item.job:
		return true
	case <-wp.ctx.Done():
		return false
	default:
		// No worker available, put job back in queue
		wp.jobQueue.mu.Lock()
		heap.Push(wp.jobQueue, item)
		wp.jobQueue.mu.Unlock()
		return false
	}
}

// workerLoop is the main loop for a worker goroutine.
func (wp *WorkerPool) workerLoop(worker *Worker) {
	defer wp.wg.Done()

	for {
		// Update metrics - worker is idle
		worker.setStatus(WorkerIdle)
		worker.updateHeartbeat()
		wp.metrics.IdleWorkers.Add(1)

		select {
		case <-wp.ctx.Done():
			worker.setStatus(WorkerStopped)
			wp.metrics.IdleWorkers.Add(-1)
			return

		case job, ok := <-wp.workChan:
			wp.metrics.IdleWorkers.Add(-1)

			if !ok {
				// Channel closed
				worker.setStatus(WorkerStopped)
				return
			}

			// Execute job
			result := wp.executeJob(worker, job)

			// Send result
			select {
			case wp.results <- result:
			case <-wp.ctx.Done():
				worker.setStatus(WorkerStopped)
				return
			}
		}
	}
}

// executeJob runs a job and returns the result.
func (wp *WorkerPool) executeJob(worker *Worker, job Job) JobResult {
	worker.setStatus(WorkerBusy)
	worker.updateHeartbeat()
	wp.metrics.ActiveWorkers.Add(1)
	defer wp.metrics.ActiveWorkers.Add(-1)

	startTime := time.Now()
	result := JobResult{
		JobID:     job.ID(),
		StartTime: startTime,
	}

	// Create job context with timeout
	jobCtx, cancel := context.WithTimeout(wp.ctx, wp.config.WorkerTimeout)
	defer cancel()

	// Execute job
	res, err := job.Execute(jobCtx)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	result.Result = res
	result.Error = err
	result.EndTime = endTime
	result.Duration = duration

	// Update metrics
	if err != nil {
		wp.metrics.JobsFailed.Add(1)
		worker.setStatus(WorkerFailed)
		worker.setLastError(err)
	} else {
		wp.metrics.JobsCompleted.Add(1)
		worker.incrementJobsProcessed()
	}

	wp.metrics.recordLatency(duration)

	return result
}

// healthMonitor periodically checks worker health.
func (wp *WorkerPool) healthMonitor() {
	defer wp.wg.Done()

	ticker := time.NewTicker(wp.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			wp.checkWorkerHealth()
		}
	}
}

// checkWorkerHealth examines all workers for potential issues.
func (wp *WorkerPool) checkWorkerHealth() {
	now := time.Now()
	timeout := wp.config.WorkerTimeout + (30 * time.Second) // Add buffer

	wp.mu.RLock()
	defer wp.mu.RUnlock()

	for _, worker := range wp.workers {
		lastHeartbeat := worker.LastHeartbeat()
		if now.Sub(lastHeartbeat) > timeout {
			if worker.Status() == WorkerBusy {
				// Worker appears to be stuck
				worker.setStatus(WorkerFailed)
				worker.setLastError(fmt.Errorf("worker timeout: no heartbeat for %v", now.Sub(lastHeartbeat)))
			}
		}
	}
}

// CollectResults collects all results from the results channel and aggregates errors.
// This is a helper function that can be used after submitting jobs.
func CollectResults(results <-chan JobResult) ([]JobResult, error) {
	var allResults []JobResult
	var errors []error

	for result := range results {
		allResults = append(allResults, result)
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("job %s failed: %w", result.JobID, result.Error))
		}
	}

	if len(errors) == 0 {
		return allResults, nil
	}

	return allResults, combineWorkerPoolErrors(errors)
}

// combineErrors combines multiple errors into a single error.
// This follows the error aggregation pattern from instance.go.
func combineWorkerPoolErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	errMsg := "multiple errors occurred:"
	for _, err := range errs {
		errMsg += "\n  - " + err.Error()
	}
	return fmt.Errorf("%s", errMsg)
}
