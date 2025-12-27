# Composition Laws

This document explains the mathematical laws that govern how agent patches compose in the KGC substrate, enabling provably correct multi-agent collaboration.

## What is Composition?

**Composition** is the process of combining multiple agent patches into a single coherent state.

### The Challenge

When 10 agents work in parallel, each produces changes:

```
Agent 1: Modifies files {A, B}
Agent 2: Modifies files {C, D}
Agent 3: Modifies files {B, E}  ← Overlaps with Agent 1!
...
Agent 10: Modifies files {X, Y}
```

**Questions:**

- Do these changes conflict?
- Can they be merged safely?
- Is the result deterministic?

**Solution:** Composition laws provide formal guarantees.

## The Composition Operator (⊕)

The **composition operator** `⊕` combines two patches:

```
Δ₁ ⊕ Δ₂ = Δ_final
```

**Read as:** "Patch Δ₁ composed with Δ₂ produces final patch Δ_final"

### Example

```
Δ₁: Add function foo() to file.go
Δ₂: Add function bar() to file.go

Δ₁ ⊕ Δ₂: file.go contains both foo() and bar()
```

## Four Fundamental Laws

### Law 1: Idempotence

```
∀ Δ. Δ ⊕ Δ = Δ
```

**Read as:** Applying the same patch twice is equivalent to applying it once.

**Example:**

```go
store := NewKnowledgeStore()
record := Record{Key: "x", Value: "1"}

// Apply patch twice
hash1, _ := store.Append(ctx, record)
hash2, _ := store.Append(ctx, record)

// Results are identical
assert(hash1 == hash2)  // ✓
```

**Why Important?**

- Makes retries safe
- Network failures don't corrupt state
- Distributed systems can safely retry operations

**Real-World Impact:**

```bash
# Network failure during git push
$ git push origin main
error: RPC failed; HTTP 500

# Safe to retry (idempotent)
$ git push origin main
✓ Everything up-to-date
```

---

### Law 2: Associativity

```
∀ Δ₁, Δ₂, Δ₃. (Δ₁ ⊕ Δ₂) ⊕ Δ₃ = Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
```

**Read as:** Grouping doesn't matter if patches are disjoint.

**Example:**

```
Δ₁: Edit file_a.go
Δ₂: Edit file_b.go
Δ₃: Edit file_c.go

(Δ₁ ⊕ Δ₂) ⊕ Δ₃ = Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
```

**Why Important?**

- Agent execution order doesn't matter
- Can process patches in any order
- Enables parallel composition

**Visual Proof:**

```
Path 1: ((Δ₁ ⊕ Δ₂) ⊕ Δ₃)
  Step 1: Combine Δ₁ and Δ₂ → {file_a.go, file_b.go}
  Step 2: Add Δ₃ → {file_a.go, file_b.go, file_c.go}

Path 2: (Δ₁ ⊕ (Δ₂ ⊕ Δ₃))
  Step 1: Combine Δ₂ and Δ₃ → {file_b.go, file_c.go}
  Step 2: Add Δ₁ → {file_a.go, file_b.go, file_c.go}

Result: Same final state ✓
```

**Caveat:** Only holds if patches are **disjoint** (no overlapping files).

---

### Law 3: Conflict Detection

```
∀ Δ₁, Δ₂. (Δ₁.files ∩ Δ₂.files ≠ ∅) ⟹ CONFLICT(Δ₁, Δ₂)
```

**Read as:** If patches overlap, explicit conflict is triggered.

**Example:**

```
Δ₁: Modify config.json (lines 1-10)
Δ₂: Modify config.json (lines 5-15)

Δ₁.files ∩ Δ₂.files = {config.json} ≠ ∅

Result: CONFLICT detected
```

**Why Important?**

- No silent data loss
- Explicit conflict resolution required
- Prevents race conditions

**Conflict Resolution Policies:**

```go
type ConflictPolicy string

const (
    FailFast ConflictPolicy = "fail_fast"  // Abort immediately
    Merge    ConflictPolicy = "merge"      // Attempt automatic merge
    Skip     ConflictPolicy = "skip"       // Ignore conflicting patch
)
```

**Example: FailFast**

```go
reconciler := NewReconciler()

delta1 := &Delta{Files: []string{"config.json"}}
delta2 := &Delta{Files: []string{"config.json"}}

_, conflict, err := reconciler.Reconcile(ctx, []*Delta{delta1, delta2})
if err != nil {
    log.Fatalf("Reconciliation failed: %v", err)
}

if !conflict.Resolved {
    log.Printf("Conflict: %v", conflict.Conflicts)
    // Manual resolution required
}
```

---

### Law 4: Determinism

```
∀ Δ. Replay(Δ.ReplayScript, Δ.InputHash) = Δ.OutputHash
```

