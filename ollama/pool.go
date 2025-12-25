package ollama

import (
	"claude-squad/log"
	"claude-squad/session"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AgentState represents the state of an agent in the pool
type AgentState int

const (
	AgentStateIdle AgentState = iota
	AgentStateActive
	AgentStateRecycling
	AgentStateTerminated
)

// Agent wraps a session.Instance with pool-specific metadata
type Agent struct {
	instance      *session.Instance
	state         AgentState
	mu            sync.RWMutex
	lastUsedAt    time.Time
	totalRequests int64
	createdAt     time.Time
	recycleCount  int32
	idleTime      time.Duration
}

// NewAgent creates a new agent wrapping a session.Instance
func NewAgent(instance *session.Instance) *Agent {
	return &Agent{
		instance:   instance,
		state:      AgentStateIdle,
		lastUsedAt: time.Now(),
		createdAt:  time.Now(),
	}
}

// SetState atomically updates the agent state
func (a *Agent) SetState(state AgentState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// GetState returns the current agent state
func (a *Agent) GetState() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// IncrementRequests increments the request counter
func (a *Agent) IncrementRequests() {
	atomic.AddInt64(&a.totalRequests, 1)
}

// GetTotalRequests returns the total number of requests processed
func (a *Agent) GetTotalRequests() int64 {
	return atomic.LoadInt64(&a.totalRequests)
}

// UpdateLastUsed sets the last used timestamp to now
func (a *Agent) UpdateLastUsed() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastUsedAt = time.Now()
}

// GetLastUsed returns the last used timestamp
func (a *Agent) GetLastUsed() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastUsedAt
}

// GetIdleTime returns how long the agent has been idle
func (a *Agent) GetIdleTime() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return time.Since(a.lastUsedAt)
}

// GetRecycleCount returns the number of times this agent has been recycled
func (a *Agent) GetRecycleCount() int32 {
	return atomic.LoadInt32(&a.recycleCount)
}

// IncrementRecycleCount increments the recycle counter
func (a *Agent) IncrementRecycleCount() {
	atomic.AddInt32(&a.recycleCount, 1)
}

// GetInstance returns the wrapped session.Instance
func (a *Agent) GetInstance() *session.Instance {
	return a.instance
}

// ResourceQuota defines resource limits for the pool
type ResourceQuota struct {
	MaxMemoryMB      int64
	MaxCPUPercent    float64
	MaxInstanceAge   time.Duration
	MaxRecyclesPerID int32
	RequestsPerQuota int64
}

// PoolMetrics tracks pool statistics
type PoolMetrics struct {
	ActiveAgents     int64
	IdleAgents       int64
	TotalAgents      int64
	TotalRequests    int64
	TotalRecycles    int64
	SpawnedAgents    int64
	TerminatedAgents int64
	LastScaleEvent   time.Time
	ScaleDirection   string
}

// AgentPool manages a pool of agents with auto-scaling capabilities
type AgentPool struct {
	mu sync.RWMutex

	// Core pool state
	agents         map[string]*Agent // ID -> Agent
	availableQueue chan *Agent
	closedChan     chan struct{}

	// Configuration
	minPoolSize         int
	maxPoolSize         int
	idleTimeoutDuration time.Duration
	recycleThreshold    int64
	resourceQuota       ResourceQuota

	// State management
	running         atomic.Bool
	activeCount     atomic.Int64
	idleCount       atomic.Int64
	totalRequests   atomic.Int64
	totalRecycles   atomic.Int64
	spawnedCount    atomic.Int64
	terminatedCount atomic.Int64

	// Storage integration
	storage *session.Storage

	// Sync pool for reusing Agent objects (object pool pattern)
	agentPool sync.Pool

	// Background maintenance
	ticker        *time.Ticker
	done          chan struct{}
	maintenanceWg sync.WaitGroup
	wg            sync.WaitGroup // Tracks child goroutines spawned during maintenance

	// Metrics snapshot
	metricsLock sync.RWMutex
	lastMetrics PoolMetrics
}

