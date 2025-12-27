# Agent 0: Reconciler & Coordinator - Design Document

## Overview

Agent 0 serves as the composition authority for the KGC knowledge substrate swarm. It validates that all 9 agent patches (Δ₁...Δ₉) compose without conflict and produces either a unified final state or a detailed conflict report.

---

## Formal Specification

### O (Observable Inputs)

The reconciler operates on observable inputs from all 9 agents:

```
O = {
    Δ₁, Δ₂, ..., Δ₉  : Set of agent deltas (patches)
    R₁, R₂, ..., R₉  : Set of agent receipts
}

where each Δᵢ contains:
    - AgentID        : string (identifier: "agent-1" through "agent-9")
    - Files          : map[string]FileChange  (file path → modification)
    - InputHash      : string (SHA256 of input state)
    - OutputHash     : string (SHA256 of output state)
    - CompositionOp  : "append" | "merge" | "replace" | "extend"
    - ConflictPolicy : "fail_fast" | "merge" | "skip"
    - Receipt        : Receipt (proof of execution)
```

### A = μ(O) (Transformation)

The reconciliation transformation μ : [Δ] → (Δ_final ⊕ ConflictReport):

```
μ(O) = {
    1. ValidateInputs(Δ₁...Δ₉)
       → Ensure all deltas are well-formed and have valid receipts

    2. DetectConflicts(Δ₁...Δ₉)
       → ∀ i,j. (Δᵢ.files ∩ Δⱼ.files ≠ ∅) ⟹ CONFLICT
       → Build conflict graph G = (V, E) where:
          - V = {Δ₁...Δ₉}
          - E = {(Δᵢ, Δⱼ) | Δᵢ.files ∩ Δⱼ.files ≠ ∅}

    3. ValidateCompositionLaws(Δ₁...Δ₉)
       → Law 1 (Idempotence): Δᵢ ⊕ Δᵢ = Δᵢ
       → Law 2 (Associativity): (Δ₁ ⊕ Δ₂) ⊕ Δ₃ = Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
       → Law 3 (Conflict Detection): explicit collision → explicit error
       → Law 4 (Determinism): Replay(Δᵢ) produces identical output

    4. ComposeFinal(Δ₁...Δ₉)
       → If no conflicts: Δ_final = ⊕(Δ₁, Δ₂, ..., Δ₉)
       → If conflicts: ConflictReport = details of all collisions
}
```

### H (Forbidden States / Guards)

The reconciler enforces strict guards against invalid states:

```
H = {
    H₁: PARTIAL_APPLICATION
        → MUST NOT apply subset of deltas silently
        → Either all patches reconcile OR full failure with report

    H₂: SILENT_CONFLICTS
        → MUST NOT merge overlapping file edits without explicit policy
        → All conflicts must be reported deterministically

    H₃: NON_DETERMINISTIC_ORDER
        → MUST NOT allow patch order to affect final state
        → Composition must be commutative for disjoint patches

    H₄: INVALID_RECEIPTS
        → MUST NOT accept deltas without valid receipts
        → Every delta must have InputHash, OutputHash, ReplayScript

    H₅: UNVERIFIABLE_CLAIMS
        → MUST NOT produce final delta without proof artifacts
        → Global receipt must include all sub-receipts for replay
}
```

### Π (Proof Targets)

The reconciler must demonstrate the following properties:

```
Π₁: Conflict Detection Soundness
    ∀ Δᵢ, Δⱼ. (Δᵢ.files ∩ Δⱼ.files ≠ ∅) ⟹ Reconcile(Δᵢ, Δⱼ) = CONFLICT
    Proof: Test with deliberately overlapping file edits
    Test: TestConflictDetection_OverlappingFiles

Π₂: Idempotence Law
    ∀ Δ. Reconcile([Δ, Δ]) = Reconcile([Δ])
    Proof: Applying same patch twice yields identical result
    Test: TestCompositionLaws_Idempotence

Π₃: Associativity Law (for disjoint patches)
    ∀ Δ₁, Δ₂, Δ₃. Disjoint(Δ₁, Δ₂, Δ₃) ⟹
        Reconcile([Δ₁, Δ₂, Δ₃]) = Reconcile([Reconcile([Δ₁, Δ₂]), Δ₃])
    Proof: Order-independent composition for non-overlapping patches
    Test: TestCompositionLaws_Associativity

Π₄: Complete Reconciliation
    ∀ [Δ]. Reconcile([Δ]) produces either:
        - (Δ_final, nil, nil)       : successful composition
        - (nil, ConflictReport, nil) : explicit conflict report
        - (nil, nil, error)          : system error
    Never: (partial_Δ, _, _) or silent failures
    Test: TestReconcile_AllOrNothing

Π₅: Deterministic Conflict Reports
    ∀ [Δ]. Reconcile([Δ]) produces identical reports across runs
    Proof: Conflict detection is hash-stable and reproducible
    Test: TestReconcile_DeterministicConflicts
```

