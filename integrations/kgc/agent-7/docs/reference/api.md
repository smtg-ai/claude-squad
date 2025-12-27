# API Reference

Complete API reference for the KGC Knowledge Substrate.

## Table of Contents

- [Agent 1: KnowledgeStore](#agent-1-knowledgestore)
- [Agent 2: Receipt](#agent-2-receipt)
- [Agent 3: PolicyPackBridge](#agent-3-policypackbridge)
- [Agent 4: CapacityAllocator](#agent-4-capacityallocator)
- [Agent 5: WorkspaceIsolator](#agent-5-workspaceisolator)
- [Agent 6: TaskRouter](#agent-6-taskrouter)
- [Agent 0: Reconciler](#agent-0-reconciler)

---

## Agent 1: KnowledgeStore

**Import:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-1`

### Types

#### KnowledgeStore

```go
type KnowledgeStore struct {
    // contains filtered or unexported fields
}
```

Provides immutable append-log semantics with hash-stable snapshots.

### Constructors

#### NewKnowledgeStore

```go
func NewKnowledgeStore() *KnowledgeStore
```

Creates a new in-memory knowledge store with default configuration.

**Returns:** `*KnowledgeStore` - New store instance

**Example:**

```go
store := agent1.NewKnowledgeStore()
defer store.Close()
```

#### NewKnowledgeStoreWithConfig

```go
func NewKnowledgeStoreWithConfig(config KnowledgeStoreConfig) (*KnowledgeStore, error)
```

Creates a knowledge store with custom configuration.

**Parameters:**
- `config` - Store configuration (backend, path, limits)

**Returns:**
- `*KnowledgeStore` - New store instance
- `error` - Configuration validation error

**Example:**

```go
config := agent1.KnowledgeStoreConfig{
    Backend:       "file",
    StoragePath:   "/var/lib/kgc/store.db",
    MaxRecords:    1000000,
    Deterministic: true,
}
store, err := agent1.NewKnowledgeStoreWithConfig(config)
if err != nil {
    log.Fatalf("Config error: %v", err)
}
```

### Methods

#### Append

```go
func (ks *KnowledgeStore) Append(ctx context.Context, record Record) (string, error)
```

Appends a record to the store (idempotent operation).

**Parameters:**
- `ctx` - Context (with timeout recommended)
- `record` - Record to append

**Returns:**
- `string` - SHA256 hash of appended record
- `error` - Append error

**Example:**

```go
record := agent1.Record{
    Key:   "user:123",
    Value: "alice@example.com",
}
hash, err := store.Append(ctx, record)
if err != nil {
    return fmt.Errorf("append failed: %w", err)
}
```

**Invariants:**
- Idempotent: `Append(x) ⊕ Append(x) = Append(x)`
- Hash is deterministic for same record

#### Snapshot

```go
func (ks *KnowledgeStore) Snapshot(ctx context.Context) (string, []byte, error)
```

Creates a deterministic snapshot of current state.

**Parameters:**
- `ctx` - Context

**Returns:**
- `string` - SHA256 hash of snapshot
- `[]byte` - Serialized snapshot data
- `error` - Snapshot error

**Example:**

```go
hash, data, err := store.Snapshot(ctx)
if err != nil {
    return fmt.Errorf("snapshot failed: %w", err)
}
fmt.Printf("Snapshot hash: %s (size: %d bytes)\n", hash, len(data))
```

**Invariants:**
- Hash-stable: `Snapshot(O) = Snapshot(O)` (same hash on repeated calls)
- Deterministic: Same state always produces same hash

#### Verify

```go
func (ks *KnowledgeStore) Verify(ctx context.Context, snapshotHash string) (bool, error)
```

Verifies a snapshot hash matches current state.

**Parameters:**
- `ctx` - Context
- `snapshotHash` - Expected hash to verify

**Returns:**
- `bool` - True if hash matches current state
- `error` - Verification error

**Example:**

```go
valid, err := store.Verify(ctx, expectedHash)
if err != nil {
    return fmt.Errorf("verification failed: %w", err)
}
if !valid {
    return fmt.Errorf("tamper detected: hash mismatch")
}
```

#### Replay

```go
func (ks *KnowledgeStore) Replay(ctx context.Context, events []Event) (string, error)
```

Replays events to reconstruct state deterministically.

**Parameters:**
- `ctx` - Context
- `events` - Ordered list of events to replay

**Returns:**
- `string` - Final hash after replay
- `error` - Replay error

**Example:**

```go
events := []agent1.Event{
    {Type: "append", Data: record1},
    {Type: "append", Data: record2},
}
finalHash, err := store.Replay(ctx, events)
if err != nil {
    return fmt.Errorf("replay failed: %w", err)
}
```

**Invariants:**
- Deterministic: `Replay(E) = Replay(E)` (same events → same result)

#### Close

```go
func (ks *KnowledgeStore) Close() error
```

Closes the store and releases resources.

**Returns:** `error` - Close error

**Example:**

```go
defer store.Close()
```

---

## Agent 2: Receipt

**Import:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-2`

### Types

#### Receipt

```go
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
```

### Functions

#### CreateReceipt

```go
func CreateReceipt(beforeHash, afterHash, replayScript string) *Receipt
```

Creates a new receipt with generated execution ID.

**Parameters:**
- `beforeHash` - Input state hash
- `afterHash` - Output state hash
- `replayScript` - Bash script to reproduce execution

**Returns:** `*Receipt` - New receipt instance

**Example:**

```go
receipt := agent2.CreateReceipt(
    "sha256:abc123",
    "sha256:def456",
    "#!/bin/bash\ngo test -v\n",
)
receipt.AgentID = "agent-1"
receipt.ProofArtifacts["test_log"] = testOutput
```

#### VerifyReceipt

```go
func VerifyReceipt(receipt *Receipt) bool
```

Verifies receipt has all required fields and valid hashes.

**Parameters:**
- `receipt` - Receipt to verify

**Returns:** `bool` - True if receipt is valid

**Example:**

```go
if !agent2.VerifyReceipt(receipt) {
    log.Fatal("Invalid receipt")
}
```

#### ChainReceipts

```go
func ChainReceipts(receipts []*Receipt) (*ChainedReceipt, error)
```

Chains receipts, verifying continuity.

**Parameters:**
- `receipts` - Ordered list of receipts to chain

**Returns:**
- `*ChainedReceipt` - Chained receipt with combined hash
- `error` - Chain error (if continuity broken)

**Example:**

```go
chained, err := agent2.ChainReceipts([]*Receipt{r1, r2, r3})
if err != nil {
    log.Fatalf("Chain broken: %v", err)
}
fmt.Printf("Chain hash: %s\n", chained.ChainHash)
```

---

## Agent 3: PolicyPackBridge

**Import:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-3`

### Types

#### PolicyPackBridge

```go
type PolicyPackBridge struct {
    // contains filtered or unexported fields
}
```

### Functions

#### NewPolicyPackBridge

```go
func NewPolicyPackBridge(unrdfPath string) (*PolicyPackBridge, error)
```

Creates a bridge to unrdf policy packs.

**Parameters:**
- `unrdfPath` - Path to unrdf repository

**Returns:**
- `*PolicyPackBridge` - New bridge instance
- `error` - Initialization error

**Example:**

```go
bridge, err := agent3.NewPolicyPackBridge("/tmp/unrdf-integration")
if err != nil {
    log.Fatalf("Bridge init failed: %v", err)
}
```

### Methods

#### LoadPolicyPack

```go
func (ppb *PolicyPackBridge) LoadPolicyPack(packName string) (*PolicyPack, error)
```

Loads a policy pack from unrdf.

**Parameters:**
- `packName` - Name of policy pack to load

**Returns:**
- `*PolicyPack` - Loaded policy pack
- `error` - Load error

**Example:**

```go
pack, err := bridge.LoadPolicyPack("kgc-validation-v1")
if err != nil {
    return fmt.Errorf("load failed: %w", err)
}
```

#### ValidateAgainstPolicies

```go
func (ppb *PolicyPackBridge) ValidateAgainstPolicies(ctx context.Context, patch *Delta) error
```

Validates a patch against loaded policies.

**Parameters:**
- `ctx` - Context
- `patch` - Delta/patch to validate

**Returns:** `error` - Validation error (nil if valid)

**Example:**

```go
if err := bridge.ValidateAgainstPolicies(ctx, delta); err != nil {
    log.Printf("Policy violation: %v", err)
    return err
}
```

---

## Agent 4: CapacityAllocator

**Import:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-4`

### Types

#### CapacityAllocator

```go
type CapacityAllocator struct {
    // contains filtered or unexported fields
}
```

### Functions

#### NewCapacityAllocator

```go
func NewCapacityAllocator(maxAgents int, resourceBudget int) *CapacityAllocator
```

Creates a new capacity allocator.

**Parameters:**
- `maxAgents` - Maximum number of agents
- `resourceBudget` - Total resource budget

**Returns:** `*CapacityAllocator` - New allocator

**Example:**

```go
allocator := agent4.NewCapacityAllocator(10, 1000)
```

### Methods

#### AllocateResources

```go
func (ca *CapacityAllocator) AllocateResources(agentCount int) (*Allocation, error)
```

Allocates resources deterministically.

**Parameters:**
- `agentCount` - Number of agents requesting resources

**Returns:**
- `*Allocation` - Resource allocation plan
- `error` - Allocation error

**Example:**

```go
allocation, err := allocator.AllocateResources(3)
if err != nil {
    return fmt.Errorf("allocation failed: %w", err)
}
```

#### RoundRobinSchedule

```go
func (ca *CapacityAllocator) RoundRobinSchedule(agents []string, tasks []string) *Schedule
```

Schedules tasks using round-robin algorithm.

**Parameters:**
- `agents` - List of agent IDs
- `tasks` - List of task IDs

**Returns:** `*Schedule` - Deterministic schedule

**Example:**

```go
schedule := allocator.RoundRobinSchedule(
    []string{"agent-1", "agent-2", "agent-3"},
    []string{"task-1", "task-2", "task-3", "task-4", "task-5"},
)
```

---

## Agent 6: TaskRouter

**Import:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-6`

### Types

#### TaskRouter

```go
type TaskRouter struct {
    // contains filtered or unexported fields
}
```

### Functions

#### NewTaskRouter

```go
func NewTaskRouter() *TaskRouter
```

Creates a new task router.

**Returns:** `*TaskRouter` - New router instance

**Example:**

```go
router := agent6.NewTaskRouter()
```

### Methods

#### Route

```go
func (tr *TaskRouter) Route(task *Task, predicates map[string]interface{}) (string, error)
```

Routes a task deterministically based on predicates.

**Parameters:**
- `task` - Task to route
- `predicates` - Routing predicates (XOR/AND/OR)

**Returns:**
- `string` - Target agent ID
- `error` - Routing error

**Example:**

```go
predicates := map[string]interface{}{
    "type":     "validation",
    "priority": 1,
}
agentID, err := router.Route(task, predicates)
if err != nil {
    return fmt.Errorf("routing failed: %w", err)
}
```

#### EvaluateTaskGraph

```go
func (tr *TaskRouter) EvaluateTaskGraph(tasks []*Task) ([]string, error)
```

Evaluates task graph and returns topologically sorted execution order.

**Parameters:**
- `tasks` - List of tasks with dependencies

**Returns:**
- `[]string` - Ordered task IDs
- `error` - Graph evaluation error (e.g., cycle detected)

**Example:**

```go
executionOrder, err := router.EvaluateTaskGraph(tasks)
if err != nil {
    return fmt.Errorf("graph evaluation failed: %w", err)
}
```

---

## Agent 0: Reconciler

**Import:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-0`

### Types

#### Reconciler

```go
type Reconciler struct {
    // contains filtered or unexported fields
}
```

### Functions

#### NewReconciler

```go
func NewReconciler() *Reconciler
```

Creates a new reconciler.

**Returns:** `*Reconciler` - New reconciler instance

**Example:**

```go
reconciler := agent0.NewReconciler()
```

### Methods

#### Reconcile

```go
func (r *Reconciler) Reconcile(ctx context.Context, deltas []*Delta) (*Delta, *ConflictReport, error)
```

Reconciles multiple deltas/patches into single coherent state.

**Parameters:**
- `ctx` - Context
- `deltas` - List of deltas to reconcile

**Returns:**
- `*Delta` - Final reconciled delta (if successful)
- `*ConflictReport` - Conflict report (if conflicts detected)
- `error` - Reconciliation error

**Example:**

```go
finalDelta, conflicts, err := reconciler.Reconcile(ctx, deltas)
if err != nil {
    return fmt.Errorf("reconciliation failed: %w", err)
}
if !conflicts.Resolved {
    log.Printf("Conflicts detected: %d", len(conflicts.Conflicts))
}
```

#### ValidateComposition

```go
func (r *Reconciler) ValidateComposition(delta1, delta2 *Delta) (bool, string)
```

Validates if two deltas compose without collision.

**Parameters:**
- `delta1` - First delta
- `delta2` - Second delta

**Returns:**
- `bool` - True if compatible
- `string` - Reason (if incompatible)

**Example:**

```go
compatible, reason := reconciler.ValidateComposition(delta1, delta2)
if !compatible {
    log.Printf("Incompatible deltas: %s", reason)
}
```

---

## Error Handling

All API methods follow Go error handling conventions:

```go
result, err := someOperation(ctx, params)
if err != nil {
    // Handle error
    return fmt.Errorf("operation failed: %w", err)
}
// Use result
```

### Common Error Types

```go
var (
    ErrInvalidConfig     = errors.New("invalid configuration")
    ErrStoreNotFound     = errors.New("knowledge store not found")
    ErrReceiptInvalid    = errors.New("receipt validation failed")
    ErrChainBroken       = errors.New("receipt chain continuity broken")
    ErrConflictDetected  = errors.New("patch conflict detected")
    ErrNonDeterministic  = errors.New("non-determinism detected")
)
```

---

## Context Usage

All operations support context for:

- Timeout control
- Cancellation
- Request-scoped values

**Example:**

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

hash, err := store.Append(ctx, record)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Fatal("Operation timed out")
    }
    log.Fatalf("Append failed: %v", err)
}
```

---

## See Also

- [Substrate Interfaces](substrate_interfaces.md) - Interface definitions
- [CLI Reference](cli.md) - Command-line tools
- [Getting Started Tutorial](../tutorial/getting_started.md)
- [How-To Guides](../how_to/)
