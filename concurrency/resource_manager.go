package concurrency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ResourceType represents different types of managed resources
type ResourceType int

const (
	CPU ResourceType = iota
	Memory
	FileHandles
	Network
)

func (rt ResourceType) String() string {
	switch rt {
	case CPU:
		return "CPU"
	case Memory:
		return "Memory"
	case FileHandles:
		return "FileHandles"
	case Network:
		return "Network"
	default:
		return "Unknown"
	}
}

var (
	ErrResourceExhausted   = errors.New("resource exhausted")
	ErrQuotaExceeded       = errors.New("quota exceeded")
	ErrDeadlockDetected    = errors.New("potential deadlock detected")
	ErrInvalidCapacity     = errors.New("invalid capacity")
	ErrInvalidRate         = errors.New("invalid rate")
	ErrResourceNotAcquired = errors.New("resource not acquired")
)

// TokenBucket implements token bucket rate limiting algorithm
type TokenBucket struct {
	mu           sync.Mutex
	capacity     int64
	tokens       int64
	refillRate   int64 // tokens per second
	lastRefill   time.Time
	refillTicker *time.Ticker
	stopChan     chan struct{}
	cond         *sync.Cond
}

// NewTokenBucket creates a new token bucket with specified capacity and refill rate
func NewTokenBucket(capacity, refillRate int64) (*TokenBucket, error) {
	if capacity <= 0 || refillRate <= 0 {
		return nil, ErrInvalidRate
	}

	tb := &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
		stopChan:   make(chan struct{}),
	}
	tb.cond = sync.NewCond(&tb.mu)

	// Start refill goroutine
	tb.refillTicker = time.NewTicker(100 * time.Millisecond)
	go tb.refillLoop()

	return tb, nil
}

func (tb *TokenBucket) refillLoop() {
	for {
		select {
		case <-tb.refillTicker.C:
			tb.refill()
		case <-tb.stopChan:
			return
		}
	}
}

func (tb *TokenBucket) refill() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokensToAdd := int64(elapsed * float64(tb.refillRate))

	if tokensToAdd > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
		tb.lastRefill = now
		tb.cond.Broadcast() // Wake up waiting goroutines
	}
}

