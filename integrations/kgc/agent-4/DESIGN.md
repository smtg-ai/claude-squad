# Agent 4: Resource Allocation & Capacity - Design Document

## Mission

Implement a deterministic capacity allocator for agent resource scheduling with fairness guarantees, supporting round-robin, priority-based, and exhaustion scenarios. All scheduling decisions must be deterministic and reproducible.

---

## Formal Specification

### Observable Inputs (O)

```
O = {
    AgentCount    : ℕ           // Number of agents requiring resources
    ResourceBudget: ℕ           // Total available resource units
    Agents        : [Agent]     // Ordered list of agents (deterministic order)
    Tasks         : [Task]      // Ordered list of tasks (deterministic order)
    Priorities    : Task → ℕ    // Priority function mapping tasks to priority levels
}

Agent = {
    ID         : string
    MinResources: ℕ    // Minimum resources needed to function
    MaxResources: ℕ    // Maximum resources this agent can utilize
}

Task = {
    ID          : string
    RequiredResources: ℕ
    Priority    : ℕ    // Higher number = higher priority
}
```

### Transformation (A = μ(O))

```
μ: O → (Allocation ∪ Schedule ∪ FailureReport)

where:

Allocation = {
    Assignments: Agent → ℕ      // Resources assigned per agent
    Remaining  : ℕ              // Unallocated resources
    Fairness   : ℝ              // Gini coefficient (0 = perfect equality)
}

Schedule = {
    TaskOrder  : [TaskAssignment]  // Deterministically ordered task assignments
    AgentLoads : Agent → ℕ         // Tasks assigned to each agent
}

TaskAssignment = {
    TaskID : string
    AgentID: string
    Order  : ℕ                     // Execution order (deterministic)
}

FailureReport = {
    Reason     : string
    RequestedResources: ℕ
    AvailableResources: ℕ
    Deficit    : ℕ
}
```

### Core Operations

#### 1. AllocateResources

```
AllocateResources(agentCount: ℕ, resourceBudget: ℕ) → (Allocation, error)

Algorithm:
  1. Validate: agentCount > 0 ∧ resourceBudget ≥ 0
  2. baseQuota := ⌊resourceBudget / agentCount⌋
  3. remainder := resourceBudget mod agentCount
  4. For i ∈ [0, agentCount):
       if i < remainder:
         allocation[i] := baseQuota + 1
       else:
         allocation[i] := baseQuota
  5. Return Allocation with assignments

Determinism Guarantee:
  ∀ (n, r). AllocateResources(n, r) = AllocateResources(n, r)
  Same inputs always produce identical allocation map
```

#### 2. RoundRobinSchedule

```
RoundRobinSchedule(agents: [Agent], tasks: [Task]) → Schedule

Algorithm:
  1. Sort agents by ID (lexicographic, deterministic)
  2. Sort tasks by ID (lexicographic, deterministic)
  3. assignments := []
  4. For i, task ∈ enumerate(tasks):
       agentIndex := i mod len(agents)
       assignments.append({
         TaskID:  task.ID,
         AgentID: agents[agentIndex].ID,
         Order:   i
       })
  5. Return Schedule with assignments

Determinism Guarantee:
  ∀ A, T. RoundRobinSchedule(A, T) = RoundRobinSchedule(A, T)
  Sorting ensures stable, reproducible assignment order

Fairness Property:
  ∀ a₁, a₂ ∈ agents. |load(a₁) - load(a₂)| ≤ 1
  No agent differs from another by more than 1 task
```

#### 3. PrioritySchedule

```
PrioritySchedule(agents: [Agent], prioritizedTasks: [Task]) → Schedule

Algorithm:
  1. Sort agents by ID (deterministic)
  2. Sort tasks by (Priority DESC, ID ASC) (stable sort, deterministic)
  3. agentLoads := map[AgentID]int (initialized to 0)
  4. assignments := []
  5. For i, task ∈ enumerate(sortedTasks):
       // Select agent with minimum current load
       // Break ties by agent ID (lexicographic)
       selectedAgent := argmin_{a}(agentLoads[a.ID], a.ID)
       assignments.append({
         TaskID:  task.ID,
         AgentID: selectedAgent.ID,
         Order:   i
       })
       agentLoads[selectedAgent.ID]++
  6. Return Schedule with assignments

Determinism Guarantee:
  ∀ A, T. PrioritySchedule(A, T) = PrioritySchedule(A, T)
  Stable sort + deterministic tie-breaking ensures reproducibility

Priority Property:
  ∀ t₁, t₂. Priority(t₁) > Priority(t₂) ⟹ Order(t₁) < Order(t₂)
  Higher priority tasks are always scheduled earlier
```

#### 4. ExhaustionTest

```
ExhaustionTest(resourceLimit: ℕ) → FailureReport

Algorithm:
  1. demand := resourceLimit + 1  // Exceed limit by 1
  2. available := resourceLimit
  3. Return FailureReport{
       Reason: "Resource exhaustion: insufficient capacity",
       RequestedResources: demand,
       AvailableResources: available,
       Deficit: demand - available
     }

Determinism Guarantee:
  ∀ L. ExhaustionTest(L) = ExhaustionTest(L)
  Pure function with no side effects
```

