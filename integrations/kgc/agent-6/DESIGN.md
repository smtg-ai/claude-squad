# Agent 6: Task Graph & Routing - Formal Design

## Overview

This module implements deterministic task routing based on predicates with bounded evaluation cost. All routing decisions are reproducible and support replay semantics.

---

## Formal Specification

### O (Observable Inputs)

```
O = (T, P, G)

where:
  T = Task = {
    ID:       string
    Type:     string
    Priority: int
    Metadata: map[string]interface{}
  }

  P = Predicate = T → bool

  G = TaskGraph = {
    Tasks:        []Task
    Dependencies: map[TaskID][]TaskID
  }
```

**Input Assumptions:**
- Task IDs are unique within a graph
- Predicate evaluation is deterministic (same T → same bool)
- Dependencies form a DAG (no cycles)
- All predicates have O(1) evaluation cost (bounded)

---

### A = μ(O) (Transformation)

```
μ: (T, P) → AgentID

Route(task T, predicates []Predicate) → (AgentID, error)
  ├─ Evaluate predicates in lexicographic order (deterministic)
  ├─ Apply routing combinator (XOR, AND, OR)
  ├─ Return deterministic agent assignment
  └─ Error if no route matches

μ_graph: G → []TaskID

EvaluateTaskGraph(graph G) → (ExecutionOrder []TaskID, error)
  ├─ Perform topological sort on DAG
  ├─ Preserve deterministic ordering (stable sort on task IDs)
  ├─ Return execution order
  └─ Error if cycle detected

μ_replay: (T, P, ReplayScript) → AgentID

ReplayRoute(task T, predicates []Predicate, script string) → (AgentID, error)
  ├─ Parse replay script to extract previous decision
  ├─ Re-evaluate Route(T, P)
  ├─ Verify: replay_result == original_result
  └─ Error if mismatch (non-determinism detected)
```

**Transformation Properties:**
1. **Determinism**: ∀ T, P. μ(T, P) = μ(T, P) (same inputs → same output)
2. **Commutativity**: Predicate order is fixed (lexicographic), so evaluation is commutative
3. **Idempotence**: ∀ T, P. μ(μ(T, P)) = μ(T, P) (routing is stable)

---

### Σ (Type Assumptions)

```go
type Task struct {
    ID       string                 // Unique identifier
    Type     string                 // Task type (e.g., "build", "test", "deploy")
    Priority int                    // Higher = more urgent
    Metadata map[string]interface{} // Arbitrary key-value data
}

type Predicate func(task *Task) bool

type RoutingCombinator int
const (
    XOR RoutingCombinator = iota  // Exactly one predicate must match
    AND                           // All predicates must match
    OR                            // At least one predicate must match
)

type Route struct {
    Predicates  []Predicate
    Combinator  RoutingCombinator
    TargetAgent string  // AgentID if this route matches
}

type TaskGraph struct {
    Tasks        []*Task
    Dependencies map[string][]string  // TaskID → []DependsOnTaskID
}

type Router struct {
    routes []Route  // Ordered list of routes
}
```

**Type Invariants:**
- Task.ID is non-empty
- Predicates are pure functions (no side effects)
- TaskGraph.Dependencies references only valid task IDs
- Route.TargetAgent is non-empty

---

### Λ (Priority Order of Operations)

**Routing Evaluation Order:**
1. Sort predicates lexicographically by function pointer address (deterministic)
2. Evaluate predicates in sorted order
3. Apply combinator logic (XOR, AND, OR)
4. Return first matching route
5. If no routes match, return error

**Task Graph Evaluation Order:**
1. Validate DAG (detect cycles using DFS)
2. Perform topological sort via Kahn's algorithm
3. Break ties by lexicographic task ID (stable sort)
4. Return ordered task list

**Replay Order:**
1. Parse replay script to extract previous routing decision
2. Re-execute Route() with same inputs
3. Compare results
4. Return error if mismatch

---

### Q (Invariants Preserved)

**I1: Determinism Invariant**
```
∀ T, P, t₁, t₂.
  (T, P evaluated at t₁) = (T, P evaluated at t₂)
  ⟹ Route(T, P) at t₁ == Route(T, P) at t₂
```

**I2: DAG Invariant**
```
∀ G ∈ TaskGraph.
  EvaluateTaskGraph(G) succeeds
  ⟹ ¬∃ cycle in G.Dependencies
```

**I3: Replay Invariant**
```
∀ T, P, script.
  script = ReplayScript(Route(T, P))
  ⟹ ReplayRoute(T, P, script) = Route(T, P)
```

**I4: Bounded Evaluation Cost**
```
∀ P ∈ Predicates.
  eval_time(P) ∈ O(1)
```
Each predicate must evaluate in constant time.

**I5: XOR Exclusivity**
```
∀ routes with combinator=XOR.
  count(predicates evaluating to true) ∈ {0, 1}
```
For XOR routes, at most one predicate path should match.

---

### H (Forbidden States / Guards)

**H1: Cyclic Dependencies**
```
FORBIDDEN: ∃ cycle in TaskGraph.Dependencies
GUARD: Detect cycles during EvaluateTaskGraph(); return error
```

**H2: Non-Deterministic Predicates**
```
FORBIDDEN: Predicate(T) at t₁ ≠ Predicate(T) at t₂
GUARD: Predicates must be pure functions; enforce via testing
```

**H3: Unbounded Evaluation**
```
FORBIDDEN: Predicate evaluation time → ∞
GUARD: All predicates must be O(1); enforce via code review
```

**H4: Ambiguous Routing**
```
FORBIDDEN: Multiple routes match with equal priority
GUARD: Routes are evaluated in order; first match wins
```