// PoolConfig holds configuration for the AgentPool
type PoolConfig struct {
	MinPoolSize         int
	MaxPoolSize         int
	IdleTimeout         time.Duration
	RecycleThreshold    int64
	MaintenanceInterval time.Duration
	ResourceQuota       ResourceQuota
	Storage             *session.Storage
}

// DefaultPoolConfig returns a sensible default configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinPoolSize:         1,
		MaxPoolSize:         10,
		IdleTimeout:         5 * time.Minute,
		RecycleThreshold:    1000,
		MaintenanceInterval: 30 * time.Second,
		ResourceQuota: ResourceQuota{
			MaxMemoryMB:      512,
			MaxCPUPercent:    80.0,
			MaxInstanceAge:   1 * time.Hour,
			MaxRecyclesPerID: 100,
			RequestsPerQuota: 5000,
		},
		Storage: nil,
	}
}

// NewAgentPool creates a new agent pool with the given configuration
func NewAgentPool(config PoolConfig) (*AgentPool, error) {
	if config.MinPoolSize < 1 {
		config.MinPoolSize = 1
	}
	if config.MaxPoolSize < config.MinPoolSize {
		config.MaxPoolSize = config.MinPoolSize
	}
	if config.MaxPoolSize > 10 {
		config.MaxPoolSize = 10 // Hard cap at 10
	}

	// Validate ResourceQuota
	if config.ResourceQuota.MaxMemoryMB < 0 {
		config.ResourceQuota.MaxMemoryMB = 512
	}
	if config.ResourceQuota.MaxCPUPercent < 0 || config.ResourceQuota.MaxCPUPercent > 100 {
		config.ResourceQuota.MaxCPUPercent = 80.0
	}
	if config.ResourceQuota.MaxRecyclesPerID < 0 {
		config.ResourceQuota.MaxRecyclesPerID = 100
	}
	if config.ResourceQuota.RequestsPerQuota < 0 {
		config.ResourceQuota.RequestsPerQuota = 5000
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 5 * time.Minute
	}

	pool := &AgentPool{
		agents:              make(map[string]*Agent),
		availableQueue:      make(chan *Agent, config.MaxPoolSize),
		closedChan:          make(chan struct{}),
		minPoolSize:         config.MinPoolSize,
		maxPoolSize:         config.MaxPoolSize,
		idleTimeoutDuration: config.IdleTimeout,
		recycleThreshold:    config.RecycleThreshold,
		resourceQuota:       config.ResourceQuota,
		storage:             config.Storage,
		done:                make(chan struct{}),
		ticker:              time.NewTicker(config.MaintenanceInterval),
		agentPool: sync.Pool{
			New: func() interface{} {
				return &Agent{}
			},
		},
	}

	pool.running.Store(true)

	// Start background maintenance goroutine
	pool.maintenanceWg.Add(1)
	go pool.maintenanceLoop()

	// Initialize warm pool
	if err := pool.initializeWarmPool(); err != nil {
		log.ErrorLog.Printf("failed to initialize warm pool: %v", err)
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// initializeWarmPool creates the minimum number of agents
func (p *AgentPool) initializeWarmPool() error {
	for i := 0; i < p.minPoolSize; i++ {
		agent, err := p.spawnAgent()
		if err != nil {
			log.ErrorLog.Printf("failed to spawn agent %d: %v", i, err)
			// Clean up previously spawned agents
			p.Close()
			return fmt.Errorf("failed to initialize warm pool: %w", err)
		}

		p.mu.Lock()
		defer p.mu.Unlock()
		agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano())
		p.agents[agentID] = agent

		select {
		case p.availableQueue <- agent:
			p.idleCount.Add(1)
		case <-p.closedChan:
			return fmt.Errorf("pool closed during initialization")
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout adding agent to queue during initialization")
		}
	}

	log.InfoLog.Printf("initialized warm pool with %d agents", p.minPoolSize)
	return nil
}

// spawnAgent creates a new agent instance
func (p *AgentPool) spawnAgent() (*Agent, error) {
	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   fmt.Sprintf("agent-%d", time.Now().UnixNano()),
		Path:    "/tmp",
		Program: "bash",
		AutoYes: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	if err := instance.Start(true); err != nil {
		return nil, fmt.Errorf("failed to start instance: %w", err)
	}

	agent := NewAgent(instance)
	agent.SetState(AgentStateIdle)

	p.spawnedCount.Add(1)
	log.InfoLog.Printf("spawned new agent: %s", instance.Title)

	return agent, nil
}

// Acquire retrieves an agent from the pool, spawning one if necessary
func (p *AgentPool) Acquire(ctx context.Context) (*Agent, error) {
	if !p.running.Load() {
		return nil, fmt.Errorf("pool is closed")
	}

	select {
	case <-p.closedChan:
		return nil, fmt.Errorf("pool is closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	case agent := <-p.availableQueue:
		// Verify agent is healthy before returning
		if !p.isAgentHealthy(agent) {
			p.killAgent(agent)
			return p.Acquire(ctx)
		}

		p.idleCount.Add(-1)
		p.activeCount.Add(1)

		agent.SetState(AgentStateActive)
		agent.UpdateLastUsed()
		agent.IncrementRequests()

		p.totalRequests.Add(1)
		return agent, nil

	default:
		// No agents available, try to spawn a new one
		currentSize := p.idleCount.Load() + p.activeCount.Load()
		if currentSize < int64(p.maxPoolSize) {
			agent, err := p.spawnAgent()
			if err != nil {
				// Fallback: wait for available agent with timeout
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case agent := <-p.availableQueue:
					p.idleCount.Add(-1)
					p.activeCount.Add(1)
					agent.SetState(AgentStateActive)
					agent.UpdateLastUsed()
					agent.IncrementRequests()
					return agent, nil
				case <-time.After(5 * time.Second):
					return nil, fmt.Errorf("timeout waiting for agent")
				}
			}

			p.mu.Lock()
			agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano())
			p.agents[agentID] = agent
			p.mu.Unlock()

			agent.SetState(AgentStateActive)
			agent.UpdateLastUsed()
			agent.IncrementRequests()

			p.activeCount.Add(1)
			return agent, nil
		}

		// Wait for agent availability
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case agent := <-p.availableQueue:
			p.idleCount.Add(-1)
			p.activeCount.Add(1)
			agent.SetState(AgentStateActive)
			agent.UpdateLastUsed()
			agent.IncrementRequests()
			return agent, nil
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("timeout acquiring agent")
		}
	}
}

