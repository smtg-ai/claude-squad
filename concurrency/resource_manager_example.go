package concurrency

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ExampleResourceManager demonstrates the usage of ResourceManager
func ExampleResourceManager() {
	// Create resource manager with default config
	config := DefaultResourceManagerConfig()
	rm, err := NewResourceManager(config)
	if err != nil {
		log.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	// Set quotas for different agents
	rm.SetQuota("agent1", CPU, 50)
	rm.SetQuota("agent1", Memory, 512*1024*1024) // 512MB
	rm.SetQuota("agent2", CPU, 30)
	rm.SetQuota("agent2", Memory, 256*1024*1024) // 256MB

	// Register callback for load monitoring
	rm.RegisterLoadCallback(func(rt ResourceType, load float64) {
		fmt.Printf("Resource %s at %.2f%% load\n", rt, load)
	})

	// Simulate concurrent resource acquisition
	var wg sync.WaitGroup
	ctx := context.Background()

	// Agent 1 tasks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			// Acquire CPU resources
			if err := rm.Acquire(ctx, "agent1", CPU, 10); err != nil {
				log.Printf("Agent1 task %d failed to acquire CPU: %v", taskID, err)
				return
			}
			defer rm.Release("agent1", CPU, 10)

			// Acquire Memory resources
			if err := rm.Acquire(ctx, "agent1", Memory, 100*1024*1024); err != nil {
				log.Printf("Agent1 task %d failed to acquire Memory: %v", taskID, err)
				return
			}
			defer rm.Release("agent1", Memory, 100*1024*1024)

			// Simulate work
			fmt.Printf("Agent1 task %d working with resources\n", taskID)
			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	// Agent 2 tasks
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			// Try acquire without blocking
			acquired, err := rm.TryAcquire("agent2", CPU, 10)
			if err != nil {
				log.Printf("Agent2 task %d error: %v", taskID, err)
				return
			}
			if !acquired {
				log.Printf("Agent2 task %d could not acquire CPU immediately", taskID)
				return
			}
			defer rm.Release("agent2", CPU, 10)

			// Acquire file handles
			if err := rm.Acquire(ctx, "agent2", FileHandles, 5); err != nil {
				log.Printf("Agent2 task %d failed to acquire FileHandles: %v", taskID, err)
				return
			}
			defer rm.Release("agent2", FileHandles, 5)

			fmt.Printf("Agent2 task %d working with resources\n", taskID)
			time.Sleep(50 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	// Print statistics
	printStatistics(rm)
}

// ExampleResourceManagerWithDeadlockDetection demonstrates deadlock detection
func ExampleResourceManagerWithDeadlockDetection() {
	config := DefaultResourceManagerConfig()
	config.EnableDeadlockDetection = true

	rm, err := NewResourceManager(config)
	if err != nil {
		log.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	ctx := context.Background()

	// This would detect potential deadlock scenarios
	go func() {
		if err := rm.Acquire(ctx, "agent1", CPU, 50); err != nil {
			log.Printf("Agent1 acquire failed: %v", err)
			return
		}
		defer rm.Release("agent1", CPU, 50)

		time.Sleep(100 * time.Millisecond)

		if err := rm.Acquire(ctx, "agent1", Memory, 512*1024*1024); err != nil {
			log.Printf("Agent1 acquire Memory failed: %v", err)
			return
		}
		defer rm.Release("agent1", Memory, 512*1024*1024)
	}()

	time.Sleep(50 * time.Millisecond)

	go func() {
		if err := rm.Acquire(ctx, "agent2", Memory, 256*1024*1024); err != nil {
			log.Printf("Agent2 acquire failed: %v", err)
			return
		}
		defer rm.Release("agent2", Memory, 256*1024*1024)

		time.Sleep(100 * time.Millisecond)

		if err := rm.Acquire(ctx, "agent2", CPU, 30); err != nil {
			log.Printf("Agent2 acquire CPU failed: %v", err)
			return
		}
		defer rm.Release("agent2", CPU, 30)
	}()

	time.Sleep(500 * time.Millisecond)
}

// ExampleResourceManagerWithContextCancellation demonstrates context cancellation
func ExampleResourceManagerWithContextCancellation() {
	rm, err := NewResourceManager(DefaultResourceManagerConfig())
	if err != nil {
		log.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Try to acquire resources with timeout
	if err := rm.Acquire(ctx, "agent1", CPU, 100); err != nil {
		if err == context.DeadlineExceeded {
			fmt.Println("Resource acquisition timed out")
		} else {
			log.Printf("Error acquiring resource: %v", err)
		}
		return
	}
	defer rm.Release("agent1", CPU, 100)

	fmt.Println("Successfully acquired resources within timeout")
}

// ExampleResourceManagerDynamicScaling demonstrates dynamic capacity scaling
func ExampleResourceManagerDynamicScaling() {
	config := DefaultResourceManagerConfig()
	config.ScaleUpThreshold = 70.0
	config.ScaleDownThreshold = 30.0
	config.MonitorInterval = 1 * time.Second

	rm, err := NewResourceManager(config)
	if err != nil {
		log.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	ctx := context.Background()

	// Register callback to see scaling events
	rm.RegisterLoadCallback(func(rt ResourceType, load float64) {
		fmt.Printf("[Monitor] %s at %.2f%% load\n", rt, load)
	})

	// Create high load
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			if err := rm.Acquire(ctx, fmt.Sprintf("agent%d", id), CPU, 15); err != nil {
				log.Printf("Agent %d failed: %v", id, err)
				return
			}
			defer rm.Release(fmt.Sprintf("agent%d", id), CPU, 15)

			time.Sleep(2 * time.Second)
		}(i)
	}

	// Wait for auto-scaling to happen
	time.Sleep(3 * time.Second)

	wg.Wait()

	// Print final statistics
	printStatistics(rm)
}

// ExampleResourceManagerRateLimiting demonstrates token bucket rate limiting
func ExampleResourceManagerRateLimiting() {
	// Create token bucket with capacity 10, refill rate 5/second
	tb, err := NewTokenBucket(10, 5)
	if err != nil {
		log.Fatalf("Failed to create token bucket: %v", err)
	}
	defer tb.Stop()

	ctx := context.Background()

	// Rapid requests will be rate limited
	start := time.Now()
	for i := 0; i < 20; i++ {
		if err := tb.Acquire(ctx, 1); err != nil {
			log.Printf("Request %d failed: %v", i, err)
			continue
		}

		elapsed := time.Since(start)
		fmt.Printf("Request %d processed at %v\n", i, elapsed)
	}
}

// ExampleResourceManagerSemaphore demonstrates semaphore usage
func ExampleResourceManagerSemaphore() {
	// Create semaphore with capacity 3
	sem, err := NewSemaphore(3)
	if err != nil {
		log.Fatalf("Failed to create semaphore: %v", err)
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	// Launch 10 goroutines, but only 3 can run concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			if err := sem.Acquire(ctx, 1); err != nil {
				log.Printf("Goroutine %d failed to acquire: %v", id, err)
				return
			}
			defer sem.Release(1)

			fmt.Printf("Goroutine %d running (available: %d)\n", id, sem.Available())
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Goroutine %d done\n", id)
		}(i)
	}

	wg.Wait()
}

// printStatistics prints resource manager statistics
func printStatistics(rm *ResourceManager) {
	fmt.Println("\n=== Resource Manager Statistics ===")

	resourceTypes := []ResourceType{CPU, Memory, FileHandles, Network}
	for _, rt := range resourceTypes {
		usage, err := rm.GetPoolUsage(rt)
		if err != nil {
			log.Printf("Error getting usage for %s: %v", rt, err)
			continue
		}

		current, peak, acquisitions, failures, avgWait, err := rm.GetPoolStats(rt)
		if err != nil {
			log.Printf("Error getting stats for %s: %v", rt, err)
			continue
		}

		fmt.Printf("\n%s:\n", rt)
		fmt.Printf("  Current Usage: %.2f%%\n", usage)
		fmt.Printf("  Current Amount: %d\n", current)
		fmt.Printf("  Peak Usage: %d\n", peak)
		fmt.Printf("  Acquisitions: %d\n", acquisitions)
		fmt.Printf("  Failures: %d\n", failures)
		fmt.Printf("  Avg Wait Time: %v\n", avgWait)
	}

	fmt.Println("\n=== Agent Usage ===")
	agents := []string{"agent1", "agent2"}
	for _, agent := range agents {
		fmt.Printf("\n%s:\n", agent)
		for _, rt := range resourceTypes {
			usage := rm.GetUsage(agent, rt)
			if usage > 0 {
				fmt.Printf("  %s: %d\n", rt, usage)
			}
		}
	}
}