**H5: Missing Route**
```
FORBIDDEN: No route matches task
GUARD: Return explicit error; do not fail silently
```

---

### Π (Proof Targets)

**Π1: Deterministic Routing**
```
Claim: Route(T, P) is deterministic
Proof:
  1. Predicates are pure functions (no side effects)
  2. Evaluation order is fixed (lexicographic)
  3. Combinator logic is deterministic (XOR/AND/OR)
  4. First-match wins (no ambiguity)
  ∴ Same inputs always produce same output

Test: Run Route(T, P) 1000 times; assert all results identical
```

**Π2: DAG Topological Sort**
```
Claim: EvaluateTaskGraph(G) produces valid execution order
Proof:
  1. Kahn's algorithm guarantees topological ordering
  2. Tie-breaking by task ID ensures determinism
  3. Cycle detection prevents invalid graphs
  ∴ Output is a valid execution order

Test: Verify ∀ (A → B) ∈ Dependencies, index(A) < index(B) in result
```

**Π3: Replay Consistency**
```
Claim: ReplayRoute(T, P, script) = Route(T, P)
Proof:
  1. ReplayScript captures original decision
  2. Route re-evaluates with same inputs
  3. Comparison detects any divergence
  ∴ Replay produces identical result or errors

Test: Generate script from Route(); verify ReplayRoute() matches
```

**Π4: XOR Exclusivity**
```
Claim: XOR routes match exactly one predicate
Proof:
  1. Evaluate all predicates
  2. Count true results
  3. Match succeeds iff count == 1
  ∴ Exactly one predicate must be true

Test: XOR with [false, true, false] → success
      XOR with [true, true] → failure
      XOR with [false, false] → failure
```

**Π5: AND Conjunction**
```
Claim: AND routes match only if all predicates are true
Proof:
  1. Evaluate all predicates
  2. Match succeeds iff ∀ p. p(T) = true
  ∴ All predicates must be true

Test: AND with [true, true, true] → success
      AND with [true, false, true] → failure
```

**Π6: OR Disjunction**
```
Claim: OR routes match if at least one predicate is true
Proof:
  1. Evaluate all predicates
  2. Match succeeds iff ∃ p. p(T) = true
  ∴ At least one predicate must be true

Test: OR with [false, true, false] → success
      OR with [false, false, false] → failure
```

**Π7: Bounded Evaluation Cost**
```
Claim: Route evaluation is O(n) where n = number of predicates
Proof:
  1. Each predicate is O(1)
  2. Evaluate at most n predicates
  3. Combinator logic is O(n)
  ∴ Total cost is O(n)

Test: Measure evaluation time; assert linear relationship with n
```

---

## Implementation Strategy

### Phase 1: Core Routing Logic
1. Define Task, Predicate, Route types
2. Implement Route() with XOR, AND, OR combinators
3. Ensure deterministic predicate evaluation order

### Phase 2: Task Graph Evaluation
1. Implement topological sort (Kahn's algorithm)
2. Add cycle detection
3. Ensure stable ordering (tie-breaking by task ID)

### Phase 3: Replay Support
1. Generate ReplayScript format
2. Implement ReplayRoute() with verification
3. Test replay consistency

### Phase 4: Testing
1. Unit tests for each combinator (XOR, AND, OR)
2. Determinism tests (1000 runs, identical results)
3. Task graph tests (valid order, cycle detection)
4. Replay tests (script generation + verification)

---

## Replay Script Format

```bash
#!/bin/bash
# KGC Agent-6 Task Router Replay Script
# Execution ID: <UUID>
# Timestamp: <Unix Nanoseconds>

# Original Inputs
TASK_ID="<task_id>"
TASK_TYPE="<task_type>"
TASK_PRIORITY=<priority>

# Original Decision
ROUTED_TO_AGENT="<agent_id>"

# Verification
# Re-run routing with same inputs and verify result matches
```

---

## Error Handling

| Error Condition | Handling Strategy |
|----------------|-------------------|
| No route matches | Return error with task details |
| Cyclic dependencies | Return error with cycle path |
| XOR multiple matches | Return error with matched predicates |
| Invalid task graph | Return error with validation details |
| Replay mismatch | Return error showing divergence |

---

## Performance Characteristics

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Route(T, P) | O(n) where n = len(P) | O(1) |
| EvaluateTaskGraph(G) | O(V + E) where V=tasks, E=edges | O(V) |
| ReplayRoute(T, P, S) | O(n) | O(1) |

All operations are bounded and deterministic.

---

## Composition Law

**CompositionOp:** `extend`
- Agent 6 routing logic extends the substrate; does not conflict with other agents
- Routes are self-contained and do not modify shared state

**ConflictPolicy:** `fail_fast`
- If routing encounters an error, fail immediately
- Do not attempt to merge or skip errors

---

## Verification Checklist

- [ ] Route() produces identical results across 1000 runs
- [ ] XOR routes match exactly one predicate
- [ ] AND routes match only when all predicates are true
- [ ] OR routes match when at least one predicate is true
- [ ] Task graph topological sort is valid
- [ ] Cycle detection works correctly
- [ ] Replay produces identical routing decisions
- [ ] All operations are O(n) or better
- [ ] Tests are deterministic and reproducible

---

## References

- Charter: `/integrations/kgc/contracts/10_AGENT_SWARM_CHARTER.md`
- Interfaces: `/integrations/kgc/contracts/SUBSTRATE_INTERFACES.md`
- Kahn's Algorithm: https://en.wikipedia.org/wiki/Topological_sorting

---

**Agent:** 6 (Task Graph & Routing)
**Status:** Design Complete
**Next:** Implementation