---

## Forbidden States (H)

```
H = {
    h₁: ∃ agent. allocation(agent) < 0                    // Negative allocation
    h₂: ∑ allocation(agent) > resourceBudget              // Over-allocation
    h₃: ∃ task. task not in schedule                      // Dropped task
    h₄: ∃ (t₁, t₂). Order(t₁) = Order(t₂) ∧ t₁ ≠ t₂     // Non-unique ordering
    h₅: Schedule(O₁) ≠ Schedule(O₁)                       // Non-determinism
    h₆: ∃ agent. ¬assigned(agent)                         // Agent not scheduled
}

Guards:
  - All allocations are validated: allocation ≥ 0
  - Sum of allocations never exceeds budget
  - All tasks appear exactly once in schedule
  - Ordering is deterministic and unique
  - Identical inputs produce identical outputs
  - All agents participate in scheduling
```

---

## Proof Targets (Π)

### Π₁: Determinism

**Claim:** All scheduling operations are deterministic.

**Proof Strategy:**
```
Given:
  - agents A₁ = [a₁, a₂, ..., aₙ]
  - tasks T₁ = [t₁, t₂, ..., tₘ]

Prove:
  RoundRobinSchedule(A₁, T₁) = RoundRobinSchedule(A₁, T₁)
  PrioritySchedule(A₁, T₁) = PrioritySchedule(A₁, T₁)

Method:
  1. Run scheduling twice with identical inputs
  2. Compare resulting schedules (deep equality)
  3. Assert: schedule₁ == schedule₂

Test: TestDeterminism (runs each scheduler 10 times, verifies identical output)
```

### Π₂: Fairness (Round-Robin)

**Claim:** Round-robin scheduling distributes tasks fairly.

**Proof Strategy:**
```
Given:
  - agents A = [a₁, a₂, ..., aₙ]
  - tasks T = [t₁, t₂, ..., tₘ]
  - schedule S = RoundRobinSchedule(A, T)

Prove:
  ∀ aᵢ, aⱼ ∈ A. |load(aᵢ) - load(aⱼ)| ≤ 1

Method:
  1. Count tasks assigned to each agent
  2. Compute max_load and min_load
  3. Assert: max_load - min_load ≤ 1

Test: TestRoundRobinFairness
```

### Π₃: Priority Ordering

**Claim:** Priority scheduling respects task priorities.

**Proof Strategy:**
```
Given:
  - tasks T = [t₁, t₂, ..., tₘ] with priorities
  - schedule S = PrioritySchedule(agents, T)

Prove:
  ∀ tᵢ, tⱼ. Priority(tᵢ) > Priority(tⱼ) ⟹ Order(tᵢ) < Order(tⱼ)

Method:
  1. Create tasks with distinct priorities
  2. Run PrioritySchedule
  3. Verify that higher priority tasks come first in schedule order

Test: TestPriorityOrdering
```

### Π₄: Resource Conservation

**Claim:** Allocated resources never exceed budget.

**Proof Strategy:**
```
Given:
  - agentCount n
  - resourceBudget B
  - allocation A = AllocateResources(n, B)

Prove:
  ∑ᵢ A.Assignments[i] = B

Method:
  1. Sum all allocations
  2. Assert: totalAllocated == resourceBudget

Test: TestResourceConservation
```

### Π₅: Exhaustion Detection

**Claim:** Exhaustion is correctly detected and reported.

**Proof Strategy:**
```
Given:
  - resourceLimit L
  - report R = ExhaustionTest(L)

Prove:
  R.Deficit = R.RequestedResources - R.AvailableResources
  R.Deficit > 0

Method:
  1. Run ExhaustionTest
  2. Verify deficit calculation
  3. Assert deficit is positive

Test: TestExhaustionScenarios
```

---

## Type Assumptions (Σ)

```
Σ = {
    AgentID ∈ string                 // Non-empty UTF-8 string
    TaskID  ∈ string                 // Non-empty UTF-8 string
    ℕ       ⊆ int                    // Non-negative integers (Go int type)
    Priority∈ ℕ                      // 0 = lowest, higher = more important

    Allocation.Assignments : map[string]int
    Schedule.TaskOrder     : []TaskAssignment

    // All maps are deterministically serializable
    // All slices maintain insertion order
}

Invariants:
  - Agent IDs are unique within agent list
  - Task IDs are unique within task list
  - All resource counts are non-negative
  - Priority values are non-negative integers
```

---

## Priority Order (Λ)

```
Λ = [
    λ₁: Input validation (fail fast on invalid inputs)
    λ₂: Deterministic sorting (stable sort by ID)
    λ₃: Resource allocation (base quota + remainder distribution)
    λ₄: Task assignment (round-robin or priority-based)
    λ₅: Load balancing (minimize variance across agents)
    λ₆: Schedule construction (ordered task assignments)
    λ₇: Verification (check determinism and fairness properties)
]

Execution Order:
  All operations follow this priority order from λ₁ to λ₇.
  No step can proceed until prior steps complete successfully.
```

