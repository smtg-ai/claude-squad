package session

import (
	"context"
	"runtime"
	"sync"
)

// Task represents a unit of work to be executed by a worker pool
type Task func() error

// WorkerPool manages a pool of workers for executing tasks concurrently
type WorkerPool struct {
	workers    int
	taskQueue  chan Task
	resultCh   chan error
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	started    bool
	mu         sync.Mutex
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerPool{
		workers:   workers,
		taskQueue: make(chan Task, workers*2), // Buffer to prevent blocking
		resultCh:  make(chan error, workers*2),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start initializes and starts the worker pool
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	
	if wp.started {
		return
	}
	
	wp.started = true
	
	// Start workers
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker runs tasks from the task queue
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	
	for {
		select {
		case <-wp.ctx.Done():
			return
		case task, ok := <-wp.taskQueue:
			if !ok {
				return
			}
			
			// Execute task and send result
			err := task()
			select {
			case wp.resultCh <- err:
			case <-wp.ctx.Done():
				return
			}
		}
	}
}

// Submit adds a task to the worker pool and returns immediately
func (wp *WorkerPool) Submit(task Task) {
	wp.mu.Lock()
	if !wp.started {
		wp.Start()
	}
	wp.mu.Unlock()
	
	select {
	case wp.taskQueue <- task:
	case <-wp.ctx.Done():
	}
}

// Wait waits for all submitted tasks to complete and returns any errors
func (wp *WorkerPool) Wait() []error {
	close(wp.taskQueue)
	wp.wg.Wait()
	
	var errors []error
	close(wp.resultCh)
	
	for err := range wp.resultCh {
		if err != nil {
			errors = append(errors, err)
		}
	}
	
	return errors
}

// Shutdown gracefully shuts down the worker pool
func (wp *WorkerPool) Shutdown() {
	wp.cancel()
	wp.wg.Wait()
}

// Global worker pools for different operation types
var (
	gitWorkerPool  *WorkerPool
	tmuxWorkerPool *WorkerPool
	initOnce       sync.Once
)

// initWorkerPools initializes the global worker pools
func initWorkerPools() {
	initOnce.Do(func() {
		gitWorkerPool = NewWorkerPool(2)  // 2 workers for git operations
		tmuxWorkerPool = NewWorkerPool(4) // 4 workers for tmux operations
	})
}

// GetGitWorkerPool returns the global git worker pool
func GetGitWorkerPool() *WorkerPool {
	initWorkerPools()
	return gitWorkerPool
}

// GetTmuxWorkerPool returns the global tmux worker pool
func GetTmuxWorkerPool() *WorkerPool {
	initWorkerPools()
	return tmuxWorkerPool
}

// SubmitGitTask submits a git-related task to the git worker pool
func SubmitGitTask(task Task) {
	GetGitWorkerPool().Submit(task)
}

// SubmitTmuxTask submits a tmux-related task to the tmux worker pool
func SubmitTmuxTask(task Task) {
	GetTmuxWorkerPool().Submit(task)
}