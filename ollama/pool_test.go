package ollama

import (
	"claude-squad/session"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-agent",
		Path:    "/tmp",
		Program: "bash",
	})
	require.NoError(t, err)

	agent := NewAgent(instance)
	assert.NotNil(t, agent)
	assert.Equal(t, agent.GetState(), AgentStateIdle)
	assert.Equal(t, agent.GetTotalRequests(), int64(0))
	assert.Equal(t, agent.GetRecycleCount(), int32(0))
	assert.Equal(t, agent.GetInstance(), instance)
}

func TestAgentMetrics(t *testing.T) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-agent",
		Path:    "/tmp",
		Program: "bash",
	})
	require.NoError(t, err)

	agent := NewAgent(instance)

	// Test request counting
	for i := 0; i < 5; i++ {
		agent.IncrementRequests()
	}
	assert.Equal(t, agent.GetTotalRequests(), int64(5))

	// Test state management
	agent.SetState(AgentStateActive)
	assert.Equal(t, agent.GetState(), AgentStateActive)

	agent.SetState(AgentStateIdle)
	assert.Equal(t, agent.GetState(), AgentStateIdle)

	// Test recycle counting
	for i := 0; i < 3; i++ {
		agent.IncrementRecycleCount()
	}
	assert.Equal(t, agent.GetRecycleCount(), int32(3))
}

func TestAgentIdleTime(t *testing.T) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-agent",
		Path:    "/tmp",
		Program: "bash",
	})
	require.NoError(t, err)

	agent := NewAgent(instance)
	initialLastUsed := agent.GetLastUsed()

	time.Sleep(100 * time.Millisecond)

	// Idle time should increase
	idleTime1 := agent.GetIdleTime()
	assert.True(t, idleTime1 >= 100*time.Millisecond)

	// Update last used
	agent.UpdateLastUsed()
	newLastUsed := agent.GetLastUsed()
	assert.True(t, newLastUsed.After(initialLastUsed))

	// Idle time should reset
	idleTime2 := agent.GetIdleTime()
	assert.True(t, idleTime2 < 50*time.Millisecond)
}

func TestNewAgentPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()
	assert.Equal(t, config.MinPoolSize, 1)
	assert.Equal(t, config.MaxPoolSize, 10)
	assert.Equal(t, config.IdleTimeout, 5*time.Minute)
	assert.Equal(t, config.RecycleThreshold, int64(1000))
	assert.Equal(t, config.ResourceQuota.MaxMemoryMB, int64(512))
	assert.Equal(t, config.ResourceQuota.MaxCPUPercent, 80.0)
	assert.Equal(t, config.ResourceQuota.MaxInstanceAge, 1*time.Hour)
}

func TestAgentPoolInitialization(t *testing.T) {
	config := PoolConfig{
		MinPoolSize:         2,
		MaxPoolSize:         5,
		IdleTimeout:         1 * time.Minute,
		RecycleThreshold:    100,
		MaintenanceInterval: 10 * time.Second,
		ResourceQuota: ResourceQuota{
			MaxMemoryMB:      512,
			MaxCPUPercent:    80.0,
			MaxInstanceAge:   30 * time.Minute,
			MaxRecyclesPerID: 50,
			RequestsPerQuota: 1000,
		},
	}

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Check pool size constraints
	assert.Equal(t, pool.minPoolSize, 2)
	assert.Equal(t, pool.maxPoolSize, 5)
	assert.True(t, pool.running.Load())
}

func TestAgentPoolMaxSizeConstraint(t *testing.T) {
	config := DefaultPoolConfig()
	config.MaxPoolSize = 15 // Try to set above hard cap

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Should be capped at 10
	assert.Equal(t, pool.maxPoolSize, 10)
}

func TestAgentPoolMinSizeConstraint(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 0

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Should be set to at least 1
	assert.Equal(t, pool.minPoolSize, 1)
}

func TestAgentPoolAcquireAndRelease(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 2
	config.MaxPoolSize = 5

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Acquire an agent
	agent, err := pool.Acquire(ctx)
	require.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, agent.GetState(), AgentStateActive)

	// Release the agent
	err = pool.Release(agent)
	assert.NoError(t, err)
	assert.Equal(t, agent.GetState(), AgentStateIdle)
}

