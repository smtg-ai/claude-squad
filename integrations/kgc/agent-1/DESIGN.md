# Agent 1: Knowledge Store Core - Design Document

## Formal Specification

### **O** - Observable Inputs

The KnowledgeStore operates on an append-only log of immutable records:

```
O = {
  records: []Record           // Ordered sequence of records (append-log)
  metadata: map[string]string // System metadata
  version: int64              // Monotonic version counter
}
```

**Record Structure:**
```
Record = {
  ID: string                  // Unique identifier (UUID)
  Timestamp: int64            // Unix nanoseconds (monotonic)
  Content: []byte             // Arbitrary payload
  Metadata: map[string]string // User-defined tags
}
```

**Event Structure:**
```
Event = {
  Type: "append"              // Operation type
  Record: Record              // The record being appended
  Timestamp: int64            // When the event occurred
}
```

---

### **A = μ(O)** - Transformation Function

The KnowledgeStore transformation μ implements four core operations:

#### **μ₁: Append(ctx, record) → (hash, error)**
```
Append: O → O'
  O' = O ∪ {record}
  hash = SHA256(record.ID || record.Timestamp || record.Content)

Invariant: len(O'.records) = len(O.records) + 1
```

**Properties:**
- **Monotonic:** Version counter always increases
- **Idempotent:** Duplicate ID detection prevents double-append
- **Atomic:** Either fully succeeds or fully fails

#### **μ₂: Snapshot(ctx) → (hash, data, error)**
```
Snapshot: O → (H, Σ)
  Σ = Canonicalize(O)       // Deterministic serialization
  H = SHA256(Σ)             // Cryptographic hash

Invariant: ∀ O. Snapshot(O) = Snapshot(O)  (deterministic)
```

**Canonicalization Algorithm:**
1. Sort records by timestamp (ascending)
2. Serialize to JSON with sorted keys
3. Normalize whitespace (no pretty-print)
4. Compute SHA256

**Properties:**
- **Deterministic:** Same state always produces same hash
- **Hash-Stable:** Byte-for-byte identical across runs
- **Tamper-Evident:** Any mutation changes hash

#### **μ₃: Verify(ctx, snapshotHash) → (valid, error)**
```
Verify: O × H → {true, false}
  current = Snapshot(O)
  valid = (current.hash == snapshotHash)
```

**Properties:**
- **Tamper Detection:** O(1) comparison
- **No False Positives:** Collision resistance of SHA256

#### **μ₄: Replay(ctx, events) → (hash, error)**
```
Replay: [E] → O'
  O' = ∅
  for each event ∈ events:
    O' = Append(O', event.Record)
  hash = Snapshot(O').hash

Invariant: Replay(Replay(E)) = Replay(E)
```

**Properties:**
- **Deterministic:** Same events → same final state
- **Reproducible:** Can reconstruct exact state from event log
- **Verifiable:** Output hash matches original

---

### **H** - Forbidden States & Guards

The following states are prevented by design:

1. **H₁: Non-Deterministic Snapshots**
   - Guard: Canonical serialization with sorted keys
   - Test: Snapshot twice, hashes must match

2. **H₂: Concurrent Append Races**
   - Guard: Mutex protection on append operations
   - Test: Concurrent appends must serialize correctly

3. **H₃: Hash Collisions**
   - Guard: SHA256 with full record content
   - Test: Different records produce different hashes

4. **H₄: Unbounded Growth**
   - Guard: (Future) Implement pruning/compaction
   - Current: Accept unbounded growth for MVP

5. **H₅: Non-Idempotent Appends**
   - Guard: Duplicate ID detection
   - Test: Append(x) twice → error on second attempt

---

### **Π** - Proof Targets

#### **Π₁: Deterministic Snapshots**
```
Proof: ∀ O. Snapshot(O) == Snapshot(O)
Test: Call Snapshot 10 times, all hashes identical
```

#### **Π₂: Idempotent Appends**
```
Proof: ∀ x. Append(x) ⊕ Append(x) = Append(x)
Test: First Append(x) succeeds, second Append(x) with same ID fails
```

#### **Π₃: Replay Determinism**
```
Proof: ∀ E. Replay(E) produces hash H, Replay(E) again produces H
Test: Replay event log twice, hashes match
```

#### **Π₄: Tamper Detection**
```
Proof: ∀ O, O'. O ≠ O' ⟹ hash(O) ≠ hash(O')
Test: Modify one record, snapshot hash changes
```

---

### **Σ** - Type Assumptions

```go
// Core Types
type KnowledgeStore struct {
    mu       sync.RWMutex              // Concurrency control
    records  []Record                   // Append-log
    metadata map[string]string          // Store metadata
    version  int64                      // Monotonic counter
    index    map[string]int             // ID → position (fast lookup)
}

type Record struct {
    ID        string            `json:"id"`
    Timestamp int64             `json:"timestamp"`
    Content   []byte            `json:"content"`
    Metadata  map[string]string `json:"metadata"`
}

type Event struct {
    Type      string            `json:"type"`
    Record    Record            `json:"record"`
    Timestamp int64             `json:"timestamp"`
}

// Snapshot serialization format
type SnapshotData struct {
    Records  []Record          `json:"records"`
    Metadata map[string]string `json:"metadata"`
    Version  int64             `json:"version"`
}
```

---

### **Λ** - Priority Order of Operations

1. **Correctness > Performance**
   - All operations must be deterministic
   - Accept slower execution for guaranteed correctness

2. **Idempotence > Throughput**
   - Prevent double-appends even if it costs a hash lookup

