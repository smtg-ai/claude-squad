// Package aider provides a comprehensive simulation of Aider functionality
// for hyper-advanced testing with 10 concurrent Claude Code web VM agents.
//
// This package simulates Aider's behavior without requiring an actual Aider
// installation, enabling rapid testing and validation of integration patterns.
package aider

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// SimulationMode represents the operating mode of the Aider simulator
type SimulationMode string

const (
	// RealisticMode simulates realistic response times and behavior
	RealisticMode SimulationMode = "realistic"
	// FastMode runs with minimal delays for rapid testing
	FastMode SimulationMode = "fast"
	// StressMode simulates high-load conditions
	StressMode SimulationMode = "stress"
	// ErrorMode injects random errors for resilience testing
	ErrorMode SimulationMode = "error"
)

// SimulatorConfig configures the Aider simulator behavior
type SimulatorConfig struct {
	// Mode determines the simulation behavior
	Mode SimulationMode
	// BaseLatency is the base response time in milliseconds
	BaseLatency int64
	// ErrorRate is the probability of random errors (0-1)
	ErrorRate float64
	// MaxConcurrentSessions limits concurrent sessions
	MaxConcurrentSessions int
	// EnableMetrics enables detailed metrics collection
	EnableMetrics bool
	// SessionTimeout is the max time a session can run
	SessionTimeout time.Duration
}

// DefaultSimulatorConfig returns a sensible default configuration
func DefaultSimulatorConfig() *SimulatorConfig {
	return &SimulatorConfig{
		Mode:                  RealisticMode,
		BaseLatency:           100,
		ErrorRate:             0.0,
		MaxConcurrentSessions: 50,
		EnableMetrics:         true,
		SessionTimeout:        30 * time.Minute,
	}
}

// AiderSimulator simulates Aider's behavior for testing
type AiderSimulator struct {
	mu                  sync.RWMutex
	config              *SimulatorConfig
	sessions            map[string]*SimulatedSession
	sessionCount        int32
	commandCount        int64
	errorCount          int64
	totalLatencyMs      int64
	startTime           time.Time
	fileChanges         map[string]*FileChange
	gitCommits          []string
	metrics             *SimulatorMetrics
	activeAgents        int32
	concurrentTestCount int32
}

// SimulatedSession represents an active Aider session
type SimulatedSession struct {
	ID           string
	Mode         string
	Model        string
	StartTime    time.Time
	CommandCount int
	Status       string
	LastActivity time.Time
	Context      []string
	mu           sync.RWMutex
}

// FileChange represents a simulated file modification
type FileChange struct {
	Path          string
	ChangeType    string // "created", "modified", "deleted"
	LinesAdded    int
	LinesRemoved  int
	Timestamp     time.Time
	SessionID     string
}

// SimulatorMetrics tracks detailed simulator performance
type SimulatorMetrics struct {
	TotalSessions       int64
	ActiveSessions      int32
	TotalCommands       int64
	TotalErrors         int64
	AverageLatencyMs    int64
	ConcurrentAgents    int32
	TestScenarios       int32
	ValidationsPassed   int64
	ValidationsFailed   int64
	Uptime              time.Duration
	SessionCreateRate   float64
	CommandThroughput   float64
	ErrorRate           float64
}

// NewAiderSimulator creates a new Aider simulator instance
func NewAiderSimulator(config *SimulatorConfig) (*AiderSimulator, error) {
	if config == nil {
		config = DefaultSimulatorConfig()
	}

	return &AiderSimulator{
		config:      config,
		sessions:    make(map[string]*SimulatedSession),
		fileChanges: make(map[string]*FileChange),
		gitCommits:  []string{},
		metrics:     &SimulatorMetrics{},
		startTime:   time.Now(),
	}, nil
}

// CreateSession simulates creating a new Aider session
func (as *AiderSimulator) CreateSession(ctx context.Context, mode, model string) (*SimulatedSession, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Check concurrent session limit
	if len(as.sessions) >= as.config.MaxConcurrentSessions {
		atomic.AddInt64(&as.errorCount, 1)
		return nil, fmt.Errorf("maximum concurrent sessions (%d) reached", as.config.MaxConcurrentSessions)
	}

	// Simulate random errors if in error mode
	if as.config.Mode == ErrorMode && rand.Float64() < as.config.ErrorRate {
		atomic.AddInt64(&as.errorCount, 1)
		return nil, fmt.Errorf("simulated error: session creation failed")
	}

	// Simulate latency
	as.simulateLatency()

	// Create session
	sessionID := fmt.Sprintf("session-%d-%d", time.Now().Unix(), atomic.AddInt32(&as.sessionCount, 1))
	session := &SimulatedSession{
		ID:           sessionID,
		Mode:         mode,
		Model:        model,
		StartTime:    time.Now(),
		CommandCount: 0,
		Status:       "active",
		LastActivity: time.Now(),
		Context:      []string{},
	}

	as.sessions[sessionID] = session
	atomic.AddInt64(&as.metrics.TotalSessions, 1)
	atomic.AddInt32(&as.metrics.ActiveSessions, 1)

	return session, nil
}

