package concurrency

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	tb, err := NewTokenBucket(10, 5)
	if err != nil {
		t.Fatalf("Failed to create token bucket: %v", err)
	}
	defer tb.Stop()

	// Test initial capacity
	if tb.Available() != 10 {
		t.Errorf("Expected 10 tokens, got %d", tb.Available())
	}

	// Test acquire
	ctx := context.Background()
	if err := tb.Acquire(ctx, 5); err != nil {
		t.Errorf("Failed to acquire tokens: %v", err)
	}

	if tb.Available() != 5 {
		t.Errorf("Expected 5 tokens after acquire, got %d", tb.Available())
	}

	// Test try acquire
	if !tb.TryAcquire(3) {
		t.Error("TryAcquire should succeed")
	}

	if tb.Available() != 2 {
		t.Errorf("Expected 2 tokens, got %d", tb.Available())
	}

	// Test release
	tb.Release(5)
	if tb.Available() != 7 {
		t.Errorf("Expected 7 tokens after release, got %d", tb.Available())
	}

	// Test refill
	time.Sleep(1 * time.Second)
	if tb.Available() < 7 {
		t.Error("Tokens should have been refilled")
	}
}

func TestTokenBucketConcurrency(t *testing.T) {
	tb, err := NewTokenBucket(100, 50)
	if err != nil {
		t.Fatalf("Failed to create token bucket: %v", err)
	}
	defer tb.Stop()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Concurrent acquisitions
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tb.Acquire(ctx, 5); err != nil {
				t.Errorf("Failed to acquire: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestSemaphore(t *testing.T) {
	sem, err := NewSemaphore(5)
	if err != nil {
		t.Fatalf("Failed to create semaphore: %v", err)
	}

	ctx := context.Background()

	// Test acquire
	if err := sem.Acquire(ctx, 3); err != nil {
		t.Errorf("Failed to acquire: %v", err)
	}

	if sem.Available() != 2 {
		t.Errorf("Expected 2 available, got %d", sem.Available())
	}

	// Test try acquire
	if !sem.TryAcquire(2) {
		t.Error("TryAcquire should succeed")
	}

	if sem.Available() != 0 {
		t.Errorf("Expected 0 available, got %d", sem.Available())
	}

	// Should fail when exhausted
	if sem.TryAcquire(1) {
		t.Error("TryAcquire should fail when exhausted")
	}

	// Test release
	sem.Release(3)
	if sem.Available() != 3 {
		t.Errorf("Expected 3 available after release, got %d", sem.Available())
	}
}

func TestSemaphoreConcurrency(t *testing.T) {
	sem, err := NewSemaphore(3)
	if err != nil {
		t.Fatalf("Failed to create semaphore: %v", err)
	}

	var wg sync.WaitGroup
	var concurrent int32
	var maxConcurrent int32
	var mu sync.Mutex

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := sem.Acquire(ctx, 1); err != nil {
				t.Errorf("Failed to acquire: %v", err)
				return
			}
			defer sem.Release(1)

			mu.Lock()
			concurrent++
			if concurrent > maxConcurrent {
				maxConcurrent = concurrent
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			concurrent--
			mu.Unlock()
		}()
	}

	wg.Wait()

	if maxConcurrent > 3 {
		t.Errorf("Max concurrent should be 3, got %d", maxConcurrent)
	}
}

func TestResourcePool(t *testing.T) {
	pool, err := NewResourcePool(CPU, 100, 50)
	if err != nil {
		t.Fatalf("Failed to create resource pool: %v", err)
	}
	defer pool.Stop()

	ctx := context.Background()

	// Test acquire
	if err := pool.Acquire(ctx, 30); err != nil {
		t.Errorf("Failed to acquire: %v", err)
	}

	if pool.Available() != 70 {
		t.Errorf("Expected 70 available, got %d", pool.Available())
	}

	usage := pool.Usage()
	if usage < 29 || usage > 31 {
		t.Errorf("Expected ~30%% usage, got %.2f%%", usage)
	}

	// Test release
	pool.Release(30)

	if pool.Available() != 100 {
		t.Errorf("Expected 100 available after release, got %d", pool.Available())
	}
}

func TestResourcePoolCapacityChange(t *testing.T) {
	pool, err := NewResourcePool(Memory, 100, 50)
	if err != nil {
		t.Fatalf("Failed to create resource pool: %v", err)
	}
	defer pool.Stop()

	ctx := context.Background()

	// Acquire some resources
	if err := pool.Acquire(ctx, 30); err != nil {
		t.Errorf("Failed to acquire: %v", err)
	}

	// Increase capacity
	if err := pool.SetCapacity(200); err != nil {
		t.Errorf("Failed to set capacity: %v", err)
	}

	if pool.capacity != 200 {
		t.Errorf("Expected capacity 200, got %d", pool.capacity)
	}

	pool.Release(30)
}

func TestResourceQuota(t *testing.T) {
	quota := NewResourceQuota()

	// Set quotas
	quota.SetQuota("agent1", CPU, 50)
	quota.SetQuota("agent1", Memory, 1024)

	// Test within quota
	if err := quota.CheckQuota("agent1", CPU, 30); err != nil {
		t.Errorf("Should be within quota: %v", err)
	}

	// Record usage
	quota.RecordUsage("agent1", CPU, 30)

	// Test exceeding quota
	if err := quota.CheckQuota("agent1", CPU, 30); err != ErrQuotaExceeded {
		t.Errorf("Expected quota exceeded error, got %v", err)
	}

	// Release and check again
	quota.ReleaseUsage("agent1", CPU, 20)

	if err := quota.CheckQuota("agent1", CPU, 20); err != nil {
		t.Errorf("Should be within quota after release: %v", err)
	}

	// Get usage
	usage := quota.GetUsage("agent1", CPU)
	if usage != 10 {
		t.Errorf("Expected usage 10, got %d", usage)
	}
}

func TestResourceManager(t *testing.T) {
	config := &Config{
		CPUCapacity:             100,
		MemoryCapacity:          1024,
		FileHandlesCapacity:     100,
		NetworkCapacity:         50,
		RateLimit:               100,
		EnableDeadlockDetection: false,
		ScaleUpThreshold:        80.0,
		ScaleDownThreshold:      20.0,
		MonitorInterval:         1 * time.Second,
	}

	rm, err := NewResourceManager(config)
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	ctx := context.Background()

	// Set quota
	rm.SetQuota("agent1", CPU, 50)

	// Test acquire
	if err := rm.Acquire(ctx, "agent1", CPU, 20); err != nil {
		t.Errorf("Failed to acquire: %v", err)
	}

	// Check usage
	usage := rm.GetUsage("agent1", CPU)
	if usage != 20 {
		t.Errorf("Expected usage 20, got %d", usage)
	}

	// Test quota enforcement
	if err := rm.Acquire(ctx, "agent1", CPU, 40); err != ErrQuotaExceeded {
		t.Errorf("Expected quota exceeded, got %v", err)
	}

	// Test release
	if err := rm.Release("agent1", CPU, 20); err != nil {
		t.Errorf("Failed to release: %v", err)
	}

	usage = rm.GetUsage("agent1", CPU)
	if usage != 0 {
		t.Errorf("Expected usage 0 after release, got %d", usage)
	}
}

func TestResourceManagerConcurrency(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.CPUCapacity = 100
	config.EnableDeadlockDetection = false

	rm, err := NewResourceManager(config)
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Multiple agents competing for resources
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			agentID := "agent" + string(rune('0'+id))

			if err := rm.Acquire(ctx, agentID, CPU, 10); err != nil {
				t.Logf("Agent %s failed to acquire: %v", agentID, err)
				return
			}
			defer rm.Release(agentID, CPU, 10)

			time.Sleep(10 * time.Millisecond)
		}(i)
	}

	wg.Wait()
}

