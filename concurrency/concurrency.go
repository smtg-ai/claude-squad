// Package concurrency provides advanced concurrent processing capabilities for claude-squad.
//
// This package implements a comprehensive suite of concurrency primitives and patterns
// for managing multiple AI agents, parallel git operations, resource management,
// and real-time event streaming.
//
// # Core Components
//
// WorkerPool - Generic worker pool with priority queue, health monitoring, and metrics
//
//	pool := NewWorkerPool(DefaultWorkerPoolConfig())
//	pool.Start()
//	pool.Submit(job)
//	pool.Shutdown(ctx)
//
// AgentOrchestrator - Concurrent agent lifecycle management with load balancing
//
//	orchestrator := NewOrchestrator(DefaultOrchestratorConfig())
//	orchestrator.AddAgent(agent)
//	orchestrator.DistributeTask(task)
//
// TaskQueue - Distributed task queue with DAG execution and retry support
//
//	queue, _ := NewTaskQueue(TaskQueueConfig{...})
//	queue.Enqueue(task)
//	queue.Start(ctx)
//
// MetricsCollector - Thread-safe metrics with Prometheus export
//
//	metrics := NewMetricsCollector()
//	metrics.RecordTask(agent, duration, nil)
//	json := metrics.ExportJSON()
//
// GitPipeline - Parallel git operations across multiple repositories
//
//	pipeline := NewGitPipeline(4)
//	pipeline.AddStage(NewFetchStage(...))
//	pipeline.Execute(ctx)
//
// EventBus - Real-time pub/sub with backpressure and replay
//
//	bus := NewEventBus(DefaultEventBusConfig())
//	bus.Subscribe("topic.*", subscriber)
//	bus.Publish(event)
//
// ResourceManager - Resource pools with rate limiting and deadlock prevention
//
//	rm, _ := NewResourceManager(DefaultResourceManagerConfig())
//	rm.Acquire(ctx, agent, CPU, 10)
//	rm.Release(agent, CPU, 10)
//
// NotificationService - Async notifications with delivery guarantees
//
//	service := NewNotificationService(NotificationServiceConfig{Workers: 5})
//	service.Notify(notification)
//
// BatchExecutor - Transaction-like batch operations with rollback
//
//	executor := NewBatchExecutor(10)
//	executor.ExecuteWithRollback(ctx, instances, operation, tracker)
//
// HealthMonitor - Component health monitoring with auto-recovery
//
//	monitor := NewHealthMonitor(DefaultHealthMonitorConfig())
//	monitor.Start()
//	status, results := monitor.GetHealth()
//
// # Architecture
//
// The concurrency package follows these design principles:
//
//  1. Thread Safety - All components use proper synchronization (mutexes, atomics, channels)
//  2. Context Awareness - All blocking operations accept context for cancellation
//  3. Error Aggregation - Multiple errors are combined with detailed messages
//  4. Resource Cleanup - Proper lifecycle management with defer patterns
//  5. Observability - Built-in metrics and health monitoring
//
// # Integration with claude-squad
//
// The package integrates with existing session.Instance for agent management,
// session/git for repository operations, and session/tmux for terminal sessions.
package concurrency

// Version of the concurrency package
const Version = "1.0.0"

// MaxAgentConcurrency is the recommended maximum number of concurrent agents
const MaxAgentConcurrency = 10

// Component interfaces are defined in their respective files:
// - worker_pool.go: WorkerPool, Job, JobResult
// - orchestrator.go: AgentOrchestrator, ManagedAgent, Task
// - task_queue.go: TaskQueue, QueueTask, BackoffStrategy
// - metrics.go: MetricsCollector, Counter, Gauge, Histogram, Timer
// - git_pipeline.go: GitPipeline, PipelineStage, ConflictResolver
// - event_stream.go: EventBus, Event, Subscriber
// - resource_manager.go: ResourceManager, TokenBucket, Semaphore
// - notifications.go: NotificationService, Notification, NotificationChannel
// - batch_ops.go: BatchExecutor, Operation, PartialResult
// - health_monitor.go: HealthMonitor, HealthCheck, RecoveryAction