### Σ (Type Assumptions)

```go
// Delta represents a patch from a single agent
type Delta struct {
    AgentID        string                 // "agent-1" through "agent-9"
    Files          map[string]FileChange  // file path → change
    InputHash      string                 // SHA256 of input state
    OutputHash     string                 // SHA256 of output state
    CompositionOp  string                 // "append" | "merge" | "replace" | "extend"
    ConflictPolicy string                 // "fail_fast" | "merge" | "skip"
    Receipt        *Receipt               // execution proof
}

// FileChange represents a modification to a single file
type FileChange struct {
    Path      string   // absolute path
    Operation string   // "create" | "modify" | "delete"
    ContentHash string // SHA256 of new content (if applicable)
}

// ConflictReport details all detected conflicts
type ConflictReport struct {
    Conflicts      []Conflict         // list of all conflicts
    ConflictGraph  map[string][]string // adjacency list of conflicting agents
    Resolution     string              // "manual_required" | "auto_merge" | "abort"
}

// Conflict represents a single file collision between two agents
type Conflict struct {
    File    string // conflicting file path
    Agent1  string // first agent ID
    Agent2  string // second agent ID
    Reason  string // human-readable explanation
}

// Receipt (from Agent 2 interface)
type Receipt struct {
    ExecutionID    string            // UUID
    AgentID        string            // which agent
    Timestamp      int64             // Unix nanoseconds
    InputHash      string            // SHA256(inputs)
    OutputHash     string            // SHA256(outputs)
    ReplayScript   string            // bash reproduction
    CompositionOp  string            // merge semantics
    ConflictPolicy string            // conflict handling
}
```

### Λ (Priority Order of Operations)

Reconciliation proceeds in strict priority order:

```
Λ₁: INPUT_VALIDATION (priority 1)
    - Verify all deltas are well-formed
    - Ensure all receipts are present and valid
    - Check InputHash and OutputHash are non-empty
    - Fail fast if any delta is malformed

Λ₂: CONFLICT_DETECTION (priority 2)
    - Build file ownership map: file → [agents]
    - Identify all overlapping file edits
    - Construct conflict graph
    - Exit early if conflicts found and policy = "fail_fast"

Λ₃: COMPOSITION_LAW_VALIDATION (priority 3)
    - Test idempotence: Δ ⊕ Δ = Δ
    - Test associativity for disjoint patches
    - Verify determinism: same inputs → same conflicts

Λ₄: FINAL_COMPOSITION (priority 4)
    - If no conflicts: merge all deltas into Δ_final
    - Compute global InputHash (hash of all Δᵢ.InputHash)
    - Compute global OutputHash (hash of all Δᵢ.OutputHash)
    - Produce global receipt with all sub-receipts

Λ₅: PROOF_GENERATION (priority 5)
    - Generate ReplayScript for entire reconciliation
    - Include all sub-receipts in global receipt
    - Write RECEIPT.json with complete proof chain
```

### Q (Invariants Preserved)

The reconciler maintains the following invariants:

```
Q₁: TOTALITY
    ∀ inputs. Reconcile(inputs) terminates with definitive result
    - Never hangs, never partial results

Q₂: CONFLICT_SOUNDNESS
    ConflictReport ≠ nil ⟹ ∃ i,j. Δᵢ.files ∩ Δⱼ.files ≠ ∅
    - No false positive conflicts

Q₃: CONFLICT_COMPLETENESS
    (∃ i,j. Δᵢ.files ∩ Δⱼ.files ≠ ∅) ⟹ ConflictReport ≠ nil
    - No missed conflicts

Q₄: DETERMINISM
    ∀ [Δ]. Hash(Reconcile([Δ])) is stable across runs
    - Same inputs always produce same output

Q₅: RECEIPT_CHAIN_INTEGRITY
    Global_Receipt.InputHash = Hash(Δ₁.InputHash, ..., Δ₉.InputHash)
    Global_Receipt.OutputHash = Hash(Δ₁.OutputHash, ..., Δ₉.OutputHash)
    - Receipt chain is cryptographically verifiable
```

