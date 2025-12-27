package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ==================== Core Types ====================

// Record represents an immutable knowledge record
type Record struct {
	ID        string                 `json:"id"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Event represents a deterministic event for replay
type Event struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Receipt represents proof of execution
type Receipt struct {
	ExecutionID    string            `json:"execution_id"`
	AgentID        string            `json:"agent_id"`
	Timestamp      int64             `json:"timestamp"`
	ToolchainVer   string            `json:"toolchain_ver"`
	InputHash      string            `json:"input_hash"`
	OutputHash     string            `json:"output_hash"`
	ProofArtifacts map[string]string `json:"proof_artifacts"`
	ReplayScript   string            `json:"replay_script"`
	CompositionOp  string            `json:"composition_op"`
	ConflictPolicy string            `json:"conflict_policy"`
}

// Task represents a unit of work
type Task struct {
	ID        string                 `json:"id"`
	Priority  int                    `json:"priority"`
	Data      map[string]interface{} `json:"data"`
	Predicate string                 `json:"predicate"`
}

// Delta represents a change-set
type Delta struct {
	ID       string   `json:"id"`
	Files    []string `json:"files"`
	Receipt  *Receipt `json:"receipt"`
	CheckSum string   `json:"checksum"`
}

// ConflictReport represents reconciliation conflicts
type ConflictReport struct {
	HasConflicts bool     `json:"has_conflicts"`
	Conflicts    []string `json:"conflicts"`
}

// Allocation represents resource allocation
type Allocation struct {
	AgentID   string `json:"agent_id"`
	Resources int    `json:"resources"`
}

// ==================== Agent 1: KnowledgeStore ====================

// KnowledgeStore provides immutable append-log semantics
type KnowledgeStore interface {
	Append(ctx context.Context, record Record) (hash string, err error)
	Snapshot(ctx context.Context) (hash string, data []byte, err error)
	Verify(ctx context.Context, snapshotHash string) (valid bool, err error)
	Replay(ctx context.Context, events []Event) (hash string, err error)
}

// SimpleKnowledgeStore is a stub implementation
type SimpleKnowledgeStore struct {
	mu      sync.RWMutex
	records []Record
}

func NewSimpleKnowledgeStore() *SimpleKnowledgeStore {
	return &SimpleKnowledgeStore{
		records: make([]Record, 0),
	}
}

func (s *SimpleKnowledgeStore) Append(ctx context.Context, record Record) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Idempotent check
	for _, r := range s.records {
		if r.ID == record.ID {
			return s.hashRecord(record), nil // Already exists
		}
	}

	s.records = append(s.records, record)
	return s.hashRecord(record), nil
}

func (s *SimpleKnowledgeStore) Snapshot(ctx context.Context) (string, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.Marshal(s.records)
	if err != nil {
		return "", nil, err
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	return hash, data, nil
}

func (s *SimpleKnowledgeStore) Verify(ctx context.Context, snapshotHash string) (bool, error) {
	hash, _, err := s.Snapshot(ctx)
	if err != nil {
		return false, err
	}
	return hash == snapshotHash, nil
}

func (s *SimpleKnowledgeStore) Replay(ctx context.Context, events []Event) (string, error) {
	// Deterministic replay
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = make([]Record, 0)
	for _, event := range events {
		record := Record{
			ID:        event.Data["id"].(string),
			Timestamp: event.Timestamp,
			Data:      event.Data,
		}
		s.records = append(s.records, record)
	}

	data, _ := json.Marshal(s.records)
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (s *SimpleKnowledgeStore) hashRecord(r Record) string {
	data, _ := json.Marshal(r)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// ==================== Agent 2: Receipt Chain ====================

// ReceiptChain manages receipt creation and verification
type ReceiptChain interface {
	CreateReceipt(executionID, agentID string, inputHash, outputHash, replayScript string) *Receipt
	VerifyReceipt(receipt *Receipt) bool
	ChainReceipts(receipts []*Receipt) (*Receipt, error)
}

// SimpleReceiptChain is a stub implementation
type SimpleReceiptChain struct{}

func NewSimpleReceiptChain() *SimpleReceiptChain {
	return &SimpleReceiptChain{}
}

func (r *SimpleReceiptChain) CreateReceipt(executionID, agentID, inputHash, outputHash, replayScript string) *Receipt {
	return &Receipt{
		ExecutionID:    executionID,
		AgentID:        agentID,
		Timestamp:      time.Now().UnixNano(),
		ToolchainVer:   "go1.21",
		InputHash:      inputHash,
		OutputHash:     outputHash,
		ProofArtifacts: make(map[string]string),
		ReplayScript:   replayScript,
		CompositionOp:  "append",
		ConflictPolicy: "fail_fast",
	}
}

func (r *SimpleReceiptChain) VerifyReceipt(receipt *Receipt) bool {
	// Basic validation
	return receipt != nil &&
		receipt.ExecutionID != "" &&
		receipt.AgentID != "" &&
		receipt.InputHash != "" &&
		receipt.OutputHash != ""
}

func (r *SimpleReceiptChain) ChainReceipts(receipts []*Receipt) (*Receipt, error) {
	if len(receipts) == 0 {
		return nil, fmt.Errorf("no receipts to chain")
	}

	// Create global receipt from all sub-receipts
	inputHashes := ""
	outputHashes := ""
	for _, rec := range receipts {
		inputHashes += rec.InputHash
		outputHashes += rec.OutputHash
	}

	globalInputHash := fmt.Sprintf("%x", sha256.Sum256([]byte(inputHashes)))
	globalOutputHash := fmt.Sprintf("%x", sha256.Sum256([]byte(outputHashes)))

	return &Receipt{
		ExecutionID:    "global-" + receipts[0].ExecutionID,
		AgentID:        "agent-0-reconciler",
		Timestamp:      time.Now().UnixNano(),
		ToolchainVer:   "go1.21",
		InputHash:      globalInputHash,
		OutputHash:     globalOutputHash,
		ProofArtifacts: map[string]string{"sub_receipts": fmt.Sprintf("%d", len(receipts))},
		ReplayScript:   "# Global replay script",
		CompositionOp:  "merge",
		ConflictPolicy: "fail_fast",
	}, nil
}

// ==================== Agent 4: Resource Allocator ====================

// ResourceAllocator manages deterministic capacity allocation
type ResourceAllocator interface {
	AllocateResources(agentCount, resourceBudget int) []Allocation
	RoundRobinSchedule(agents []string, tasks []*Task) map[string][]*Task
}

// SimpleResourceAllocator is a stub implementation
type SimpleResourceAllocator struct{}

func NewSimpleResourceAllocator() *SimpleResourceAllocator {
	return &SimpleResourceAllocator{}
}

func (a *SimpleResourceAllocator) AllocateResources(agentCount, resourceBudget int) []Allocation {
	allocations := make([]Allocation, agentCount)
	perAgent := resourceBudget / agentCount

	for i := 0; i < agentCount; i++ {
		allocations[i] = Allocation{
			AgentID:   fmt.Sprintf("agent-%d", i),
			Resources: perAgent,
		}
	}

	return allocations
}

func (a *SimpleResourceAllocator) RoundRobinSchedule(agents []string, tasks []*Task) map[string][]*Task {
	schedule := make(map[string][]*Task)

	for i, task := range tasks {
		agentID := agents[i%len(agents)]
		schedule[agentID] = append(schedule[agentID], task)
	}

	return schedule
}

// ==================== Agent 6: Task Router ====================

// TaskRouter provides deterministic routing
type TaskRouter interface {
	Route(task *Task, predicates map[string]bool) (string, error)
	EvaluateTaskGraph(tasks []*Task) ([]*Task, error)
}

// SimpleTaskRouter is a stub implementation
type SimpleTaskRouter struct{}

func NewSimpleTaskRouter() *SimpleTaskRouter {
	return &SimpleTaskRouter{}
}

func (t *SimpleTaskRouter) Route(task *Task, predicates map[string]bool) (string, error) {
	// Deterministic routing based on task priority
	if task.Priority > 5 {
		return "high-priority-agent", nil
	}
	return "normal-priority-agent", nil
}

func (t *SimpleTaskRouter) EvaluateTaskGraph(tasks []*Task) ([]*Task, error) {
	// Simple topological sort by priority
	sorted := make([]*Task, len(tasks))
	copy(sorted, tasks)

	// Sort by priority (deterministic)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Priority < sorted[j].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted, nil
}

// ==================== Agent 0: Reconciler ====================

// Reconciler validates and merges deltas
type Reconciler interface {
	Reconcile(ctx context.Context, deltas []*Delta) (*Delta, *ConflictReport, error)
	ValidateComposition(delta1, delta2 *Delta) (bool, string)
}

// SimpleReconciler is a stub implementation
type SimpleReconciler struct{}

func NewSimpleReconciler() *SimpleReconciler {
	return &SimpleReconciler{}
}

func (r *SimpleReconciler) Reconcile(ctx context.Context, deltas []*Delta) (*Delta, *ConflictReport, error) {
	report := &ConflictReport{
		HasConflicts: false,
		Conflicts:    make([]string, 0),
	}

	// Check for file conflicts
	fileMap := make(map[string]bool)
	for _, delta := range deltas {
		for _, file := range delta.Files {
			if fileMap[file] {
				report.HasConflicts = true
				report.Conflicts = append(report.Conflicts, fmt.Sprintf("file conflict: %s", file))
			}
			fileMap[file] = true
		}
	}

	if report.HasConflicts {
		return nil, report, nil
	}

	// Create merged delta
	allFiles := make([]string, 0)
	for file := range fileMap {
		allFiles = append(allFiles, file)
	}

	merged := &Delta{
		ID:       "merged-delta",
		Files:    allFiles,
		CheckSum: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%v", allFiles)))),
	}

	return merged, report, nil
}

func (r *SimpleReconciler) ValidateComposition(delta1, delta2 *Delta) (bool, string) {
	// Check for file overlap
	for _, f1 := range delta1.Files {
		for _, f2 := range delta2.Files {
			if f1 == f2 {
				return false, fmt.Sprintf("file overlap: %s", f1)
			}
		}
	}
	return true, ""
}