// Release returns an agent to the pool
func (p *AgentPool) Release(agent *Agent) error {
	if agent == nil {
		return fmt.Errorf("cannot release nil agent")
	}

	if !p.running.Load() {
		return p.killAgent(agent)
	}

	// Check if agent needs recycling
	if p.shouldRecycle(agent) {
		return p.recycleAgent(agent)
	}

	agent.SetState(AgentStateIdle)
	p.activeCount.Add(-1)
	p.idleCount.Add(1)

	select {
	case p.availableQueue <- agent:
		return nil
	case <-p.closedChan:
		return p.killAgent(agent)
	default:
		// Queue full, recycle the agent
		return p.recycleAgent(agent)
	}
}

// shouldRecycle determines if an agent should be recycled
func (p *AgentPool) shouldRecycle(agent *Agent) bool {
	// Recycle if max recycles per instance exceeded
	if agent.GetRecycleCount() >= p.resourceQuota.MaxRecyclesPerID {
		return true
	}

	// Recycle if instance age exceeds max
	if time.Since(agent.createdAt) > p.resourceQuota.MaxInstanceAge {
		return true
	}

	// Recycle if requests exceed threshold
	if agent.GetTotalRequests() > p.recycleThreshold {
		return true
	}

	return false
}

// recycleAgent kills and respawns an agent
func (p *AgentPool) recycleAgent(agent *Agent) error {
	agent.SetState(AgentStateRecycling)

	// Kill the old agent
	if err := p.killAgent(agent); err != nil {
		log.ErrorLog.Printf("error killing agent during recycle: %v", err)
	}

	// Try to spawn a new replacement
	newAgent, err := p.spawnAgent()
	if err != nil {
		log.ErrorLog.Printf("failed to spawn replacement agent: %v", err)
		p.activeCount.Add(-1)
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano())
	p.agents[agentID] = newAgent

	p.totalRecycles.Add(1)
	newAgent.IncrementRecycleCount()

	// Return new agent to pool
	newAgent.SetState(AgentStateIdle)
	p.activeCount.Add(-1)
	p.idleCount.Add(1)

	select {
	case p.availableQueue <- newAgent:
	case <-p.closedChan:
		p.killAgent(newAgent)
	}

	return nil
}