---

## Algorithm Details

### Conflict Detection Algorithm

```
DetectConflicts([Δ₁...Δ₉]) → ConflictReport | nil:
    1. fileOwners := map[string][]string{}  // file → [agentIDs]
    2. FOR each Δᵢ in deltas:
         FOR each file in Δᵢ.Files:
           fileOwners[file].append(Δᵢ.AgentID)

    3. conflicts := []
    4. FOR each file, owners in fileOwners:
         IF len(owners) > 1:
           FOR each pair (agent_i, agent_j) in owners:
             conflicts.append(Conflict{file, agent_i, agent_j, "overlapping edit"})

    5. IF len(conflicts) > 0:
         RETURN ConflictReport{conflicts, buildGraph(conflicts), "manual_required"}
       ELSE:
         RETURN nil
```

### Composition Algorithm (for disjoint patches)

```
Compose([Δ₁...Δ₉]) → Δ_final:
    1. Δ_final := EmptyDelta()
    2. Δ_final.AgentID = "agent-0-global"

    3. FOR each Δᵢ in deltas:
         FOR each file, change in Δᵢ.Files:
           Δ_final.Files[file] = change  // disjoint, so no collision

    4. Δ_final.InputHash = Hash(Δ₁.InputHash, ..., Δ₉.InputHash)
    5. Δ_final.OutputHash = Hash(Δ₁.OutputHash, ..., Δ₉.OutputHash)
    6. Δ_final.CompositionOp = "merge"
    7. Δ_final.ConflictPolicy = "fail_fast"

    8. RETURN Δ_final
```

---

## Test Strategy

### Test Categories

1. **Unit Tests** (`reconciler_test.go`)
   - `TestValidateComposition_DisjointPatches`: Two patches with no overlap → compatible
   - `TestValidateComposition_OverlappingPatches`: Overlapping files → incompatible
   - `TestReconcile_EmptyDeltas`: Empty input → empty final delta
   - `TestReconcile_SingleDelta`: One delta → identical output
   - `TestReconcile_DisjointDeltas`: Multiple non-overlapping → merged
   - `TestReconcile_ConflictingDeltas`: Overlapping edits → conflict report
   - `TestCompositionLaws_Idempotence`: Δ ⊕ Δ = Δ
   - `TestCompositionLaws_Associativity`: Order independence for disjoint
   - `TestReconcile_Deterministic`: Same input → same conflict report

2. **Integration Tests**
   - `TestReconcile_All9Agents`: Simulate 9 disjoint agents → success
   - `TestReconcile_All9Agents_WithConflict`: Simulate conflict → detailed report

3. **Property Tests**
   - Determinism: Run reconciliation 100 times → identical hashes
   - Completeness: All overlaps detected
   - Soundness: No false conflicts

---

## Success Criteria

✅ **Compilation**: `go build ./integrations/kgc/agent-0` succeeds
✅ **Tests Pass**: `go test ./integrations/kgc/agent-0 -v` all green
✅ **Conflict Detection**: Overlapping files always trigger explicit conflict
✅ **Composition Laws**: Idempotence and associativity verified
✅ **Determinism**: Repeated runs produce identical conflict reports
✅ **Receipt Generation**: RECEIPT.json includes ReplayScript and proof artifacts

---

## Timeline

- **Design**: 15 minutes (this document)
- **Implementation**: 30 minutes (reconciler.go)
- **Testing**: 15 minutes (reconciler_test.go)
- **Receipt Generation**: 10 minutes (RECEIPT.json)
- **Total**: 70 minutes (target: 60 minutes)

---

## References

- `/home/user/claude-squad/integrations/kgc/contracts/10_AGENT_SWARM_CHARTER.md`
- `/home/user/claude-squad/integrations/kgc/contracts/SUBSTRATE_INTERFACES.md`
- Composition Laws: Idempotence, Associativity, Conflict Detection, Determinism

---

**Agent**: 0 (Reconciler & Coordinator)
**Status**: Design Complete
**Next**: Implementation (reconciler.go)