**Read as:** Every patch is reproducible via its replay script.

**Example:**

```go
receipt := &Receipt{
    InputHash:    "sha256:abc123",
    OutputHash:   "sha256:def456",
    ReplayScript: "#!/bin/bash\ngo test -v\n",
}

// Anyone can verify by running replay script
$ bash replay_script.sh
$ compute_hash(current_state)
sha256:def456  ✓ Matches OutputHash
```

**Why Important?**

- Zero-trust verification
- Auditable claims
- Time-travel debugging

**Proof by Contradiction:**

Assume patch Δ is **non-deterministic**:

```
Run 1: Replay(Δ.ReplayScript) = hash_x
Run 2: Replay(Δ.ReplayScript) = hash_y

Where hash_x ≠ hash_y
```

But Δ.OutputHash is fixed (in receipt).

So either:

- Run 1 fails verification (hash_x ≠ Δ.OutputHash), OR
- Run 2 fails verification (hash_y ≠ Δ.OutputHash)

**Conclusion:** Non-deterministic patches are **unverifiable** (self-refuting).

---

## Composition Operations

Different operations have different composition semantics:

### 1. Append

```
CompositionOp: "append"
```

**Semantics:** Add to end of sequence

**Example:**

```
Δ₁: Append record_a
Δ₂: Append record_b

Δ₁ ⊕ Δ₂: Sequence [record_a, record_b]
```

**Properties:**

- Always succeeds (no conflicts)
- Order matters
- Associative

**Use Case:** Independent operations (logs, events)

---

### 2. Merge

```
CompositionOp: "merge"
```

**Semantics:** Combine overlapping changes intelligently

**Example:**

```
Δ₁: config.json { "port": 8080 }
Δ₂: config.json { "host": "localhost" }

Δ₁ ⊕ Δ₂: config.json { "port": 8080, "host": "localhost" }
```

**Properties:**

- May conflict (requires resolution)
- Commutative (if no conflicts)
- Context-dependent

**Use Case:** Collaborative editing, configuration merges

---

### 3. Replace

```
CompositionOp: "replace"
```

**Semantics:** Overwrite previous state

**Example:**

```
Δ₁: version = "1.0.0"
Δ₂: version = "2.0.0"

Δ₁ ⊕ Δ₂: version = "2.0.0" (Δ₂ wins)
```

**Properties:**

- Order matters
- Not commutative
- Destructive

**Use Case:** Versioned updates, upgrades

---

### 4. Extend

```
CompositionOp: "extend"
```

**Semantics:** Add without overwrite

**Example:**

```
Δ₁: dependencies = ["pkg_a"]
Δ₂: dependencies = ["pkg_b"]

Δ₁ ⊕ Δ₂: dependencies = ["pkg_a", "pkg_b"]
```

**Properties:**

- Always succeeds
- Commutative
- Preserves existing data

**Use Case:** Incremental builds, dependency addition

---

## Multi-Agent Composition

When 10 agents work in parallel:

### Phase 1: Individual Receipts

```
Agent 0: Δ₀ (receipt R₀)
Agent 1: Δ₁ (receipt R₁)
...
Agent 9: Δ₉ (receipt R₉)
```

### Phase 2: Pairwise Validation

Reconciler validates all pairs:

```
ValidateComposition(Δ₀, Δ₁)
ValidateComposition(Δ₀, Δ₂)
...
ValidateComposition(Δ₈, Δ₉)

Total pairs: C(10, 2) = 45
```

### Phase 3: Global Composition

If all pairs valid:

```
Δ_global = Δ₀ ⊕ Δ₁ ⊕ Δ₂ ⊕ ... ⊕ Δ₉
```

**Invariant:** Order doesn't matter (associativity)

```
(Δ₀ ⊕ Δ₁) ⊕ (Δ₂ ⊕ Δ₃) = Δ₀ ⊕ (Δ₁ ⊕ Δ₂) ⊕ Δ₃
```

### Phase 4: Global Receipt

```go
globalReceipt := &Receipt{
    ExecutionID:    "global-001",
    InputHash:      "sha256:initial",
    OutputHash:     hashOf(Δ_global),
    SubReceipts:    []string{R₀, R₁, ..., R₉},
    CompositionOp:  "merge",
    ConflictPolicy: "fail_fast",
}
```

---

## Formal Verification

### Theorem: Composition Preserves Determinism

**Claim:** If Δ₁ and Δ₂ are deterministic, then Δ₁ ⊕ Δ₂ is deterministic.

**Proof:**

1. Δ₁ deterministic ⟹ Replay(Δ₁) always produces same result
2. Δ₂ deterministic ⟹ Replay(Δ₂) always produces same result
3. Composition is defined as sequential application:
   ```
   Δ₁ ⊕ Δ₂ ≡ Apply(Δ₂, Apply(Δ₁, initial_state))
   ```
