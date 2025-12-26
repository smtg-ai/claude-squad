package concurrency_test

import (
	"claude-squad/concurrency"
	"context"
	"fmt"
	"time"
)

// simpleJob is a basic job implementation for examples.
type simpleJob struct {
	id       string
	priority int
	work     func() (interface{}, error)
}

func (j *simpleJob) Execute(ctx context.Context) (interface{}, error) {
	return j.work()
}

func (j *simpleJob) Priority() int {
	return j.priority
}

func (j *simpleJob) ID() string {
	return j.id
}

// ExampleNewWorkerPool demonstrates creating a worker pool
// for concurrent job execution with priority queuing.
func ExampleNewWorkerPool() {
	config := concurrency.DefaultWorkerPoolConfig()
	config.MaxWorkers = 4
	config.QueueSize = 100

	pool := concurrency.NewWorkerPool(config)
	defer pool.Shutdown(context.Background())

	fmt.Printf("WorkerPool created with %d workers\n", config.MaxWorkers)
	// Output: WorkerPool created with 4 workers
}

// ExampleWorkerPool_Start demonstrates starting the worker pool
// to begin processing jobs.
func ExampleWorkerPool_Start() {
	config := concurrency.DefaultWorkerPoolConfig()
	pool := concurrency.NewWorkerPool(config)
	defer pool.Shutdown(context.Background())

	// Start the pool
	err := pool.Start()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("WorkerPool started")
	// Output: WorkerPool started
}

// ExampleWorkerPool_Submit demonstrates submitting jobs to the worker pool
// for asynchronous execution with priority handling.
func ExampleWorkerPool_Submit() {
	config := concurrency.DefaultWorkerPoolConfig()
	config.MaxWorkers = 2
	pool := concurrency.NewWorkerPool(config)
	_ = pool.Start()
	defer pool.Shutdown(context.Background())

	// Create a simple job
	job := &simpleJob{
		id:       "job-1",
		priority: 5,
		work: func() (interface{}, error) {
			return "completed", nil
		},
	}

	// Submit the job
	ctx := context.Background()
	err := pool.Submit(ctx, job)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Job submitted successfully")
	// Output: Job submitted successfully
}

// ExampleWorkerPool_Results demonstrates collecting results from completed jobs.
func ExampleWorkerPool_Results() {
	config := concurrency.DefaultWorkerPoolConfig()
	config.MaxWorkers = 2
	pool := concurrency.NewWorkerPool(config)
	_ = pool.Start()

	// Submit multiple jobs
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		job := &simpleJob{
			id:       fmt.Sprintf("job-%d", i),
			priority: i,
			work: func() (interface{}, error) {
				time.Sleep(50 * time.Millisecond)
				return "done", nil
			},
		}
		_ = pool.Submit(ctx, job)
	}

	// Collect results
	go func() {
		time.Sleep(500 * time.Millisecond)
		pool.Shutdown(context.Background())
	}()

	resultCount := 0
	for result := range pool.Results() {
		if result.Error == nil {
			resultCount++
		}
	}

	fmt.Printf("Processed %d jobs\n", resultCount)
	// Output: Processed 3 jobs
}

// ExampleWorkerPool_Metrics demonstrates retrieving worker pool metrics
// for monitoring performance.
func ExampleWorkerPool_Metrics() {
	config := concurrency.DefaultWorkerPoolConfig()
	pool := concurrency.NewWorkerPool(config)
	_ = pool.Start()
	defer pool.Shutdown(context.Background())

	// Submit some jobs
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		job := &simpleJob{
			id:       fmt.Sprintf("job-%d", i),
			priority: 1,
			work: func() (interface{}, error) {
				return "done", nil
			},
		}
		_ = pool.Submit(ctx, job)
	}

	time.Sleep(200 * time.Millisecond)

	// Get metrics
	metrics := pool.Metrics()
	fmt.Printf("Jobs Submitted: %d\n", metrics.JobsSubmitted.Load())
	fmt.Printf("Jobs Completed: %d\n", metrics.JobsCompleted.Load())
}

// ExampleWorkerPool_Workers demonstrates retrieving worker status information
// for health monitoring.
func ExampleWorkerPool_Workers() {
	config := concurrency.DefaultWorkerPoolConfig()
	config.MaxWorkers = 3
	pool := concurrency.NewWorkerPool(config)
	_ = pool.Start()
	defer pool.Shutdown(context.Background())

	// Get worker information
	workers := pool.Workers()

	fmt.Printf("Total workers: %d\n", len(workers))
	for _, worker := range workers {
		fmt.Printf("Worker %d: %s\n", worker.ID(), worker.Status())
	}
}