// killAgent terminates an agent and cleans up resources
func (p *AgentPool) killAgent(agent *Agent) error {
	if agent == nil {
		return fmt.Errorf("cannot kill nil agent")
	}

	agent.SetState(AgentStateTerminated)
	p.terminatedCount.Add(1)

	if agent.instance != nil {
		if err := agent.instance.Kill(); err != nil {
			log.ErrorLog.Printf("error killing instance: %v", err)
			return err
		}
	}

	p.activeCount.Add(-1)
	return nil
}

// isAgentHealthy checks if an agent is in a valid state
func (p *AgentPool) isAgentHealthy(agent *Agent) bool {
	if agent == nil {
		return false
	}

	state := agent.GetState()
	if state == AgentStateTerminated {
		return false
	}

	if agent.instance == nil {
		return false
	}

	// Check if instance is still running
	if !agent.instance.Started() {
		return false
	}

	return true
}

// maintenanceLoop runs periodic maintenance tasks
func (p *AgentPool) maintenanceLoop() {
	defer p.maintenanceWg.Done()

	for {
		select {
		case <-p.done:
			p.ticker.Stop()
			return
		case <-p.ticker.C:
			p.performMaintenance()
		}
	}
}

// performMaintenance executes periodic pool maintenance
func (p *AgentPool) performMaintenance() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	activeCount := int64(0)
	idleCount := int64(0)
	totalRequests := int64(0)

	// Check each agent
	for id, agent := range p.agents {
		if agent.GetState() == AgentStateTerminated {
			delete(p.agents, id)
			continue
		}

		// Update counts
		if agent.GetState() == AgentStateIdle {
			idleCount++

			// Check if idle timeout exceeded
			if now.Sub(agent.GetLastUsed()) > p.idleTimeoutDuration &&
				idleCount+activeCount > int64(p.minPoolSize) {
				// Safe to remove this idle agent
				p.wg.Add(1)
				go func(a *Agent) {
					defer p.wg.Done()
					if err := p.killAgent(a); err != nil {
						log.ErrorLog.Printf("error killing idle agent: %v", err)
					}
				}(agent)
				delete(p.agents, id)
				continue
			}
		} else {
			activeCount++
		}

		totalRequests += agent.GetTotalRequests()
	}

	// Update metrics
	p.metricsLock.Lock()
	defer p.metricsLock.Unlock()
	p.lastMetrics = PoolMetrics{
		ActiveAgents:     activeCount,
		IdleAgents:       idleCount,
		TotalAgents:      int64(len(p.agents)),
		TotalRequests:    totalRequests,
		TotalRecycles:    p.totalRecycles.Load(),
		SpawnedAgents:    p.spawnedCount.Load(),
		TerminatedAgents: p.terminatedCount.Load(),
		LastScaleEvent:   now,
	}

	// Auto-scale logic
	p.autoscale(activeCount, idleCount)
}