4. Sequential application of deterministic operations is deterministic
5. Therefore: Δ₁ ⊕ Δ₂ is deterministic ∎

---

### Theorem: Conflict Detection is Sound

**Claim:** If CONFLICT(Δ₁, Δ₂) is triggered, then Δ₁ and Δ₂ modify overlapping files.

**Proof:**

1. Conflict detection rule:
   ```
   CONFLICT(Δ₁, Δ₂) ⟺ Δ₁.files ∩ Δ₂.files ≠ ∅
   ```
2. By definition, if triggered, intersection is non-empty
3. Non-empty intersection means at least one file in common
4. Therefore: Conflict detection is sound ∎

**Corollary:** No false positives (conflict only when necessary)

---

## Real-World Example

### Scenario: 3 Agents Building a Web App

**Agent 1:** Creates backend API

```
Δ₁:
  Files: [api/server.go, api/routes.go]
  CompositionOp: append
```

**Agent 2:** Creates frontend UI

```
Δ₂:
  Files: [ui/index.html, ui/app.js]
  CompositionOp: append
```

**Agent 3:** Creates shared config

```
Δ₃:
  Files: [config.json]
  CompositionOp: merge
```

### Composition Check

```
Δ₁.files ∩ Δ₂.files = ∅  ✓ (disjoint)
Δ₁.files ∩ Δ₃.files = ∅  ✓ (disjoint)
Δ₂.files ∩ Δ₃.files = ∅  ✓ (disjoint)
```

### Composition

```
Δ_final = Δ₁ ⊕ Δ₂ ⊕ Δ₃

Result:
  Files: [
    api/server.go,
    api/routes.go,
    ui/index.html,
    ui/app.js,
    config.json
  ]
```

**Verification:**

```bash
$ kgc-receipt verify-composition --receipts=r1.json,r2.json,r3.json
✓ All pairs compatible
✓ Composition succeeded
✓ Global receipt: gr-webapp-001
```

---

## Edge Cases

### Case 1: Self-Composition

```
Δ ⊕ Δ = ?
```

**Answer:** Δ (by idempotence)

---

### Case 2: Empty Composition

```
Δ ⊕ ∅ = ?
```

**Answer:** Δ (identity element)

---

### Case 3: Conflict Composition

```
Δ₁.files = {config.json}
Δ₂.files = {config.json}

Δ₁ ⊕ Δ₂ = ?
```

**Answer:** CONFLICT (explicit error)

---

### Case 4: Ordered vs Unordered

**Ordered (replace):**

```
Δ₁: version = "1.0"
Δ₂: version = "2.0"

Δ₁ ⊕ Δ₂ ≠ Δ₂ ⊕ Δ₁
(order matters)
```

**Unordered (append disjoint):**

```
Δ₁: Add file_a.go
Δ₂: Add file_b.go

Δ₁ ⊕ Δ₂ = Δ₂ ⊕ Δ₁
(order doesn't matter)
```

---

## Best Practices

### 1. Declare Composition Operations

```go
receipt.CompositionOp = "append"  // Explicit!
```

### 2. Choose Appropriate Conflict Policy

```go
// For critical operations
receipt.ConflictPolicy = "fail_fast"

// For collaborative editing
receipt.ConflictPolicy = "merge"
```

### 3. Validate Before Composition

```go
compatible, reason := reconciler.ValidateComposition(delta1, delta2)
if !compatible {
    log.Fatalf("Incompatible: %s", reason)
}
```

### 4. Test Associativity

```go
func TestAssociativity(t *testing.T) {
    // (Δ₁ ⊕ Δ₂) ⊕ Δ₃
    result1 := compose(compose(d1, d2), d3)

    // Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
    result2 := compose(d1, compose(d2, d3))

    if !equal(result1, result2) {
        t.Errorf("Associativity violated")
    }
}
```

---

## Conclusion

Composition laws provide **mathematical guarantees** for multi-agent collaboration:

- ✅ **Idempotence** - Safe retries
- ✅ **Associativity** - Parallel composition
- ✅ **Conflict Detection** - No silent failures
- ✅ **Determinism** - Reproducible results

**Core Principle:**

> "Patches compose via formal laws, not ad-hoc merging. Conflicts are explicit, not hidden."

## Next Steps

- [Why Determinism Matters](why_determinism.md) - Foundation of composition
- [Receipt Chaining](receipt_chaining.md) - How receipts enable composition
- [How to Run Multi-Agent Demo](../how_to/run_multi_agent_demo.md) - See composition in action

## See Also

- [API Reference](../reference/api.md)
- [Substrate Interfaces](../reference/substrate_interfaces.md)
- [Getting Started Tutorial](../tutorial/getting_started.md)