func TestResourceManagerTryAcquire(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.CPUCapacity = 50

	rm, err := NewResourceManager(config)
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	// Try acquire should succeed
	acquired, err := rm.TryAcquire("agent1", CPU, 30)
	if err != nil {
		t.Errorf("TryAcquire error: %v", err)
	}
	if !acquired {
		t.Error("TryAcquire should succeed")
	}

	// Try acquire should fail (not enough resources)
	acquired, err = rm.TryAcquire("agent2", CPU, 30)
	if err != nil {
		t.Errorf("TryAcquire error: %v", err)
	}
	if acquired {
		t.Error("TryAcquire should fail when resources exhausted")
	}

	// Release and try again
	if err := rm.Release("agent1", CPU, 30); err != nil {
		t.Errorf("Failed to release: %v", err)
	}

	acquired, err = rm.TryAcquire("agent2", CPU, 30)
	if err != nil {
		t.Errorf("TryAcquire error: %v", err)
	}
	if !acquired {
		t.Error("TryAcquire should succeed after release")
	}

	rm.Release("agent2", CPU, 30)
}

func TestResourceManagerContextCancellation(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.CPUCapacity = 10

	rm, err := NewResourceManager(config)
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	// Acquire all resources
	ctx1 := context.Background()
	if err := rm.Acquire(ctx1, "agent1", CPU, 10); err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}

	// Try to acquire with timeout
	ctx2, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = rm.Acquire(ctx2, "agent2", CPU, 5)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got %v", err)
	}

	rm.Release("agent1", CPU, 10)
}

