# KGC Knowledge Substrate - Interface Contracts

## Overview

This document defines the formal composition contracts for the KGC-backed knowledge substrate. All agents must implement these interfaces to ensure deterministic reconciliation.

---

## Core Interfaces

### 1. KnowledgeStore Interface

```go
// KnowledgeStore provides immutable append-log semantics with hash-stable snapshots
type KnowledgeStore interface {
    // Append: O → O' (monotonic operation)
    Append(ctx context.Context, record Record) (hash string, err error)

    // Snapshot: O → Σ (deterministic canonical form)
    Snapshot(ctx context.Context) (hash string, data []byte, err error)

    // Verify: O × H → bool (tamper detection)
    Verify(ctx context.Context, snapshotHash string) (valid bool, err error)

    // Replay: O × [E] → O' (deterministic reconstruction)
    Replay(ctx context.Context, events []Event) (hash string, err error)
}
```

**Invariants:**
- `Append` is idempotent: `∀ x. Append(x) = Append(Append(x))`
- `Snapshot` is deterministic: `∀ O. Snapshot(O) = Snapshot(O)` (identical hash)
- `Verify` detects mutations: `∀ O, O'. O ≠ O' ⟹ hash(O) ≠ hash(O')`
- `Replay` produces identical results: `∀ E. Replay(Replay(E)) = Replay(E)`

---

### 2. Receipt Interface

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
    CompositionOp  string            `json:"composition_op"`  // How this patches merge with siblings
    ConflictPolicy string            `json:"conflict_policy"` // "fail_fast" | "merge" | "skip"
}
```

**Proof Requirements:**
- Every Receipt must include `ReplayScript` that is executable
- `InputHash` + `OutputHash` + `ReplayScript` form a verifiable triplet
- `CompositionOp` declares merge semantics to Reconciler

---

### 3. AgentRun Interface

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

---

### 4. Reconciler Interface

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

---

### 5. Proof Targets

| Proof | Description | Test Command |
|-------|-------------|--------------|
| **P1** | Deterministic substrate build | `make proof-p1` |
| **P2** | Multi-agent patch integrity | `make proof-p2` |
| **P3** | Receipt-chain correctness | `make proof-p3` |
| **P4** | Cross-repo integration contract | `make proof-p4` |

---

## Tranche File Ownership (No Collisions)

```
integrations/kgc/
├── contracts/                 (Agent 0 only)
├── agent-0/
│   ├── DESIGN.md
│   ├── reconciler.go         (Reconciler implementation)
│   ├── reconciler_test.go
│   └── RECEIPT.json
├── agent-1/
│   ├── DESIGN.md
│   ├── knowledge_store.go    (KnowledgeStore core)
│   ├── knowledge_store_test.go
│   └── RECEIPT.json
├── agent-2/
│   ├── DESIGN.md
│   ├── receipt.go            (Receipt chaining + verification)
│   ├── receipt_test.go
│   └── RECEIPT.json
├── agent-3/
│   ├── DESIGN.md
│   ├── policy_bridge.go      (Bridge to unrdf policy packs)
│   ├── policy_bridge_test.go
│   └── RECEIPT.json
├── agent-4/
│   ├── DESIGN.md
│   ├── capacity_allocator.go (Resource allocation)
│   ├── capacity_allocator_test.go
│   └── RECEIPT.json
├── agent-5/
│   ├── DESIGN.md
│   ├── workspace_isolator.go (Per-agent sandboxing)
│   ├── workspace_isolator_test.go
│   └── RECEIPT.json
├── agent-6/
│   ├── DESIGN.md
│   ├── task_router.go        (Routing + task graph)
│   ├── task_router_test.go
│   └── RECEIPT.json
├── agent-7/
│   ├── DESIGN.md
│   ├── docs/                 (Diataxis scaffolding)
│   ├── tutorial/
│   ├── how_to/
│   ├── reference/
│   └── RECEIPT.json
├── agent-8/
│   ├── DESIGN.md
│   ├── harness.go            (Workload harness)
│   ├── harness_test.go
│   └── RECEIPT.json
└── agent-9/
    ├── DESIGN.md
    ├── demo.go               (End-to-end demo)
    ├── demo_test.go
    └── RECEIPT.json
```

---

## Composition Laws (⊕ Operator)

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

## Integration with seanchatmangpt/unrdf

### Boundary Contract

The KGC substrate provides a thin adapter boundary to unrdf:

```go
// PolicyPackBridge: unrdf policies → KGC operations
type PolicyPackBridge interface {
    LoadPolicyPack(packName string) (*PolicyPack, error)
    ValidateAgainstPolicies(ctx context.Context, patch *Delta) error
    ApplyPolicies(ctx context.Context, agent *AgentRun) (*AgentRun, error)
}
```

**Versioning:**
- unrdf contract version: TBD (to be populated from unrdf repo)
- KGC substrate version: v0.1.0-alpha
- Compatibility: loose coupling via interface only

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

## Success Criteria

1. ✅ All agents produce DESIGN.md + code + tests + RECEIPT.json
2. ✅ No file collisions (each agent owns distinct tranche)
3. ✅ All tests pass independently
4. ✅ Reconciler validates all patch compositions without conflicts
5. ✅ All four proof targets (P1-P4) pass
6. ✅ End-to-end demo runs 3+ concurrent agents with deterministic output