func TestAgentPoolMetrics(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 1
	config.MaxPoolSize = 5

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get initial metrics
	metrics := pool.GetMetrics()
	assert.Equal(t, metrics.TotalAgents, int64(1))

	// Acquire an agent
	agent, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Verify agent metrics
	assert.Equal(t, agent.GetTotalRequests(), int64(1))

	// Release agent
	pool.Release(agent)
}

func TestAgentPoolPoolSize(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 2
	config.MaxPoolSize = 5

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initially should have 2 agents idle
	active, idle, total := pool.GetPoolSize()
	assert.Equal(t, active, 0)
	assert.GreaterOrEqual(t, idle, 1) // At least 1 from warm pool init
	assert.GreaterOrEqual(t, total, 1)

	// Acquire agents
	agent1, _ := pool.Acquire(ctx)
	active, idle, total = pool.GetPoolSize()
	assert.Equal(t, active, 1)

	agent2, _ := pool.Acquire(ctx)
	active, idle, total = pool.GetPoolSize()
	assert.Equal(t, active, 2)

	// Release agents
	pool.Release(agent1)
	active, idle, total = pool.GetPoolSize()
	assert.Equal(t, active, 1)

	pool.Release(agent2)
	active, idle, total = pool.GetPoolSize()
	assert.Equal(t, active, 0)
}

func TestAgentPoolClosedPoolReject(t *testing.T) {
	config := DefaultPoolConfig()
	pool, err := NewAgentPool(config)
	require.NoError(t, err)

	pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = pool.Acquire(ctx)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "pool is closed")
}

func TestAgentPoolContextCancellation(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 1
	config.MaxPoolSize = 1

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Acquire the only agent to make pool empty
	_, _ = pool.Acquire(ctx)

	// Try to acquire when pool is empty with cancelled context
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()

	_, err = pool.Acquire(ctx2)
	assert.Error(t, err)
}

func TestAgentShouldRecycle(t *testing.T) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   "test-agent",
		Path:    "/tmp",
		Program: "bash",
	})
	require.NoError(t, err)

	config := DefaultPoolConfig()
	config.RecycleThreshold = 10
	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	agent := NewAgent(instance)

	// Should not recycle initially
	assert.False(t, pool.shouldRecycle(agent))

	// Exceed recycle threshold
	for i := 0; i < 15; i++ {
		agent.IncrementRequests()
	}
	assert.True(t, pool.shouldRecycle(agent))
}

func TestAgentPoolListAgents(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 2
	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	agents := pool.ListAgents()
	assert.GreaterOrEqual(t, len(agents), 1)

	for _, agent := range agents {
		assert.NotNil(t, agent)
		assert.NotNil(t, agent.GetInstance())
	}
}

func TestAgentPoolStats(t *testing.T) {
	config := DefaultPoolConfig()
	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	stats := pool.GetAgentPoolStats()
	assert.NotNil(t, stats)

	poolSize, ok := stats["pool_size"].(map[string]int)
	assert.True(t, ok)
	assert.Greater(t, poolSize["total"], 0)
	assert.Equal(t, poolSize["min"], 1)
	assert.Equal(t, poolSize["max"], 10)

	metrics, ok := stats["metrics"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, metrics["total_requests"])
	assert.NotNil(t, metrics["spawned_agents"])

	quotas, ok := stats["quotas"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, quotas["max_memory_mb"])
	assert.NotNil(t, quotas["max_cpu_percent"])
}

func TestWarmPool(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 3

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Warm the pool explicitly
	err = pool.WarmPool(context.Background())
	assert.NoError(t, err)

	// Should have at least min pool size agents
	_, idle, _ := pool.GetPoolSize()
	assert.GreaterOrEqual(t, idle, 1)
}

func TestDrainPool(t *testing.T) {
	config := DefaultPoolConfig()
	config.MinPoolSize = 1
	config.MaxPoolSize = 5

	pool, err := NewAgentPool(config)
	require.NoError(t, err)
	defer pool.Close()

	// Drain should remove excess idle agents but keep minimum
	err = pool.DrainPool()
	assert.NoError(t, err)

	_, idle, _ := pool.GetPoolSize()
	assert.GreaterOrEqual(t, idle, 1) // At least minimum pool size
}