---

## Invariants (Q)

```
Q = {
    q₁: Determinism
        ∀ inputs. μ(inputs) = μ(inputs)

    q₂: Resource Conservation
        ∀ allocation. ∑ allocation.Assignments ≤ resourceBudget

    q₃: Fairness (Round-Robin)
        ∀ agents aᵢ, aⱼ. |load(aᵢ) - load(aⱼ)| ≤ 1

    q₄: Priority Preservation
        ∀ tasks tᵢ, tⱼ. Priority(tᵢ) > Priority(tⱼ) ⟹ Order(tᵢ) < Order(tⱼ)

    q₅: Completeness
        ∀ task ∈ tasks. ∃ assignment ∈ schedule. assignment.TaskID = task.ID

    q₆: Uniqueness
        ∀ assignments a₁, a₂. a₁.Order = a₂.Order ⟹ a₁ = a₂

    q₇: Non-Negative Allocations
        ∀ agent. allocation(agent) ≥ 0
}

Preservation:
  Each operation μ preserves all invariants in Q.
  Tests verify invariant preservation across all operations.
```

---

## Algorithm Complexity

| Operation | Time Complexity | Space Complexity |
|-----------|-----------------|------------------|
| AllocateResources | O(n) | O(n) |
| RoundRobinSchedule | O(n log n + m) | O(m) |
| PrioritySchedule | O(n log n + m log m) | O(m) |
| ExhaustionTest | O(1) | O(1) |

Where:
- n = number of agents
- m = number of tasks

---

## Determinism Guarantees

### Sources of Determinism

1. **Stable Sorting**: All sorting uses Go's `sort.SliceStable` with deterministic comparison functions
2. **Lexicographic Ordering**: Agent and task IDs are compared lexicographically (string comparison)
3. **Reproducible Tie-Breaking**: When loads are equal, agents are selected by ID order
4. **No Randomization**: Zero use of `math/rand` or non-deterministic functions
5. **No External State**: All functions are pure (no global state access)

### Determinism Tests

```
TestDeterminism:
  For each scheduling function:
    1. Run 10 times with identical inputs
    2. Collect all outputs
    3. Assert all outputs are deeply equal

TestReplayEquivalence:
  1. Run scheduling with inputs O₁
  2. Record output S₁
  3. Replay with same inputs O₁
  4. Assert S₁ == Replay(O₁)
```

---

## Error Handling

```
Errors = {
    ErrInvalidAgentCount    : agentCount ≤ 0
    ErrNegativeResources    : resourceBudget < 0
    ErrEmptyAgentList       : len(agents) = 0
    ErrEmptyTaskList        : len(tasks) = 0
    ErrDuplicateAgentID     : ∃ i, j. agents[i].ID = agents[j].ID ∧ i ≠ j
    ErrDuplicateTaskID      : ∃ i, j. tasks[i].ID = tasks[j].ID ∧ i ≠ j
}

Error Handling Policy:
  - Fail fast on invalid inputs
  - All errors are explicit (no silent failures)
  - Error messages include context and remediation hints
```

---

## Composition Properties

```
CompositionOp: "extend"
  - Allocations are additive
  - Schedules can be concatenated
  - No destructive updates

ConflictPolicy: "fail_fast"
  - If inputs are invalid, fail immediately
  - If resource exhaustion detected, report and halt
  - No silent degradation
```

---

## Testing Strategy

### Test Categories

1. **Unit Tests**
   - AllocateResources with various agent counts and budgets
   - RoundRobinSchedule with different task/agent ratios
   - PrioritySchedule with various priority distributions
   - ExhaustionTest with edge cases

2. **Property Tests**
   - Determinism (10 runs with same inputs)
   - Fairness (max_load - min_load ≤ 1)
   - Priority ordering (higher priority → earlier order)
   - Resource conservation (sum ≤ budget)

3. **Edge Cases**
   - Zero resources
   - Single agent
   - Single task
   - More tasks than agents
   - Fewer tasks than agents
   - Equal priorities (tie-breaking)

4. **Exhaustion Tests**
   - Exceed resource limit
   - Zero resource limit
   - Negative resource request (error)

---

## Success Criteria

- ✅ All functions compile without errors
- ✅ All tests pass: `go test -v`
- ✅ Determinism verified across 10+ runs
- ✅ Fairness property holds for round-robin
- ✅ Priority ordering respected in priority schedule
- ✅ Resource conservation guaranteed (no over-allocation)
- ✅ Exhaustion scenarios correctly detected
- ✅ RECEIPT.json includes replay script

---

## References

- Charter: `/integrations/kgc/contracts/10_AGENT_SWARM_CHARTER.md`
- Interfaces: `/integrations/kgc/contracts/SUBSTRATE_INTERFACES.md`
- Go Documentation: https://golang.org/doc/

---

**Agent:** 4 (Resource Allocation & Capacity)
**Version:** 0.1.0-alpha
**Status:** Implementation Ready