// autoscale adjusts pool size based on demand
func (p *AgentPool) autoscale(activeCount, idleCount int64) {
	totalCount := activeCount + idleCount
	if totalCount > 0 {
		utilization := float64(activeCount) / float64(totalCount)
		// Handle utilization > 80%, scale up if possible
		if utilization > 0.8 && totalCount < int64(p.maxPoolSize) {
			agent, err := p.spawnAgent()
			if err != nil {
				log.ErrorLog.Printf("failed to spawn agent during scale-up: %v", err)
				return
			}

			agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano())
			p.agents[agentID] = agent

			select {
			case p.availableQueue <- agent:
				p.idleCount.Add(1)
				p.metricsLock.Lock()
				defer p.metricsLock.Unlock()
				p.lastMetrics.ScaleDirection = "UP"
			default:
				// Queue full, kill the newly spawned agent
				log.ErrorLog.Printf("failed to add agent to queue during scale-up: queue full")
				p.killAgent(agent)
			}

			log.InfoLog.Printf("scaled pool up: utilization=%.2f%%, total agents=%d", utilization*100, totalCount+1)
		}

		// Handle utilization < 20%, scale down if possible
		if utilization < 0.2 && totalCount > int64(p.minPoolSize) {
			// Remove one idle agent
			for id, agent := range p.agents {
				if agent.GetState() == AgentStateIdle {
					p.wg.Add(1)
					go func(a *Agent) {
						defer p.wg.Done()
						if err := p.killAgent(a); err != nil {
							log.ErrorLog.Printf("error killing agent during scale-down: %v", err)
						}
					}(agent)
					delete(p.agents, id)

					p.metricsLock.Lock()
					defer p.metricsLock.Unlock()
					p.lastMetrics.ScaleDirection = "DOWN"

					log.InfoLog.Printf("scaled pool down: utilization=%.2f%%, total agents=%d", utilization*100, totalCount-1)
					return
				}
			}
		}
	}
}

// GetMetrics returns a snapshot of current pool metrics
func (p *AgentPool) GetMetrics() PoolMetrics {
	p.metricsLock.RLock()
	defer p.metricsLock.RUnlock()
	return p.lastMetrics
}

// GetPoolSize returns current number of agents in the pool
func (p *AgentPool) GetPoolSize() (active, idle, total int) {
	active = int(p.activeCount.Load())
	idle = int(p.idleCount.Load())
	total = active + idle
	return
}

// GetAgent retrieves a specific agent by ID (for testing/debugging)
func (p *AgentPool) GetAgent(id string) (*Agent, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	agent, exists := p.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	return agent, nil
}

// ListAgents returns a list of all agents in the pool
func (p *AgentPool) ListAgents() []*Agent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	agents := make([]*Agent, 0, len(p.agents))
	for _, agent := range p.agents {
		agents = append(agents, agent)
	}

	return agents
}

// Close gracefully shuts down the pool
func (p *AgentPool) Close() error {
	if !p.running.CompareAndSwap(true, false) {
		return fmt.Errorf("pool already closed")
	}

	close(p.closedChan)
	close(p.done)

	// Wait for maintenance loop to finish with timeout
	maintenanceDone := make(chan struct{})
	go func() {
		p.maintenanceWg.Wait()
		close(maintenanceDone)
	}()

	select {
	case <-maintenanceDone:
		log.InfoLog.Printf("maintenance loop stopped gracefully")
	case <-time.After(10 * time.Second):
		log.WarningLog.Printf("maintenance loop shutdown timeout exceeded")
	}

	// Wait for all child goroutines spawned during maintenance to finish with timeout
	childrenDone := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(childrenDone)
	}()

	select {
	case <-childrenDone:
		log.InfoLog.Printf("all child goroutines stopped gracefully")
	case <-time.After(10 * time.Second):
		log.WarningLog.Printf("child goroutines shutdown timeout exceeded")
	}

	// Kill all agents
	p.mu.Lock()
	defer p.mu.Unlock()
	var errs []error
	for _, agent := range p.agents {
		if err := p.killAgent(agent); err != nil {
			errs = append(errs, err)
		}
	}

	// Close available queue
	close(p.availableQueue)

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	log.InfoLog.Print("agent pool closed successfully")
	return nil
}