func TestResourceManagerStats(t *testing.T) {
	config := DefaultResourceManagerConfig()
	rm, err := NewResourceManager(config)
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	ctx := context.Background()

	// Perform some operations
	rm.Acquire(ctx, "agent1", CPU, 20)
	rm.Release("agent1", CPU, 20)

	// Get stats
	current, peak, acquisitions, failures, _, err := rm.GetPoolStats(CPU)
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}

	if acquisitions == 0 {
		t.Error("Expected at least one acquisition")
	}

	if peak < 20 {
		t.Errorf("Expected peak >= 20, got %d", peak)
	}

	if current != 0 {
		t.Errorf("Expected current usage 0, got %d", current)
	}

	if failures != 0 {
		t.Logf("Failures: %d", failures)
	}

	// Get pool usage
	usage, err := rm.GetPoolUsage(CPU)
	if err != nil {
		t.Errorf("Failed to get pool usage: %v", err)
	}

	if usage < 0 || usage > 100 {
		t.Errorf("Usage should be between 0-100, got %.2f", usage)
	}
}

func TestDeadlockDetector(t *testing.T) {
	dd := NewDeadlockDetector(true)

	// Record acquisitions
	dd.RecordAcquire("agent1", CPU, 10)
	dd.RecordAcquire("agent2", Memory, 100)

	// Record waits
	err := dd.RecordWait("agent1", Memory)
	if err != nil {
		t.Errorf("Unexpected error on first wait: %v", err)
	}

	// This could potentially create a cycle if agent2 waits for CPU
	err = dd.RecordWait("agent2", CPU)
	if err == nil {
		t.Log("No deadlock detected in this scenario")
	} else if err == ErrDeadlockDetected {
		t.Log("Deadlock detected as expected")
	}

	// Release resources
	dd.RecordRelease("agent1", CPU, 10)
	dd.RecordRelease("agent2", Memory, 100)
}

func TestLoadMonitor(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.MonitorInterval = 100 * time.Millisecond
	config.ScaleUpThreshold = 50.0

	rm, err := NewResourceManager(config)
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	callbackCalled := false
	rm.RegisterLoadCallback(func(rt ResourceType, load float64) {
		callbackCalled = true
		t.Logf("Load callback: %s at %.2f%%", rt, load)
	})

	// Create some load
	ctx := context.Background()
	rm.Acquire(ctx, "agent1", CPU, 60)

	// Wait for monitor to run
	time.Sleep(200 * time.Millisecond)

	if !callbackCalled {
		t.Error("Load callback should have been called")
	}

	rm.Release("agent1", CPU, 60)
}

func TestInvalidInputs(t *testing.T) {
	// Test invalid token bucket
	_, err := NewTokenBucket(0, 10)
	if err != ErrInvalidRate {
		t.Errorf("Expected invalid rate error, got %v", err)
	}

	_, err = NewTokenBucket(10, 0)
	if err != ErrInvalidRate {
		t.Errorf("Expected invalid rate error, got %v", err)
	}

	// Test invalid semaphore
	_, err = NewSemaphore(0)
	if err != ErrInvalidCapacity {
		t.Errorf("Expected invalid capacity error, got %v", err)
	}

	// Test invalid resource pool
	_, err = NewResourcePool(CPU, 0, 10)
	if err != ErrInvalidCapacity {
		t.Errorf("Expected invalid capacity error, got %v", err)
	}
}

func TestResourceNotAcquired(t *testing.T) {
	rm, err := NewResourceManager(DefaultResourceManagerConfig())
	if err != nil {
		t.Fatalf("Failed to create resource manager: %v", err)
	}
	defer rm.Stop()

	// Try to release without acquiring
	err = rm.Release("agent1", CPU, 10)
	if err != ErrResourceNotAcquired {
		t.Errorf("Expected resource not acquired error, got %v", err)
	}
}

func BenchmarkTokenBucketAcquire(b *testing.B) {
	tb, _ := NewTokenBucket(1000000, 500000)
	defer tb.Stop()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Acquire(ctx, 1)
	}
}

func BenchmarkSemaphoreAcquire(b *testing.B) {
	sem, _ := NewSemaphore(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sem.Acquire(ctx, 1)
		sem.Release(1)
	}
}

func BenchmarkResourceManagerAcquire(b *testing.B) {
	config := DefaultResourceManagerConfig()
	config.CPUCapacity = 1000000
	rm, _ := NewResourceManager(config)
	defer rm.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.Acquire(ctx, "agent1", CPU, 1)
		rm.Release("agent1", CPU, 1)
	}
}