// ExampleCollectResults demonstrates using the helper function
// to collect and aggregate results with error handling.
func ExampleCollectResults() {
	config := concurrency.DefaultWorkerPoolConfig()
	pool := concurrency.NewWorkerPool(config)
	_ = pool.Start()

	ctx := context.Background()

	// Submit jobs with some failures
	for i := 0; i < 3; i++ {
		shouldFail := i == 1 // Second job will fail
		job := &simpleJob{
			id:       fmt.Sprintf("job-%d", i),
			priority: 1,
			work: func() (interface{}, error) {
				if shouldFail {
					return nil, fmt.Errorf("job failed")
				}
				return "success", nil
			},
		}
		_ = pool.Submit(ctx, job)
	}

	// Shutdown and collect results
	go func() {
		time.Sleep(500 * time.Millisecond)
		pool.Shutdown(context.Background())
	}()

	results, err := concurrency.CollectResults(pool.Results())

	fmt.Printf("Collected %d results\n", len(results))
	if err != nil {
		fmt.Println("Some jobs failed")
	}
}

// ExampleNewTaskQueue demonstrates creating a task queue
// for ordered task processing.
func ExampleNewTaskQueue() {
	config := concurrency.DefaultTaskQueueConfig()
	config.MaxConcurrency = 3
	config.BufferSize = 50

	queue := concurrency.NewTaskQueue(config)
	defer queue.Shutdown(context.Background())

	fmt.Printf("TaskQueue created with concurrency=%d\n", config.MaxConcurrency)
	// Output: TaskQueue created with concurrency=3
}

// ExampleNewEventStream demonstrates creating an event stream
// for real-time event broadcasting to subscribers.
func ExampleNewEventStream() {
	config := concurrency.DefaultEventStreamConfig()
	stream := concurrency.NewEventStream(config)
	defer stream.Close()

	fmt.Println("EventStream created")
	// Output: EventStream created
}

// ExampleEventStream_Subscribe demonstrates subscribing to events
// from the event stream.
func ExampleEventStream_Subscribe() {
	config := concurrency.DefaultEventStreamConfig()
	stream := concurrency.NewEventStream(config)
	defer stream.Close()

	// Subscribe to events
	subscriber := stream.Subscribe("subscriber-1")

	go func() {
		// Publish an event
		event := concurrency.Event{
			Type:      "test",
			Timestamp: time.Now(),
			Data:      "Hello, subscriber!",
		}
		_ = stream.Publish(event)
		time.Sleep(100 * time.Millisecond)
		stream.Close()
	}()

	// Receive events
	for event := range subscriber {
		fmt.Printf("Received event: %s\n", event.Type)
	}
	// Output: Received event: test
}

// ExampleEventStream_Publish demonstrates publishing events
// to all subscribers.
func ExampleEventStream_Publish() {
	config := concurrency.DefaultEventStreamConfig()
	stream := concurrency.NewEventStream(config)
	defer stream.Close()

	_ = stream.Subscribe("subscriber-1")
	_ = stream.Subscribe("subscriber-2")

	// Publish an event to all subscribers
	event := concurrency.Event{
		Type:      "notification",
		Timestamp: time.Now(),
		Data:      "System update",
	}

	err := stream.Publish(event)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Event published to all subscribers")
	// Output: Event published to all subscribers
}

// ExampleNewHealthMonitor demonstrates creating a health monitor
// for tracking service component health.
func ExampleNewHealthMonitor() {
	config := concurrency.DefaultHealthMonitorConfig()
	config.CheckInterval = 5 * time.Second

	monitor := concurrency.NewHealthMonitor(config)
	defer monitor.Stop()

	fmt.Println("HealthMonitor created")
	// Output: HealthMonitor created
}

// ExampleHealthMonitor_RegisterCheck demonstrates registering a health check
// for periodic execution.
func ExampleHealthMonitor_RegisterCheck() {
	config := concurrency.DefaultHealthMonitorConfig()
	monitor := concurrency.NewHealthMonitor(config)
	defer monitor.Stop()

	// Register a health check
	check := concurrency.HealthCheck{
		Name:     "database",
		CheckFn:  func(ctx context.Context) error { return nil },
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
	}

	monitor.RegisterCheck(check)

	fmt.Println("Health check registered")
	// Output: Health check registered
}

// ExampleHealthMonitor_GetStatus demonstrates retrieving current health status
// for all registered checks.
func ExampleHealthMonitor_GetStatus() {
	config := concurrency.DefaultHealthMonitorConfig()
	monitor := concurrency.NewHealthMonitor(config)
	defer monitor.Stop()

	// Register a check
	check := concurrency.HealthCheck{
		Name:     "api",
		CheckFn:  func(ctx context.Context) error { return nil },
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
	}
	monitor.RegisterCheck(check)

	_ = monitor.Start()
	time.Sleep(100 * time.Millisecond)

	// Get health status
	status := monitor.GetStatus()

	fmt.Printf("Health checks registered: %d\n", len(status))
}