// SaveState persists pool state to storage (if configured)
func (p *AgentPool) SaveState(ctx context.Context) error {
	if p.storage == nil {
		return fmt.Errorf("storage not configured")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	instances := make([]*session.Instance, 0, len(p.agents))
	for _, agent := range p.agents {
		instances = append(instances, agent.instance)
	}

	return p.storage.SaveInstances(ctx, instances)
}

// LoadState restores pool state from storage (if configured)
func (p *AgentPool) LoadState(ctx context.Context) error {
	if p.storage == nil {
		return fmt.Errorf("storage not configured")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	instances, err := p.storage.LoadInstances(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, instance := range instances {
		agent := NewAgent(instance)
		agentID := instance.Title
		p.agents[agentID] = agent
	}

	return nil
}

// WarmPool ensures at least minPoolSize agents are available
func (p *AgentPool) WarmPool(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	currentSize := len(p.agents)

	for currentSize < p.minPoolSize {
		// Check context cancellation before spawning
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		agent, err := p.spawnAgent()
		if err != nil {
			return fmt.Errorf("failed to warm pool: %w", err)
		}

		p.mu.Lock()
		defer p.mu.Unlock()
		agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano())
		p.agents[agentID] = agent

		select {
		case p.availableQueue <- agent:
			p.idleCount.Add(1)
		case <-ctx.Done():
			return ctx.Err()
		case <-p.closedChan:
			return fmt.Errorf("pool closed during warm pool operation")
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout adding agent to queue in WarmPool")
		}

		currentSize++
	}

	return nil
}

// DrainPool removes excess idle agents if needed
func (p *AgentPool) DrainPool() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	agents := make([]*Agent, 0)
	for _, agent := range p.agents {
		if agent.GetState() == AgentStateIdle {
			agents = append(agents, agent)
		}
	}

	// Keep only minPoolSize idle agents
	toRemove := len(agents) - p.minPoolSize
	for i := 0; i < toRemove && i < len(agents); i++ {
		if err := p.killAgent(agents[i]); err != nil {
			log.ErrorLog.Printf("error draining pool: %v", err)
		}
	}

	return nil
}

// GetAgentPoolStats returns detailed statistics about the pool
func (p *AgentPool) GetAgentPoolStats() map[string]interface{} {
	metrics := p.GetMetrics()
	active, idle, total := p.GetPoolSize()

	return map[string]interface{}{
		"pool_size": map[string]int{
			"active": active,
			"idle":   idle,
			"total":  total,
			"min":    p.minPoolSize,
			"max":    p.maxPoolSize,
		},
		"metrics": map[string]interface{}{
			"total_requests":    metrics.TotalRequests,
			"total_recycles":    metrics.TotalRecycles,
			"spawned_agents":    metrics.SpawnedAgents,
			"terminated_agents": metrics.TerminatedAgents,
			"scale_direction":   metrics.ScaleDirection,
		},
		"quotas": map[string]interface{}{
			"max_memory_mb":       p.resourceQuota.MaxMemoryMB,
			"max_cpu_percent":     p.resourceQuota.MaxCPUPercent,
			"max_instance_age":    p.resourceQuota.MaxInstanceAge.String(),
			"max_recycles_per_id": p.resourceQuota.MaxRecyclesPerID,
			"requests_per_quota":  p.resourceQuota.RequestsPerQuota,
		},
	}
}