// Acquire attempts to acquire n tokens, blocking until available or context cancelled
func (tb *TokenBucket) Acquire(ctx context.Context, n int64) error {
	if n <= 0 {
		return nil
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	for tb.tokens < n {
		// Wait with context cancellation support
		done := make(chan struct{})
		go func() {
			tb.cond.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Continue loop to check if we have enough tokens
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	tb.tokens -= n
	return nil
}

// TryAcquire attempts to acquire n tokens without blocking
func (tb *TokenBucket) TryAcquire(n int64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// Release returns tokens to the bucket (for cancelled operations)
func (tb *TokenBucket) Release(n int64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens = min(tb.capacity, tb.tokens+n)
	tb.cond.Broadcast()
}

// Stop stops the token bucket refill
func (tb *TokenBucket) Stop() {
	close(tb.stopChan)
	tb.refillTicker.Stop()
}

// Available returns the number of available tokens
func (tb *TokenBucket) Available() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.tokens
}

// Semaphore implements a counting semaphore for concurrency control
type Semaphore struct {
	mu       sync.Mutex
	capacity int
	current  int
	cond     *sync.Cond
}

// NewSemaphore creates a new semaphore with specified capacity
func NewSemaphore(capacity int) (*Semaphore, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	s := &Semaphore{
		capacity: capacity,
		current:  0,
	}
	s.cond = sync.NewCond(&s.mu)
	return s, nil
}

// Acquire acquires n permits, blocking until available
func (s *Semaphore) Acquire(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for s.current+n > s.capacity {
		done := make(chan struct{})
		go func() {
			s.cond.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Continue loop
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	s.current += n
	return nil
}

// TryAcquire attempts to acquire n permits without blocking
func (s *Semaphore) TryAcquire(n int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current+n <= s.capacity {
		s.current += n
		return true
	}
	return false
}

// Release releases n permits
func (s *Semaphore) Release(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current -= n
	if s.current < 0 {
		s.current = 0
	}
	s.cond.Broadcast()
}

// Available returns available permits
func (s *Semaphore) Available() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capacity - s.current
}

// ResourcePool manages a pool of specific resources
type ResourcePool struct {
	resourceType ResourceType
	capacity     int64
	semaphore    *Semaphore
	tokenBucket  *TokenBucket
	mu           sync.RWMutex
	allocated    int64
	stats        *ResourceStats
}

// ResourceStats tracks resource usage statistics
type ResourceStats struct {
	mu            sync.RWMutex
	totalAcquired int64
	totalReleased int64
	currentUsage  int64
	peakUsage     int64
	acquisitions  int64
	failures      int64
	avgWaitTime   time.Duration
	totalWaitTime time.Duration
}

func (rs *ResourceStats) RecordAcquisition(amount int64, waitTime time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.totalAcquired += amount
	rs.currentUsage += amount
	rs.acquisitions++
	rs.totalWaitTime += waitTime

	if rs.currentUsage > rs.peakUsage {
		rs.peakUsage = rs.currentUsage
	}

	if rs.acquisitions > 0 {
		rs.avgWaitTime = rs.totalWaitTime / time.Duration(rs.acquisitions)
	}
}

func (rs *ResourceStats) RecordRelease(amount int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.totalReleased += amount
	rs.currentUsage -= amount
	if rs.currentUsage < 0 {
		rs.currentUsage = 0
	}
}

func (rs *ResourceStats) RecordFailure() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.failures++
}

func (rs *ResourceStats) GetStats() (current, peak, acquisitions, failures int64, avgWait time.Duration) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.currentUsage, rs.peakUsage, rs.acquisitions, rs.failures, rs.avgWaitTime
}

// NewResourcePool creates a new resource pool
func NewResourcePool(resourceType ResourceType, capacity int64, rateLimit int64) (*ResourcePool, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	sem, err := NewSemaphore(int(capacity))
	if err != nil {
		return nil, err
	}

	tb, err := NewTokenBucket(capacity, rateLimit)
	if err != nil {
		return nil, err
	}

	return &ResourcePool{
		resourceType: resourceType,
		capacity:     capacity,
		semaphore:    sem,
		tokenBucket:  tb,
		allocated:    0,
		stats:        &ResourceStats{},
	}, nil
}

// Acquire acquires the specified amount of resources
func (rp *ResourcePool) Acquire(ctx context.Context, amount int64) error {
	startTime := time.Now()

	// First check rate limit
	if err := rp.tokenBucket.Acquire(ctx, amount); err != nil {
		rp.stats.RecordFailure()
		return err
	}

	// Then check semaphore
	if err := rp.semaphore.Acquire(ctx, int(amount)); err != nil {
		// Return tokens on failure
		rp.tokenBucket.Release(amount)
		rp.stats.RecordFailure()
		return err
	}

	rp.mu.Lock()
	rp.allocated += amount
	rp.mu.Unlock()

	waitTime := time.Since(startTime)
	rp.stats.RecordAcquisition(amount, waitTime)

	return nil
}

// TryAcquire attempts to acquire resources without blocking
func (rp *ResourcePool) TryAcquire(amount int64) bool {
	if !rp.tokenBucket.TryAcquire(amount) {
		return false
	}

	if !rp.semaphore.TryAcquire(int(amount)) {
		rp.tokenBucket.Release(amount)
		return false
	}

	rp.mu.Lock()
	rp.allocated += amount
	rp.mu.Unlock()

	rp.stats.RecordAcquisition(amount, 0)
	return true
}

// Release releases the specified amount of resources
func (rp *ResourcePool) Release(amount int64) {
	rp.mu.Lock()
	rp.allocated -= amount
	if rp.allocated < 0 {
		rp.allocated = 0
	}
	rp.mu.Unlock()

	rp.semaphore.Release(int(amount))
	rp.stats.RecordRelease(amount)
}

// Available returns available resources
func (rp *ResourcePool) Available() int64 {
	return int64(rp.semaphore.Available())
}

// Usage returns current usage percentage (0-100)
func (rp *ResourcePool) Usage() float64 {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return float64(rp.allocated) / float64(rp.capacity) * 100.0
}

// SetCapacity dynamically adjusts pool capacity
func (rp *ResourcePool) SetCapacity(newCapacity int64) error {
	if newCapacity <= 0 {
		return ErrInvalidCapacity
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	oldCapacity := rp.capacity
	rp.capacity = newCapacity

	// Update semaphore capacity
	newSem, err := NewSemaphore(int(newCapacity))
	if err != nil {
		rp.capacity = oldCapacity
		return err
	}

	// Transfer current usage to new semaphore
	newSem.Acquire(context.Background(), int(rp.allocated))
	rp.semaphore = newSem

	return nil
}

// Stop stops the resource pool
func (rp *ResourcePool) Stop() {
	rp.tokenBucket.Stop()
}

// ResourceQuota defines per-agent resource limits
type ResourceQuota struct {
	mu     sync.RWMutex
	quotas map[string]map[ResourceType]int64 // agentID -> resourceType -> limit
	usage  map[string]map[ResourceType]int64 // agentID -> resourceType -> current usage
}

// NewResourceQuota creates a new resource quota manager
func NewResourceQuota() *ResourceQuota {
	return &ResourceQuota{
		quotas: make(map[string]map[ResourceType]int64),
		usage:  make(map[string]map[ResourceType]int64),
	}
}

// SetQuota sets a quota for an agent and resource type
func (rq *ResourceQuota) SetQuota(agentID string, resourceType ResourceType, limit int64) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if rq.quotas[agentID] == nil {
		rq.quotas[agentID] = make(map[ResourceType]int64)
	}
	if rq.usage[agentID] == nil {
		rq.usage[agentID] = make(map[ResourceType]int64)
	}

	rq.quotas[agentID][resourceType] = limit
}

// CheckQuota checks if an agent can acquire the specified amount
func (rq *ResourceQuota) CheckQuota(agentID string, resourceType ResourceType, amount int64) error {
	rq.mu.RLock()
	defer rq.mu.RUnlock()

	quota, hasQuota := rq.quotas[agentID][resourceType]
	if !hasQuota {
		return nil // No quota set, allow
	}

	currentUsage := rq.usage[agentID][resourceType]
	if currentUsage+amount > quota {
		return ErrQuotaExceeded
	}

	return nil
}

// RecordUsage records resource usage for an agent
func (rq *ResourceQuota) RecordUsage(agentID string, resourceType ResourceType, amount int64) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if rq.usage[agentID] == nil {
		rq.usage[agentID] = make(map[ResourceType]int64)
	}

	rq.usage[agentID][resourceType] += amount
}

// ReleaseUsage releases resource usage for an agent
func (rq *ResourceQuota) ReleaseUsage(agentID string, resourceType ResourceType, amount int64) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if rq.usage[agentID] != nil {
		rq.usage[agentID][resourceType] -= amount
		if rq.usage[agentID][resourceType] < 0 {
			rq.usage[agentID][resourceType] = 0
		}
	}
}