// ExampleNewResourceManager demonstrates creating a resource manager
// for tracking and limiting resource usage.
func ExampleNewResourceManager() {
	config := concurrency.DefaultResourceManagerConfig()
	config.MaxMemoryMB = 1024
	config.MaxCPUPercent = 80.0

	rm := concurrency.NewResourceManager(config)
	defer rm.Shutdown()

	fmt.Printf("ResourceManager created with max memory=%dMB\n", config.MaxMemoryMB)
	// Output: ResourceManager created with max memory=1024MB
}

// ExampleResourceManager_AcquireResource demonstrates acquiring resources
// with quota enforcement.
func ExampleResourceManager_AcquireResource() {
	config := concurrency.DefaultResourceManagerConfig()
	rm := concurrency.NewResourceManager(config)
	defer rm.Shutdown()

	ctx := context.Background()

	// Acquire a resource
	resource, err := rm.AcquireResource(ctx, "worker-1", 256)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Resource acquired: %s\n", resource.ID)

	// Release the resource
	_ = rm.ReleaseResource(resource.ID)
}

// ExampleResourceManager_GetMetrics demonstrates retrieving resource usage metrics
// for monitoring.
func ExampleResourceManager_GetMetrics() {
	config := concurrency.DefaultResourceManagerConfig()
	rm := concurrency.NewResourceManager(config)
	defer rm.Shutdown()

	// Get resource metrics
	metrics := rm.GetMetrics()

	fmt.Printf("Active resources: %d\n", metrics.ActiveResources)
	fmt.Printf("Total acquired: %d\n", metrics.TotalAcquired)
}

// ExampleBatchProcessor_ProcessBatch demonstrates batch processing
// of multiple items with concurrent execution.
func ExampleBatchProcessor_ProcessBatch() {
	config := concurrency.DefaultBatchConfig()
	processor := concurrency.NewBatchProcessor(config)

	items := []interface{}{"item1", "item2", "item3"}

	// Process function for each item
	processFn := func(ctx context.Context, item interface{}) (interface{}, error) {
		return fmt.Sprintf("processed-%v", item), nil
	}

	ctx := context.Background()
	results, err := processor.ProcessBatch(ctx, items, processFn)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Processed %d items\n", len(results))
	// Output: Processed 3 items
}

// ExampleParallelMap demonstrates parallel mapping over a collection
// with automatic concurrency control.
func ExampleParallelMap() {
	items := []int{1, 2, 3, 4, 5}

	// Map function to apply to each item
	mapFn := func(ctx context.Context, item int) (interface{}, error) {
		return item * 2, nil
	}

	ctx := context.Background()
	results, err := concurrency.ParallelMap(ctx, items, mapFn, 3)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Mapped %d items\n", len(results))
	// Output: Mapped 5 items
}

// ExampleParallelForEach demonstrates parallel iteration over a collection
// with side effects.
func ExampleParallelForEach() {
	items := []string{"file1.txt", "file2.txt", "file3.txt"}

	// Process function for each item
	processFn := func(ctx context.Context, item string) error {
		// Simulate processing
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	ctx := context.Background()
	err := concurrency.ParallelForEach(ctx, items, processFn, 2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Processed %d files\n", len(items))
	// Output: Processed 3 files
}

// ExampleNewOrchestrator demonstrates creating a concurrency orchestrator
// for coordinating multiple concurrent operations.
func ExampleNewOrchestrator() {
	config := concurrency.DefaultOrchestratorConfig()
	config.MaxConcurrency = 5

	orch := concurrency.NewOrchestrator(config)
	defer orch.Shutdown(context.Background())

	fmt.Printf("Orchestrator created with max concurrency=%d\n", config.MaxConcurrency)
	// Output: Orchestrator created with max concurrency=5
}

// ExampleOrchestrator_Execute demonstrates executing coordinated tasks
// with dependency management.
func ExampleOrchestrator_Execute() {
	config := concurrency.DefaultOrchestratorConfig()
	orch := concurrency.NewOrchestrator(config)
	defer orch.Shutdown(context.Background())

	// Define tasks
	tasks := []concurrency.OrchestratedTask{
		{
			ID: "task1",
			Fn: func(ctx context.Context) (interface{}, error) {
				return "result1", nil
			},
		},
		{
			ID: "task2",
			Fn: func(ctx context.Context) (interface{}, error) {
				return "result2", nil
			},
		},
	}

	ctx := context.Background()
	results, err := orch.Execute(ctx, tasks)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Executed %d tasks\n", len(results))
}
