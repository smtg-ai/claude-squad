// Package agent1 implements the KnowledgeStore interface with append-log semantics
// and deterministic hash-stable snapshots for the KGC knowledge substrate.
package agent1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Errors
var (
	ErrDuplicateID   = errors.New("record with this ID already exists")
	ErrInvalidRecord = errors.New("invalid record: ID and Content are required")
	ErrReplayFailed  = errors.New("replay failed during event processing")
)

// Record represents an immutable knowledge record
type Record struct {
	ID        string            `json:"id"`
	Timestamp int64             `json:"timestamp"`
	Content   []byte            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Event represents an append operation for replay
type Event struct {
	Type      string `json:"type"`
	Record    Record `json:"record"`
	Timestamp int64  `json:"timestamp"`
}

// SnapshotData is the canonical serialization format for snapshots
type SnapshotData struct {
	Records  []Record          `json:"records"`
	Metadata map[string]string `json:"metadata"`
	Version  int64             `json:"version"`
}

// KnowledgeStore provides immutable append-log semantics with hash-stable snapshots
type KnowledgeStore struct {
	mu       sync.RWMutex      // Concurrency control
	records  []Record          // Append-only log
	metadata map[string]string // Store metadata
	version  int64             // Monotonic version counter
	index    map[string]int    // ID → position for O(1) duplicate detection
}

// NewKnowledgeStore creates a new empty knowledge store
func NewKnowledgeStore() *KnowledgeStore {
	return &KnowledgeStore{
		records:  make([]Record, 0),
		metadata: make(map[string]string),
		version:  0,
		index:    make(map[string]int),
	}
}

// Append adds a record to the append-log with idempotence guarantee
// Returns: hash of the appended record, error if duplicate ID detected
// Invariant: Append(x) ⊕ Append(x) = Append(x) (idempotent via duplicate detection)
func (ks *KnowledgeStore) Append(ctx context.Context, record Record) (string, error) {
	// Validate record
	if record.ID == "" || len(record.Content) == 0 {
		return "", ErrInvalidRecord
	}

	ks.mu.Lock()
	defer ks.mu.Unlock()

	// Check for duplicate ID (idempotence)
	if _, exists := ks.index[record.ID]; exists {
		return "", ErrDuplicateID
	}

	// Append to log (monotonic operation)
	position := len(ks.records)
	ks.records = append(ks.records, record)
	ks.index[record.ID] = position
	ks.version++

	// Compute record hash
	hash := computeRecordHash(record)
	return hash, nil
}

// Snapshot produces a deterministic, hash-stable snapshot of the current state
// Returns: snapshot hash, serialized data, error
// Invariant: ∀ O. Snapshot(O) = Snapshot(O) (deterministic)
func (ks *KnowledgeStore) Snapshot(ctx context.Context) (string, []byte, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	// Create snapshot data structure
	snapshot := SnapshotData{
		Records:  make([]Record, len(ks.records)),
		Metadata: make(map[string]string),
		Version:  ks.version,
	}

	// Deep copy records
	copy(snapshot.Records, ks.records)

	// Deep copy metadata
	for k, v := range ks.metadata {
		snapshot.Metadata[k] = v
	}

	// Sort records by timestamp for deterministic ordering
	sort.Slice(snapshot.Records, func(i, j int) bool {
		if snapshot.Records[i].Timestamp == snapshot.Records[j].Timestamp {
			// If timestamps are equal, sort by ID for total determinism
			return snapshot.Records[i].ID < snapshot.Records[j].ID
		}
		return snapshot.Records[i].Timestamp < snapshot.Records[j].Timestamp
	})

	// Canonicalize: serialize to JSON with sorted keys (no pretty-print)
	data, err := canonicalJSON(snapshot)
	if err != nil {
		return "", nil, fmt.Errorf("failed to canonicalize snapshot: %w", err)
	}

	// Compute SHA256 hash
	hash := computeSHA256(data)

	return hash, data, nil
}

// Verify checks if the current snapshot hash matches the provided hash
// Returns: true if valid, false if tampered, error on failure
// Invariant: O ≠ O' ⟹ hash(O) ≠ hash(O')
func (ks *KnowledgeStore) Verify(ctx context.Context, snapshotHash string) (bool, error) {
	currentHash, _, err := ks.Snapshot(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to generate current snapshot: %w", err)
	}

	return currentHash == snapshotHash, nil
}

// Replay reconstructs the knowledge store from an event log
// Returns: final snapshot hash, error if replay fails
// Invariant: Replay(events) produces deterministic output
func (ks *KnowledgeStore) Replay(ctx context.Context, events []Event) (string, error) {
	// Create a new empty store for replay
	replayStore := NewKnowledgeStore()

	// Process each event in order
	for i, event := range events {
		if event.Type != "append" {
			return "", fmt.Errorf("unknown event type at index %d: %s", i, event.Type)
		}

		// Append the record from the event
		_, err := replayStore.Append(ctx, event.Record)
		if err != nil {
			// If it's a duplicate, that's okay during replay (idempotent)
			if errors.Is(err, ErrDuplicateID) {
				continue
			}
			return "", fmt.Errorf("%w: event %d: %v", ErrReplayFailed, i, err)
		}
	}

	// Get final snapshot hash
	hash, _, err := replayStore.Snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to snapshot after replay: %w", err)
	}

	// Replace current store with replayed store
	ks.mu.Lock()
	ks.records = replayStore.records
	ks.metadata = replayStore.metadata
	ks.version = replayStore.version
	ks.index = replayStore.index
	ks.mu.Unlock()

	return hash, nil
}

// GetAllRecords returns a copy of all records (for testing/debugging)
func (ks *KnowledgeStore) GetAllRecords() []Record {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	records := make([]Record, len(ks.records))
	copy(records, ks.records)
	return records
}

// GetVersion returns the current version counter
func (ks *KnowledgeStore) GetVersion() int64 {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.version
}

// SetMetadata sets a metadata key-value pair
func (ks *KnowledgeStore) SetMetadata(key, value string) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.metadata[key] = value
}

// Helper: Compute SHA256 hash of a record
func computeRecordHash(record Record) string {
	// Create deterministic representation
	data := fmt.Sprintf("%s|%d|%s", record.ID, record.Timestamp, string(record.Content))
	return computeSHA256([]byte(data))
}

// Helper: Compute SHA256 hash of byte data
func computeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Helper: Canonical JSON serialization with sorted keys
func canonicalJSON(v interface{}) ([]byte, error) {
	// Marshal with no indentation for byte-identical output
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Go's json.Marshal already uses sorted keys for maps
	// and deterministic ordering for structs, so this is canonical
	return data, nil
}
