package ollama

import (
	"context"
	"fmt"
	"log"
	"time"

	"claude-squad/session"
)

// Example1_BasicPoolUsage demonstrates basic agent pool creation and usage
func Example1_BasicPoolUsage() {
	config := DefaultPoolConfig()
	config.MinPoolSize = 2
	config.MaxPoolSize = 5

	pool, err := NewAgentPool(config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Acquire an agent from the pool
	agent, err := pool.Acquire(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Acquired agent: %s\n", agent.GetInstance().Title)
	fmt.Printf("Agent state: %d\n", agent.GetState())
	fmt.Printf("Total requests: %d\n", agent.GetTotalRequests())

	// Use the agent...
	// agent.GetInstance().SendPrompt("some command")

	// Release the agent back to the pool
	if err := pool.Release(agent); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Agent released back to pool")
}

// Example2_PoolMetrics demonstrates retrieving pool metrics
func Example2_PoolMetrics() {
	pool, err := NewAgentPool(DefaultPoolConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Get pool size information
	active, idle, total := pool.GetPoolSize()
	fmt.Printf("Pool size - Active: %d, Idle: %d, Total: %d\n", active, idle, total)

	// Get detailed metrics
	metrics := pool.GetMetrics()
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Total recycles: %d\n", metrics.TotalRecycles)
	fmt.Printf("Spawned agents: %d\n", metrics.SpawnedAgents)

	// Get comprehensive statistics
	stats := pool.GetAgentPoolStats()
	fmt.Printf("Pool stats: %+v\n", stats)
}

// Example3_ResourceQuotas demonstrates configuring resource limits
func Example3_ResourceQuotas() {
	config := DefaultPoolConfig()
	config.ResourceQuota = ResourceQuota{
		MaxMemoryMB:      1024,          // 1GB max per instance
		MaxCPUPercent:    90.0,          // 90% CPU utilization
		MaxInstanceAge:   2 * time.Hour, // Recycle instances older than 2 hours
		MaxRecyclesPerID: 50,            // Recycle after 50 uses
		RequestsPerQuota: 5000,          // Track requests per quota period
	}

	pool, err := NewAgentPool(config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	fmt.Printf("Pool configured with resource quotas:\n")
	fmt.Printf("  Max Memory: %d MB\n", config.ResourceQuota.MaxMemoryMB)
	fmt.Printf("  Max CPU: %.1f%%\n", config.ResourceQuota.MaxCPUPercent)
	fmt.Printf("  Max Instance Age: %v\n", config.ResourceQuota.MaxInstanceAge)
	fmt.Printf("  Max Recycles Per ID: %d\n", config.ResourceQuota.MaxRecyclesPerID)
}

// Example4_AutoScaling demonstrates the auto-scaling capabilities
func Example4_AutoScaling() {
	// Configure for aggressive scaling
	config := PoolConfig{
		MinPoolSize:         1,
		MaxPoolSize:         10,
		IdleTimeout:         30 * time.Second,
		RecycleThreshold:    100,
		MaintenanceInterval: 5 * time.Second, // Check every 5 seconds for demo
		ResourceQuota:       DefaultPoolConfig().ResourceQuota,
	}

	pool, err := NewAgentPool(config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simulate load
	fmt.Println("Simulating varying workload to trigger auto-scaling...")

	// High load - should scale up
	agents := make([]*Agent, 0)
	for i := 0; i < 5; i++ {
		agent, err := pool.Acquire(ctx)
		if err != nil {
			log.Printf("Failed to acquire agent: %v\n", err)
			break
		}
		agents = append(agents, agent)
		fmt.Printf("Acquired agent %d (utilization: 5 active)\n", i+1)
	}

	// Check metrics after high load
	metrics := pool.GetMetrics()
	fmt.Printf("After load - Active agents: %d\n", metrics.ActiveAgents)

	// Release all agents
	for _, agent := range agents {
		pool.Release(agent)
	}

	// Low load - should scale down after idle timeout
	fmt.Println("Agents released. Waiting for scale-down triggered by idle timeout...")
	time.Sleep(2 * time.Second)

	metrics = pool.GetMetrics()
	fmt.Printf("After idle period - Active agents: %d\n", metrics.ActiveAgents)
}

// Example5_StorageIntegration demonstrates integration with session.Storage
func Example5_StorageIntegration() {
	// This would require a configured session.Storage
	// In a real application, you'd pass a properly initialized storage
	config := DefaultPoolConfig()
	config.Storage = nil // Would be nil without proper storage setup

	pool, err := NewAgentPool(config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if config.Storage != nil {
		// Save pool state
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := pool.SaveState(ctx); err != nil {
			log.Printf("Failed to save pool state: %v\n", err)
		}

		// Load pool state
		if err := pool.LoadState(ctx); err != nil {
			log.Printf("Failed to load pool state: %v\n", err)
		}
	}

	fmt.Println("Storage integration example completed")
}

// Example6_AgentStateManagement demonstrates agent state transitions
func Example6_AgentStateManagement() {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "example-agent",
		Path:    "/tmp",
		Program: "bash",
	})
	if err != nil {
		log.Fatal(err)
	}

	agent := NewAgent(instance)

	// State transitions
	states := []AgentState{
		AgentStateIdle,
		AgentStateActive,
		AgentStateRecycling,
		AgentStateTerminated,
	}

	stateNames := map[AgentState]string{
		AgentStateIdle:       "Idle",
		AgentStateActive:     "Active",
		AgentStateRecycling:  "Recycling",
		AgentStateTerminated: "Terminated",
	}

	for _, state := range states {
		agent.SetState(state)
		fmt.Printf("Agent state changed to: %s\n", stateNames[agent.GetState()])
	}
}

// Example7_PoolWarmingAndDraining demonstrates pool maintenance operations
func Example7_PoolWarmingAndDraining() {
	config := DefaultPoolConfig()
	config.MinPoolSize = 2
	config.MaxPoolSize = 10

	pool, err := NewAgentPool(config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Warm the pool to ensure minimum agents are available
	if err := pool.WarmPool(context.Background()); err != nil {
		log.Printf("Failed to warm pool: %v\n", err)
	}
	fmt.Println("Pool warmed with minimum agents")

	// Drain excess agents while maintaining minimum
	if err := pool.DrainPool(); err != nil {
		log.Printf("Failed to drain pool: %v\n", err)
	}
	fmt.Println("Pool drained of excess agents")

	active, idle, total := pool.GetPoolSize()
	fmt.Printf("Final pool size - Active: %d, Idle: %d, Total: %d\n", active, idle, total)
}

// Example8_ErrorHandling demonstrates error handling in the pool
func Example8_ErrorHandling() {
	config := DefaultPoolConfig()
	pool, err := NewAgentPool(config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt to acquire with timeout
	agent, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Printf("Failed to acquire agent: %v\n", err)
		return
	}
	defer pool.Release(agent)

	// Close the pool
	pool.Close()

	// Try to acquire from closed pool
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	_, err = pool.Acquire(ctx2)
	if err != nil {
		fmt.Printf("Expected error from closed pool: %v\n", err)
	}
}

// ExampleAgentMetricsTracking demonstrates tracking individual agent metrics
func ExampleAgentMetricsTracking() {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "tracked-agent",
		Path:    "/tmp",
		Program: "bash",
	})
	if err != nil {
		log.Fatal(err)
	}

	agent := NewAgent(instance)

	// Simulate agent usage
	for i := 0; i < 10; i++ {
		agent.IncrementRequests()
	}

	// Log metrics
	fmt.Printf("Agent: %s\n", agent.GetInstance().Title)
	fmt.Printf("Total requests: %d\n", agent.GetTotalRequests())
	fmt.Printf("Recycle count: %d\n", agent.GetRecycleCount())
	fmt.Printf("Idle time: %v\n", agent.GetIdleTime())
	fmt.Printf("Agent state: %d\n", agent.GetState())

	// Simulate recycling
	agent.IncrementRecycleCount()
	fmt.Printf("After recycle - Recycle count: %d\n", agent.GetRecycleCount())
}