// ExecuteCommand simulates executing a command in an Aider session
func (as *AiderSimulator) ExecuteCommand(ctx context.Context, sessionID, command string) (*CommandResult, error) {
	as.mu.RLock()
	session, exists := as.sessions[sessionID]
	as.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&as.errorCount, 1)
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Simulate random errors
	if as.config.Mode == ErrorMode && rand.Float64() < as.config.ErrorRate {
		atomic.AddInt64(&as.errorCount, 1)
		return nil, fmt.Errorf("simulated error: command execution failed")
	}

	// Simulate latency
	latency := as.simulateLatency()

	// Update session
	session.mu.Lock()
	session.CommandCount++
	session.LastActivity = time.Now()
	session.mu.Unlock()

	atomic.AddInt64(&as.commandCount, 1)
	atomic.AddInt64(&as.metrics.TotalCommands, 1)

	// Simulate file changes
	changes := as.simulateFileChanges(sessionID, command)

	result := &CommandResult{
		SessionID:     sessionID,
		Command:       command,
		Success:       true,
		LatencyMs:     latency,
		FilesModified: len(changes),
		Timestamp:     time.Now(),
		Changes:       changes,
	}

	return result, nil
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	SessionID     string
	Command       string
	Success       bool
	LatencyMs     int64
	FilesModified int
	Timestamp     time.Time
	Changes       []*FileChange
}

// simulateLatency simulates response time based on mode
func (as *AiderSimulator) simulateLatency() int64 {
	var latency int64

	switch as.config.Mode {
	case FastMode:
		latency = 1 + rand.Int63n(10)
	case RealisticMode:
		latency = as.config.BaseLatency + rand.Int63n(50)
	case StressMode:
		latency = as.config.BaseLatency*2 + rand.Int63n(200)
	case ErrorMode:
		latency = as.config.BaseLatency + rand.Int63n(100)
	default:
		latency = as.config.BaseLatency
	}

	// Sleep to simulate actual latency
	time.Sleep(time.Duration(latency) * time.Millisecond)

	atomic.AddInt64(&as.totalLatencyMs, latency)
	return latency
}

// simulateFileChanges simulates file modifications
func (as *AiderSimulator) simulateFileChanges(sessionID, command string) []*FileChange {
	numChanges := 1 + rand.Intn(3)
	changes := make([]*FileChange, numChanges)

	for i := 0; i < numChanges; i++ {
		change := &FileChange{
			Path:         fmt.Sprintf("file-%d.go", rand.Intn(100)),
			ChangeType:   "modified",
			LinesAdded:   rand.Intn(50) + 1,
			LinesRemoved: rand.Intn(20),
			Timestamp:    time.Now(),
			SessionID:    sessionID,
		}
		changes[i] = change

		as.mu.Lock()
		as.fileChanges[change.Path] = change
		as.mu.Unlock()
	}

	return changes
}