3. **Hash Stability > Flexibility**
   - Use canonical JSON with sorted keys
   - No pretty-print, no variations

4. **Tamper Detection > Storage Efficiency**
   - Store full records even if redundant
   - Full SHA256 hashes, no truncation

---

### **Q** - Invariants Preserved

#### **Q₁: Monotonicity**
```
∀ t₁, t₂. (t₁ < t₂) ⟹ (version(O_{t₁}) ≤ version(O_{t₂}))
```
Version counter only increases.

#### **Q₂: Append-Only**
```
∀ O, O'. (O → O' via Append) ⟹ (O ⊂ O')
```
Records are never deleted, only added.

#### **Q₃: Deterministic Hash**
```
∀ O. SHA256(Canonicalize(O)) is unique and reproducible
```
Same state always produces same hash.

#### **Q₄: ID Uniqueness**
```
∀ r₁, r₂ ∈ O.records. (r₁.ID == r₂.ID) ⟹ (r₁ == r₂)
```
No duplicate IDs in the store.

#### **Q₅: Event Replay Equivalence**
```
∀ O, E. (E = events(O)) ⟹ (Replay(E).hash == Snapshot(O).hash)
```
Replaying events produces identical state.

---

## Implementation Strategy

### Phase 1: Core Data Structure
1. Implement `KnowledgeStore` struct with mutex
2. Add append-log storage
3. Add ID index for duplicate detection

### Phase 2: Append Operation
1. Lock for write
2. Check for duplicate ID
3. Append record
4. Increment version
5. Update index
6. Return hash

### Phase 3: Snapshot Operation
1. Lock for read
2. Create SnapshotData struct
3. Sort records by timestamp
4. Serialize to canonical JSON
5. Compute SHA256
6. Return hash + data

### Phase 4: Verify Operation
1. Call Snapshot to get current hash
2. Compare with provided hash
3. Return result

### Phase 5: Replay Operation
1. Create new empty store
2. For each event, call Append
3. Return final snapshot hash

### Phase 6: Testing
1. Test determinism (multiple snapshots)
2. Test idempotence (duplicate appends)
3. Test replay (event reconstruction)
4. Test concurrency (parallel appends)
5. Test tamper detection

---

## Concurrency Model

```
┌─────────────────┐
│  Append(record) │
└────────┬────────┘
         │
         ▼
    ┌────────┐
    │ mu.Lock()  │ ◄─── Exclusive write lock
    └────┬───┘
         │
         ▼
    ┌──────────────┐
    │ Check ID dup │
    └────┬─────────┘
         │
         ▼
    ┌──────────────┐
    │ Append to log│
    └────┬─────────┘
         │
         ▼
    ┌──────────────┐
    │ mu.Unlock()  │
    └──────────────┘

┌────────────────┐
│  Snapshot()    │
└────────┬───────┘
         │
         ▼
    ┌────────────┐
    │ mu.RLock() │ ◄─── Shared read lock (allows concurrent reads)
    └────┬───────┘
         │
         ▼
    ┌──────────────┐
    │ Canonicalize │
    └────┬─────────┘
         │
         ▼
    ┌──────────────┐
    │ mu.RUnlock() │
    └──────────────┘
```

---

## Error Handling

| Error | Condition | Response |
|-------|-----------|----------|
| `ErrDuplicateID` | Append with existing ID | Return error, no state change |
| `ErrInvalidRecord` | Nil record or empty ID | Return error, no state change |
| `ErrVerifyFailed` | Hash mismatch in Verify | Return false, no error |
| `ErrReplayFailed` | Event replay encounters error | Return error, partial state |

---

## Performance Characteristics

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Append | O(1) amortized | O(n) for n records |
| Snapshot | O(n log n) for sort | O(n) for serialization |
| Verify | O(n) for snapshot | O(n) for serialization |
| Replay | O(m) for m events | O(m) for final state |

---

## Test Coverage

### Unit Tests
1. **TestAppendDeterminism** - Append produces same hash for same record
2. **TestSnapshotDeterminism** - Multiple snapshots produce identical hash
3. **TestIdempotence** - Duplicate ID append fails
4. **TestReplayDeterminism** - Replay produces same hash
5. **TestTamperDetection** - Modified state changes hash
6. **TestConcurrentAppends** - Race-free parallel appends

### Integration Tests
1. **TestReplayFromEvents** - Full event log replay
2. **TestLargeDataset** - 1000+ records
3. **TestVerifyAfterAppend** - Verify works after modifications

---

## Success Criteria

- ✅ All tests pass: `go test -v`
- ✅ Compiles cleanly: `go build`
- ✅ Snapshot hashes are deterministic (run 10 times, same hash)
- ✅ Idempotence: Duplicate appends fail with ErrDuplicateID
- ✅ Replay: Reconstructing from events produces identical hash
- ✅ No race conditions: `go test -race`

---

## Future Enhancements

1. **Compaction** - Periodic pruning of old records
2. **Sharding** - Partition large datasets
3. **Persistence** - Disk-backed storage
4. **Streaming** - Real-time event streaming
5. **Merkle Trees** - O(log n) tamper detection

---

## References

- **Charter:** `/integrations/kgc/contracts/10_AGENT_SWARM_CHARTER.md`
- **Interface:** `/integrations/kgc/contracts/SUBSTRATE_INTERFACES.md`
- **Agent Scope:** Agent 1 (Knowledge Store Core)

---

**Status:** Implementation Ready
**Next Step:** Implement `knowledge_store.go`
