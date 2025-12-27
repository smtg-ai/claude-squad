# Substrate Interfaces Reference

This document provides the complete interface specifications for the KGC knowledge substrate. All agents must implement these interfaces to ensure deterministic reconciliation.

## Overview

The KGC substrate defines 5 core interfaces:

1. **KnowledgeStore** - Immutable append-log with hash-stable snapshots
2. **Receipt** - Cryptographic execution proofs
3. **AgentRun** - Agent execution contract
4. **Reconciler** - Multi-agent composition validator
5. **PolicyPackBridge** - Integration with unrdf policies

## 1. KnowledgeStore Interface

**Package:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-1`

### Interface Definition

```go
type KnowledgeStore interface {
    // Append: O → O' (monotonic operation)
    // Appends a record to the store
    // Returns: hash of the appended record
    // Invariant: Append(x) ⊕ Append(x) = Append(x) (idempotent)
    Append(ctx context.Context, record Record) (hash string, err error)

    // Snapshot: O → Σ (deterministic canonical form)
    // Creates a deterministic snapshot of current state
    // Returns: hash + serialized data
    // Invariant: Snapshot(O) = Snapshot(O) (hash-stable)
    Snapshot(ctx context.Context) (hash string, data []byte, err error)

    // Verify: O × H → bool (tamper detection)
    // Verifies a snapshot hash matches current state
    // Returns: true if hash is valid
    // Invariant: Verify detects mutations
    Verify(ctx context.Context, snapshotHash string) (valid bool, err error)

    // Replay: O × [E] → O' (deterministic reconstruction)
    // Replays events to reconstruct state
    // Returns: final hash after replay
    // Invariant: Replay(Replay(E)) = Replay(E)
    Replay(ctx context.Context, events []Event) (hash string, err error)
}
```

### Data Types

#### Record

```go
type Record struct {
    Key   string      `json:"key"`
    Value interface{} `json:"value"`
}
```

#### Event

```go
type Event struct {
    Type      string      `json:"type"`       // "append" | "snapshot" | "verify"
    Timestamp int64       `json:"timestamp"`  // Unix nanoseconds
    Data      interface{} `json:"data"`       // Event-specific data
}
```

#### KnowledgeStoreConfig

```go
type KnowledgeStoreConfig struct {
    Backend       string `json:"backend"`        // "memory" | "file"
    StoragePath   string `json:"storage_path"`   // File path (if backend=file)
    MaxRecords    int    `json:"max_records"`    // Maximum records to store
    Deterministic bool   `json:"deterministic"`  // Enforce determinism
    SyncMode      string `json:"sync_mode"`      // "fsync" | "async"
}
```

### Methods

#### NewKnowledgeStore

```go
func NewKnowledgeStore() *KnowledgeStore
```

Creates a new in-memory knowledge store with default configuration.

**Example:**

```go
store := agent1.NewKnowledgeStore()
```

#### NewKnowledgeStoreWithConfig

```go
func NewKnowledgeStoreWithConfig(config KnowledgeStoreConfig) (*KnowledgeStore, error)
```

Creates a knowledge store with custom configuration.

**Example:**

```go
config := agent1.KnowledgeStoreConfig{
    Backend:       "file",
    StoragePath:   "/var/lib/kgc/store.db",
    MaxRecords:    1000000,
    Deterministic: true,
    SyncMode:      "fsync",
}
store, err := agent1.NewKnowledgeStoreWithConfig(config)
```

### Invariants

#### I1: Deterministic Snapshots

```
∀ O. Snapshot(O) = Snapshot(O)
```

Taking multiple snapshots of the same state produces identical hashes.

#### I2: Idempotent Append

```
∀ x. Append(x) ⊕ Append(x) = Append(x)
```

Appending the same record twice is equivalent to appending it once.

#### I3: Tamper Detection

```
∀ O, O'. O ≠ O' ⟹ hash(O) ≠ hash(O')
```

Different states produce different hashes (collision-resistant).

#### I4: Replay Determinism

```
∀ E. Replay(Replay(E)) = Replay(E)
```

Replaying events multiple times produces the same result.

---

## 2. Receipt Interface

**Package:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-2`

### Interface Definition

```go
type Receipt struct {
    // Execution metadata
    ExecutionID    string            `json:"execution_id"`    // UUID for this run
    AgentID        string            `json:"agent_id"`        // Which agent (0-9)
    Timestamp      int64             `json:"timestamp"`       // Unix nanoseconds
    ToolchainVer   string            `json:"toolchain_ver"`   // Go version, etc.

    // Determinism proof
    InputHash      string            `json:"input_hash"`      // SHA256(inputs)
    OutputHash     string            `json:"output_hash"`     // SHA256(outputs)
    ProofArtifacts map[string]string `json:"proof_artifacts"` // test logs, diffs, snapshots

    // Replay instructions
    ReplayScript   string            `json:"replay_script"`   // Bash script that reproduces this exact run

    // Composition law
    CompositionOp  string            `json:"composition_op"`  // How patches merge with siblings
    ConflictPolicy string            `json:"conflict_policy"` // "fail_fast" | "merge" | "skip"
}
```

### Methods

#### CreateReceipt

```go
func CreateReceipt(beforeHash, afterHash, replayScript string) *Receipt
```

Creates a new receipt with generated execution ID and current timestamp.

**Example:**

```go
receipt := agent2.CreateReceipt(
    "sha256:abc123...",
    "sha256:def456...",
    "#!/bin/bash\ngo test -v\n",
)
```

#### VerifyReceipt

```go
func VerifyReceipt(receipt *Receipt) bool
```

Verifies a receipt has all required fields and valid hashes.

**Example:**

```go
if agent2.VerifyReceipt(receipt) {
    fmt.Println("Receipt is valid")
}
```

#### ChainReceipts

```go
func ChainReceipts(receipts []*Receipt) (*ChainedReceipt, error)
```

Chains multiple receipts, verifying continuity (R1.output_hash == R2.input_hash).

**Example:**

```go
chained, err := agent2.ChainReceipts([]*Receipt{r1, r2, r3})
if err != nil {
    log.Fatalf("Chain broken: %v", err)
}
```

### Data Types

#### ChainedReceipt

```go
type ChainedReceipt struct {
    Receipts  []*Receipt `json:"receipts"`
    ChainHash string     `json:"chain_hash"` // SHA256(all output hashes)
}
```

### Composition Operations

| CompositionOp | Description | Use Case |
|---------------|-------------|----------|
| `append` | Add to end of sequence | Independent operations |
| `merge` | Combine overlapping changes | Collaborative editing |
| `replace` | Overwrite previous state | Versioned updates |
| `extend` | Add without overwrite | Incremental builds |

### Conflict Policies

| ConflictPolicy | Description | Behavior on Conflict |
|----------------|-------------|---------------------|
| `fail_fast` | Reject immediately | Return error, no merge |
| `merge` | Attempt automatic merge | Use merge strategy |
| `skip` | Ignore conflicting patch | Continue with others |

---

## 3. AgentRun Interface

**Package:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-0`

### Interface Definition

```go
type AgentRun interface {
    // Execute: (O, Λ) → (O', R)
    // O = input observable state
    // Λ = priority/constraint order
    // O' = output observable state
    // R = Receipt (proof of determinism)
    Execute(ctx context.Context, inputs *AgentInput) (*AgentOutput, *Receipt, error)

    // Design: () → (DESIGN.md)
    // Must declare:
    // - Σ (typing assumptions)
    // - Λ (priority order)
    // - Q (invariants preserved)
    Design() string

    // Proof: () → PassFail
    // Run all test cases and harness; return final verdict
    Proof(ctx context.Context) (passed bool, artifacts map[string]string, err error)
}
```

### Data Types

#### AgentInput

```go
type AgentInput struct {
    State      map[string]interface{} `json:"state"`
    Priorities []string               `json:"priorities"`
    Context    map[string]string      `json:"context"`
}
```

#### AgentOutput

```go
type AgentOutput struct {
    State         map[string]interface{} `json:"state"`
    ModifiedFiles []string               `json:"modified_files"`
    Logs          []string               `json:"logs"`
}
```

---

## 4. Reconciler Interface

**Package:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-0`

### Interface Definition

```go
type Reconciler interface {
    // Π: [Δ] → Δ_final | ConflictReport
    // Input: ordered list of change-sets (receipts + patches)
    // Output: single coherent state OR detailed conflict report
    Reconcile(ctx context.Context, deltas []*Delta) (*Delta, *ConflictReport, error)

    // ValidateComposition: Δ₁ ⊕ Δ₂ → bool
    // Check if two patches compose without collision
    ValidateComposition(delta1, delta2 *Delta) (compatible bool, reason string)
}
```

### Data Types

#### Delta

```go
type Delta struct {
    AgentID        string            `json:"agent_id"`
    Files          []string          `json:"files"`          // Modified files
    Receipt        *Receipt          `json:"receipt"`
    CompositionOp  string            `json:"composition_op"`
    ConflictPolicy string            `json:"conflict_policy"`
}
```

#### ConflictReport

```go
type ConflictReport struct {
    Conflicts      []Conflict `json:"conflicts"`
    Resolved       bool       `json:"resolved"`
    ResolutionLog  []string   `json:"resolution_log"`
}

type Conflict struct {
    File    string   `json:"file"`
    Agents  []string `json:"agents"`
    Reason  string   `json:"reason"`
}
```

---

## 5. PolicyPackBridge Interface

**Package:** `github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-3`

### Interface Definition

```go
type PolicyPackBridge interface {
    // LoadPolicyPack loads a policy pack from unrdf
    LoadPolicyPack(packName string) (*PolicyPack, error)

    // ValidateAgainstPolicies validates a patch against loaded policies
    ValidateAgainstPolicies(ctx context.Context, patch *Delta) error

    // ApplyPolicies applies policies to an agent run
    ApplyPolicies(ctx context.Context, agent *AgentRun) (*AgentRun, error)
}
```

### Data Types

#### PolicyPack

```go
type PolicyPack struct {
    Name    string   `json:"name"`
    Version string   `json:"version"`
    Rules   []Rule   `json:"rules"`
}

type Rule struct {
    ID          string `json:"id"`
    Description string `json:"description"`
    Predicate   string `json:"predicate"`
    Action      string `json:"action"` // "allow" | "deny" | "warn"
}
```

---

## Composition Laws

### Law 1: Idempotence

```
∀ Δ. Δ ⊕ Δ = Δ
```

Applying the same patch twice is equivalent to applying it once.

### Law 2: Associativity

```
∀ Δ₁, Δ₂, Δ₃. (Δ₁ ⊕ Δ₂) ⊕ Δ₃ = Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
```

Patch order does not matter if patches are disjoint.

### Law 3: Conflict Detection

```
∀ Δ₁, Δ₂. (Δ₁.files ∩ Δ₂.files ≠ ∅) ⟹ CONFLICT(Δ₁, Δ₂)
```

Overlapping file edits trigger explicit conflict.

### Law 4: Determinism

```
∀ Δ. Replay(Δ.ReplayScript, Δ.InputHash) = Δ.OutputHash
```

Every patch is reproducible.

---

## Quality Attributes

| Attribute | Target | Test |
|-----------|--------|------|
| Determinism | 100% | P1, P3 |
| Idempotence | 100% | Agent-specific tests |
| Conflict detection | <1ms per delta pair | P2 |
| Replay time | <1s per receipt | P3 |
| Cross-repo integration latency | <100ms | P4 |

---

## See Also

- [API Reference](api.md) - Complete method signatures
- [CLI Reference](cli.md) - Command-line tools
- [Getting Started Tutorial](../tutorial/getting_started.md)
- [Composition Laws Explanation](../explanation/composition_laws.md)