// GetUsage returns current usage for an agent
func (rq *ResourceQuota) GetUsage(agentID string, resourceType ResourceType) int64 {
	rq.mu.RLock()
	defer rq.mu.RUnlock()

	if rq.usage[agentID] == nil {
		return 0
	}
	return rq.usage[agentID][resourceType]
}

// LoadMonitor monitors system load and triggers dynamic scaling
type LoadMonitor struct {
	mu                 sync.RWMutex
	scaleUpThreshold   float64
	scaleDownThreshold float64
	checkInterval      time.Duration
	stopChan           chan struct{}
	callbacks          []func(resourceType ResourceType, currentLoad float64)
}

// NewLoadMonitor creates a new load monitor
func NewLoadMonitor(scaleUpThreshold, scaleDownThreshold float64, checkInterval time.Duration) *LoadMonitor {
	return &LoadMonitor{
		scaleUpThreshold:   scaleUpThreshold,
		scaleDownThreshold: scaleDownThreshold,
		checkInterval:      checkInterval,
		stopChan:           make(chan struct{}),
		callbacks:          make([]func(ResourceType, float64), 0),
	}
}

// RegisterCallback registers a callback for load changes
func (lm *LoadMonitor) RegisterCallback(cb func(ResourceType, float64)) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.callbacks = append(lm.callbacks, cb)
}