// CloseSession simulates closing an Aider session
func (as *AiderSimulator) CloseSession(ctx context.Context, sessionID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	session, exists := as.sessions[sessionID]
	if !exists {
		atomic.AddInt64(&as.errorCount, 1)
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.Status = "closed"
	session.mu.Unlock()

	delete(as.sessions, sessionID)
	atomic.AddInt32(&as.metrics.ActiveSessions, -1)

	return nil
}

// GetMetrics returns current simulator metrics
func (as *AiderSimulator) GetMetrics() *SimulatorMetrics {
	as.mu.RLock()
	defer as.mu.RUnlock()

	avgLatency := int64(0)
	if as.commandCount > 0 {
		avgLatency = atomic.LoadInt64(&as.totalLatencyMs) / atomic.LoadInt64(&as.commandCount)
	}

	uptime := time.Since(as.startTime)
	sessionRate := float64(atomic.LoadInt64(&as.metrics.TotalSessions)) / uptime.Seconds()
	commandRate := float64(atomic.LoadInt64(&as.commandCount)) / uptime.Seconds()
	errorRate := 0.0
	if as.commandCount > 0 {
		errorRate = float64(atomic.LoadInt64(&as.errorCount)) / float64(atomic.LoadInt64(&as.commandCount))
	}

	return &SimulatorMetrics{
		TotalSessions:      atomic.LoadInt64(&as.metrics.TotalSessions),
		ActiveSessions:     atomic.LoadInt32(&as.metrics.ActiveSessions),
		TotalCommands:      atomic.LoadInt64(&as.commandCount),
		TotalErrors:        atomic.LoadInt64(&as.errorCount),
		AverageLatencyMs:   avgLatency,
		ConcurrentAgents:   atomic.LoadInt32(&as.activeAgents),
		TestScenarios:      atomic.LoadInt32(&as.concurrentTestCount),
		ValidationsPassed:  atomic.LoadInt64(&as.metrics.ValidationsPassed),
		ValidationsFailed:  atomic.LoadInt64(&as.metrics.ValidationsFailed),
		Uptime:             uptime,
		SessionCreateRate:  sessionRate,
		CommandThroughput:  commandRate,
		ErrorRate:          errorRate,
	}
}

// RegisterAgent registers a testing agent
func (as *AiderSimulator) RegisterAgent(agentID string) {
	atomic.AddInt32(&as.activeAgents, 1)
}

// UnregisterAgent unregisters a testing agent
func (as *AiderSimulator) UnregisterAgent(agentID string) {
	atomic.AddInt32(&as.activeAgents, -1)
}

// RecordValidation records a validation result
func (as *AiderSimulator) RecordValidation(passed bool) {
	if passed {
		atomic.AddInt64(&as.metrics.ValidationsPassed, 1)
	} else {
		atomic.AddInt64(&as.metrics.ValidationsFailed, 1)
	}
}

// GetActiveSessions returns all active sessions
func (as *AiderSimulator) GetActiveSessions() []*SimulatedSession {
	as.mu.RLock()
	defer as.mu.RUnlock()

	sessions := make([]*SimulatedSession, 0, len(as.sessions))
	for _, session := range as.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// Reset resets the simulator state (for testing)
func (as *AiderSimulator) Reset() {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.sessions = make(map[string]*SimulatedSession)
	as.fileChanges = make(map[string]*FileChange)
	as.gitCommits = []string{}
	atomic.StoreInt32(&as.sessionCount, 0)
	atomic.StoreInt64(&as.commandCount, 0)
	atomic.StoreInt64(&as.errorCount, 0)
	atomic.StoreInt64(&as.totalLatencyMs, 0)
	atomic.StoreInt32(&as.activeAgents, 0)
	atomic.StoreInt32(&as.concurrentTestCount, 0)
	as.startTime = time.Now()
	as.metrics = &SimulatorMetrics{}
}

// StressTest runs a stress test with concurrent operations
func (as *AiderSimulator) StressTest(ctx context.Context, numSessions, numCommands int) (*StressTestResult, error) {
	var wg sync.WaitGroup
	errorCh := make(chan error, numSessions*numCommands)
	successCount := int32(0)
	failureCount := int32(0)

	startTime := time.Now()

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(sessionNum int) {
			defer wg.Done()

			session, err := as.CreateSession(ctx, "code", fmt.Sprintf("model-%d", sessionNum%10))
			if err != nil {
				errorCh <- err
				atomic.AddInt32(&failureCount, 1)
				return
			}

			for j := 0; j < numCommands; j++ {
				_, err := as.ExecuteCommand(ctx, session.ID, fmt.Sprintf("command-%d", j))
				if err != nil {
					errorCh <- err
					atomic.AddInt32(&failureCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}

			_ = as.CloseSession(ctx, session.ID)
		}(i)
	}

	wg.Wait()
	close(errorCh)

	duration := time.Since(startTime)

	errors := make([]string, 0)
	for err := range errorCh {
		errors = append(errors, err.Error())
	}

	return &StressTestResult{
		TotalOperations: numSessions * numCommands,
		SuccessCount:    int(successCount),
		FailureCount:    int(failureCount),
		Duration:        duration,
		Throughput:      float64(successCount) / duration.Seconds(),
		Errors:          errors,
	}, nil
}

// StressTestResult contains stress test results
type StressTestResult struct {
	TotalOperations int
	SuccessCount    int
	FailureCount    int
	Duration        time.Duration
	Throughput      float64
	Errors          []string
}