// Start starts monitoring with the resource manager
func (lm *LoadMonitor) Start(rm *ResourceManager) {
	ticker := time.NewTicker(lm.checkInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				lm.checkLoad(rm)
			case <-lm.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

func (lm *LoadMonitor) checkLoad(rm *ResourceManager) {
	rm.mu.RLock()
	pools := rm.pools
	rm.mu.RUnlock()

	for resourceType, pool := range pools {
		usage := pool.Usage()

		lm.mu.RLock()
		callbacks := lm.callbacks
		lm.mu.RUnlock()

		for _, cb := range callbacks {
			cb(resourceType, usage)
		}

		// Auto-scaling logic
		if usage > lm.scaleUpThreshold {
			// Scale up
			currentCapacity := pool.capacity
			newCapacity := int64(float64(currentCapacity) * 1.5)
			pool.SetCapacity(newCapacity)
		} else if usage < lm.scaleDownThreshold {
			// Scale down
			currentCapacity := pool.capacity
			newCapacity := int64(float64(currentCapacity) * 0.8)
			if newCapacity > 0 {
				pool.SetCapacity(newCapacity)
			}
		}
	}
}

// Stop stops the load monitor
func (lm *LoadMonitor) Stop() {
	close(lm.stopChan)
}

// DeadlockDetector detects potential deadlocks using wait-for graph
type DeadlockDetector struct {
	mu              sync.RWMutex
	waitForGraph    map[string]map[string]bool        // agentID -> waiting for agentID
	resourceHolders map[ResourceType]map[string]int64 // resourceType -> agentID -> amount
	enabled         bool
}

// NewDeadlockDetector creates a new deadlock detector
func NewDeadlockDetector(enabled bool) *DeadlockDetector {
	return &DeadlockDetector{
		waitForGraph:    make(map[string]map[string]bool),
		resourceHolders: make(map[ResourceType]map[string]int64),
		enabled:         enabled,
	}
}

// RecordWait records that an agent is waiting for a resource
func (dd *DeadlockDetector) RecordWait(agentID string, resourceType ResourceType) error {
	if !dd.enabled {
		return nil
	}

	dd.mu.Lock()
	defer dd.mu.Unlock()

	// Find who holds this resource
	holders := dd.resourceHolders[resourceType]
	if holders == nil {
		return nil
	}

	// Add edges in wait-for graph
	if dd.waitForGraph[agentID] == nil {
		dd.waitForGraph[agentID] = make(map[string]bool)
	}

	for holderID := range holders {
		if holderID != agentID {
			dd.waitForGraph[agentID][holderID] = true
		}
	}

	// Check for cycles (deadlock)
	if dd.hasCycle(agentID) {
		return ErrDeadlockDetected
	}

	return nil
}

// RecordAcquire records that an agent acquired a resource
func (dd *DeadlockDetector) RecordAcquire(agentID string, resourceType ResourceType, amount int64) {
	if !dd.enabled {
		return
	}

	dd.mu.Lock()
	defer dd.mu.Unlock()

	if dd.resourceHolders[resourceType] == nil {
		dd.resourceHolders[resourceType] = make(map[string]int64)
	}

	dd.resourceHolders[resourceType][agentID] += amount

	// Remove from wait-for graph
	delete(dd.waitForGraph, agentID)
}

// RecordRelease records that an agent released a resource
func (dd *DeadlockDetector) RecordRelease(agentID string, resourceType ResourceType, amount int64) {
	if !dd.enabled {
		return
	}

	dd.mu.Lock()
	defer dd.mu.Unlock()

	if dd.resourceHolders[resourceType] != nil {
		dd.resourceHolders[resourceType][agentID] -= amount
		if dd.resourceHolders[resourceType][agentID] <= 0 {
			delete(dd.resourceHolders[resourceType], agentID)
		}
	}
}

// hasCycle detects cycles in wait-for graph using DFS
func (dd *DeadlockDetector) hasCycle(startNode string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	return dd.hasCycleDFS(startNode, visited, recStack)
}

func (dd *DeadlockDetector) hasCycleDFS(node string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	if neighbors := dd.waitForGraph[node]; neighbors != nil {
		for neighbor := range neighbors {
			if !visited[neighbor] {
				if dd.hasCycleDFS(neighbor, visited, recStack) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}
	}

	recStack[node] = false
	return false
}

// ResourceManager orchestrates all resource management
type ResourceManager struct {
	mu                sync.RWMutex
	pools             map[ResourceType]*ResourcePool
	quota             *ResourceQuota
	loadMonitor       *LoadMonitor
	deadlockDetector  *DeadlockDetector
	acquisitionRecord map[string]map[ResourceType]int64 // agentID -> resourceType -> amount
}

// Config holds resource manager configuration
type Config struct {
	CPUCapacity             int64
	MemoryCapacity          int64
	FileHandlesCapacity     int64
	NetworkCapacity         int64
	RateLimit               int64
	EnableDeadlockDetection bool
	ScaleUpThreshold        float64
	ScaleDownThreshold      float64
	MonitorInterval         time.Duration
}

// DefaultResourceManagerConfig returns default configuration
func DefaultResourceManagerConfig() *Config {
	return &Config{
		CPUCapacity:             100,
		MemoryCapacity:          1024 * 1024 * 1024, // 1GB
		FileHandlesCapacity:     1000,
		NetworkCapacity:         100,
		RateLimit:               50, // 50 per second
		EnableDeadlockDetection: true,
		ScaleUpThreshold:        80.0, // 80% usage
		ScaleDownThreshold:      20.0, // 20% usage
		MonitorInterval:         5 * time.Second,
	}
}

// NewResourceManager creates a new resource manager
func NewResourceManager(config *Config) (*ResourceManager, error) {
	if config == nil {
		config = DefaultResourceManagerConfig()
	}

	pools := make(map[ResourceType]*ResourcePool)

	// Create resource pools
	cpuPool, err := NewResourcePool(CPU, config.CPUCapacity, config.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU pool: %w", err)
	}
	pools[CPU] = cpuPool

	memPool, err := NewResourcePool(Memory, config.MemoryCapacity, config.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create Memory pool: %w", err)
	}
	pools[Memory] = memPool

	filePool, err := NewResourcePool(FileHandles, config.FileHandlesCapacity, config.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create FileHandles pool: %w", err)
	}
	pools[FileHandles] = filePool

	netPool, err := NewResourcePool(Network, config.NetworkCapacity, config.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create Network pool: %w", err)
	}
	pools[Network] = netPool

	loadMonitor := NewLoadMonitor(
		config.ScaleUpThreshold,
		config.ScaleDownThreshold,
		config.MonitorInterval,
	)

	deadlockDetector := NewDeadlockDetector(config.EnableDeadlockDetection)

	rm := &ResourceManager{
		pools:             pools,
		quota:             NewResourceQuota(),
		loadMonitor:       loadMonitor,
		deadlockDetector:  deadlockDetector,
		acquisitionRecord: make(map[string]map[ResourceType]int64),
	}

	// Start load monitoring
	loadMonitor.Start(rm)

	return rm, nil
}

// Acquire acquires resources for an agent
func (rm *ResourceManager) Acquire(ctx context.Context, agentID string, resourceType ResourceType, amount int64) error {
	// Check quota first
	if err := rm.quota.CheckQuota(agentID, resourceType, amount); err != nil {
		return err
	}

	// Check for potential deadlock
	if err := rm.deadlockDetector.RecordWait(agentID, resourceType); err != nil {
		return err
	}

	// Get the appropriate pool
	rm.mu.RLock()
	pool, exists := rm.pools[resourceType]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	// Acquire from pool
	if err := pool.Acquire(ctx, amount); err != nil {
		return err
	}

	// Record acquisition
	rm.mu.Lock()
	if rm.acquisitionRecord[agentID] == nil {
		rm.acquisitionRecord[agentID] = make(map[ResourceType]int64)
	}
	rm.acquisitionRecord[agentID][resourceType] += amount
	rm.mu.Unlock()

	rm.quota.RecordUsage(agentID, resourceType, amount)
	rm.deadlockDetector.RecordAcquire(agentID, resourceType, amount)

	return nil
}

// TryAcquire attempts to acquire resources without blocking
func (rm *ResourceManager) TryAcquire(agentID string, resourceType ResourceType, amount int64) (bool, error) {
	// Check quota first
	if err := rm.quota.CheckQuota(agentID, resourceType, amount); err != nil {
		return false, err
	}

	rm.mu.RLock()
	pool, exists := rm.pools[resourceType]
	rm.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("resource type %s not found", resourceType)
	}

	if pool.TryAcquire(amount) {
		rm.mu.Lock()
		if rm.acquisitionRecord[agentID] == nil {
			rm.acquisitionRecord[agentID] = make(map[ResourceType]int64)
		}
		rm.acquisitionRecord[agentID][resourceType] += amount
		rm.mu.Unlock()

		rm.quota.RecordUsage(agentID, resourceType, amount)
		rm.deadlockDetector.RecordAcquire(agentID, resourceType, amount)
		return true, nil
	}

	return false, nil
}

// Release releases resources for an agent
func (rm *ResourceManager) Release(agentID string, resourceType ResourceType, amount int64) error {
	rm.mu.Lock()
	if rm.acquisitionRecord[agentID] == nil || rm.acquisitionRecord[agentID][resourceType] < amount {
		rm.mu.Unlock()
		return ErrResourceNotAcquired
	}
	rm.acquisitionRecord[agentID][resourceType] -= amount
	rm.mu.Unlock()

	rm.mu.RLock()
	pool, exists := rm.pools[resourceType]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	pool.Release(amount)
	rm.quota.ReleaseUsage(agentID, resourceType, amount)
	rm.deadlockDetector.RecordRelease(agentID, resourceType, amount)

	return nil
}

// SetQuota sets a quota for an agent
func (rm *ResourceManager) SetQuota(agentID string, resourceType ResourceType, limit int64) {
	rm.quota.SetQuota(agentID, resourceType, limit)
}

// GetUsage returns current usage for an agent
func (rm *ResourceManager) GetUsage(agentID string, resourceType ResourceType) int64 {
	return rm.quota.GetUsage(agentID, resourceType)
}

// GetPoolUsage returns pool usage percentage
func (rm *ResourceManager) GetPoolUsage(resourceType ResourceType) (float64, error) {
	rm.mu.RLock()
	pool, exists := rm.pools[resourceType]
	rm.mu.RUnlock()

	if !exists {
		return 0, fmt.Errorf("resource type %s not found", resourceType)
	}

	return pool.Usage(), nil
}

// GetPoolStats returns detailed pool statistics
func (rm *ResourceManager) GetPoolStats(resourceType ResourceType) (current, peak, acquisitions, failures int64, avgWait time.Duration, err error) {
	rm.mu.RLock()
	pool, exists := rm.pools[resourceType]
	rm.mu.RUnlock()

	if !exists {
		return 0, 0, 0, 0, 0, fmt.Errorf("resource type %s not found", resourceType)
	}

	current, peak, acquisitions, failures, avgWait = pool.stats.GetStats()
	return current, peak, acquisitions, failures, avgWait, nil
}

// SetPoolCapacity dynamically adjusts pool capacity
func (rm *ResourceManager) SetPoolCapacity(resourceType ResourceType, newCapacity int64) error {
	rm.mu.RLock()
	pool, exists := rm.pools[resourceType]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	return pool.SetCapacity(newCapacity)
}

// RegisterLoadCallback registers a callback for load changes
func (rm *ResourceManager) RegisterLoadCallback(cb func(ResourceType, float64)) {
	rm.loadMonitor.RegisterCallback(cb)
}

// Stop stops the resource manager
func (rm *ResourceManager) Stop() {
	rm.loadMonitor.Stop()

	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, pool := range rm.pools {
		pool.Stop()
	}
}

// Helper function for min
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
