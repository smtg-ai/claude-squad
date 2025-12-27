# 10-Agent Concurrent Claude Code Swarm Charter

## Mission

Deliver a deterministic KGC-backed knowledge substrate for multi-agent code generation and verification, integrated with seanchatmangpt/unrdf, with cryptographic-style receipts proving all claims.

---

## Global Constraints (All Agents)

1. **No File Collisions**: Each agent owns a single tranche directory. Do NOT edit files outside your tranche except:
   - Shared contracts in `/integrations/kgc/contracts/` (read-only reference)
   - Final reconciliation step (Agent 0 only)

2. **Determinism First**: Every operation must be reproducible. If a tool produces non-deterministic output, fail loudly.

3. **Receipt-Driven**: Every change you make must generate a RECEIPT.json with:
   - `ExecutionID` (UUID)
   - `InputHash` (SHA256 of all inputs)
   - `OutputHash` (SHA256 of all outputs)
   - `ReplayScript` (bash script that reproduces this exact run)

4. **Design-First**: Before implementation, create DESIGN.md documenting:
   - **O** (observable inputs you assume)
   - **A = μ(O)** (transformation you perform)
   - **H** (forbidden states / guards)
   - **Π** (proof targets and how to verify)
   - **Σ** (type assumptions)
   - **Λ** (priority order of operations)
   - **Q** (invariants you preserve)

5. **Composition Law**: Your patch must declare `CompositionOp` and `ConflictPolicy`:
   - `CompositionOp`: "append" | "merge" | "replace" | "extend"
   - `ConflictPolicy`: "fail_fast" | "merge" | "skip"

6. **Testing Required**: Every feature must have tests. Tests must be deterministic and reproducible.

---

## Agent Assignments

### Agent 0: Coordinator & Reconciler

**Tranche:** `/integrations/kgc/agent-0/`

**Responsibility:**
- Serve as the reconciliation authority for the entire swarm
- Implement the Reconciler interface (see SUBSTRATE_INTERFACES.md)
- Validate that all 9 agent deliverables compose without conflict
- Produce the global RECEIPT.json that proves all patches merge cleanly
- Run the final proof suite (P1-P4)

**Deliverables:**
- `reconciler.go` - Implements `Reconciler` interface
- `reconciler_test.go` - Test conflict detection and composition laws
- `DESIGN.md` - Document the reconciliation algorithm
- `RECEIPT.json` - Global composition proof

**Success Criteria:**
- All 9 agent receipts validate
- All patches compose with zero conflicts
- Global RECEIPT.json includes all sub-receipts
- Final proof command: `cd /home/user/claude-squad && make proof-kgc`

---

### Agent 1: Knowledge Store Core

**Tranche:** `/integrations/kgc/agent-1/`

**Responsibility:**
- Implement KnowledgeStore interface with append-log semantics
- Provide hash-stable snapshots (deterministic canonicalization)
- Support immutable record storage and retrieval
- Implement Replay capability for deterministic reconstruction

**Key Invariants:**
- `Snapshot(O) = Snapshot(O)` (deterministic)
- `Append(x) ⊕ Append(x) = Append(x)` (idempotent)
- All hashes must use SHA256 and be reproducible

**Deliverables:**
- `knowledge_store.go` - KnowledgeStore implementation
- `knowledge_store_test.go` - Determinism + idempotence tests
- `DESIGN.md` - Interface contract and invariant proofs
- `RECEIPT.json` - Build + test artifacts

**Success Criteria:**
- `KnowledgeStore` compiles and passes all tests
- Snapshot hashes match across repeated runs (hash-stable)
- Replay produces identical outputs
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-1 && go test -v`

---

### Agent 2: Receipt Chain & Tamper Detection

**Tranche:** `/integrations/kgc/agent-2/`

**Responsibility:**
- Implement Receipt chaining (before_hash → after_hash)
- Provide cryptographic tamper detection
- Validate receipt integrity
- Support deliberate-tamper tests (proofs of tamper detection)

**Key Operations:**
- `CreateReceipt(before, after, replayScript)` → Receipt
- `VerifyReceipt(receipt) → bool`
- `ChainReceipts(R1, R2) → ChainedReceipt` (verify R1.after_hash == R2.before_hash)

**Deliverables:**
- `receipt.go` - Receipt implementation + chaining
- `receipt_test.go` - Tamper tests + deliberate corruption tests
- `DESIGN.md` - Cryptographic proof targets
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- Receipt chaining validates sequentially
- Deliberate tampering is detected in <1ms
- All receipts are serializable to JSON
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-2 && go test -v`

---

### Agent 3: Policy Pack Bridge (→ unrdf)

**Tranche:** `/integrations/kgc/agent-3/`

**Responsibility:**
- Build thin adapter to load policy packs from seanchatmangpt/unrdf
- Validate KGC operations against loaded policies
- Provide PolicyPackBridge interface (see SUBSTRATE_INTERFACES.md)
- Do NOT deeply couple to unrdf; use loose interface contract

**Integration Points:**
- Discover unrdf repo at `/tmp/unrdf-integration` (or git clone on demand)
- Load policy packs from unrdf structure
- Validate patches against policies
- Return pass/fail + reason

**Deliverables:**
- `policy_bridge.go` - PolicyPackBridge implementation
- `policy_bridge_test.go` - Integration tests (may require unrdf sample)
- `DESIGN.md` - Boundary contract with unrdf
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- PolicyPackBridge compiles and implements interface
- At least one policy pack can be loaded and validated
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-3 && go test -v`
- If unrdf unavailable, provide minimal stub with documented contract

---

### Agent 4: Resource Allocation & Capacity

**Tranche:** `/integrations/kgc/agent-4/`

**Responsibility:**
- Implement deterministic capacity allocator for agent resource scheduling
- Support round-robin, priority-based, and exhaustion scenarios
- Provide fair scheduling guarantees
- All decisions must be deterministic and reproducible

**Key Operations:**
- `AllocateResources(agentCount, resourceBudget) → Allocation`
- `RoundRobinSchedule(agents, tasks) → Schedule`
- `PrioritySchedule(agents, prioritizedTasks) → Schedule`
- `ExhaustionTest(resourceLimit) → FailureReport` (test what happens at limit)

**Deliverables:**
- `capacity_allocator.go` - Allocator implementation
- `capacity_allocator_test.go` - Round-robin, priority, exhaustion tests
- `DESIGN.md` - Scheduling algorithm proofs
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- All scheduling is deterministic (same inputs → same schedule)
- Fairness property: all agents eventually get resources
- Exhaustion tests are reproducible
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-4 && go test -v`

---

### Agent 5: Agent Workspace Isolation (Poka-Yoke)

**Tranche:** `/integrations/kgc/agent-5/`

**Responsibility:**
- Implement per-agent sandboxed work directories
- Enforce declared inputs/outputs contract (no undeclared writes)
- Provide poka-yoke (mistake-proof) isolation guarantees
- Make invalid operations unrepresentable

**Key Concepts:**
- Each agent declares `InputFiles`, `OutputFiles` upfront
- Isolation layer rejects all undeclared I/O
- Tests verify that violations are impossible, not just unlikely

**Deliverables:**
- `workspace_isolator.go` - Isolation implementation
- `workspace_isolator_test.go` - Tests proving denied undeclared writes
- `DESIGN.md` - Poka-yoke design + invariant proofs
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- Undeclared writes are rejected at syscall boundary
- Isolation adds <10ms overhead per operation
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-5 && go test -v`

---

### Agent 6: Task Graph & Routing

**Tranche:** `/integrations/kgc/agent-6/`

**Responsibility:**
- Implement routing based on declared predicates (with bounded cost)
- Support XOR, AND, OR routing conditions
- Provide deterministic task graph evaluation
- Support replay with identical outcomes

**Key Operations:**
- `Route(task, predicates) → NextAgent` (deterministic decision)
- `EvaluateTaskGraph(tasks) → ExecutionOrder` (topologically sorted)
- `ReplayRoute(task, predicates, replayScript) → identical route`

**Deliverables:**
- `task_router.go` - Router implementation
- `task_router_test.go` - XOR/AND/OR routing + replay tests
- `DESIGN.md` - Predicate evaluation + commutativity proofs
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- Routing decisions are deterministic
- XOR/AND/OR combinator tests all pass
- Replay produces identical routing
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-6 && go test -v`

---

### Agent 7: Documentation Scaffolding (Diataxis)

**Tranche:** `/integrations/kgc/agent-7/`

**Responsibility:**
- Create documentation structure using Diataxis framework (Tutorials, How-To, Reference, Explanation)
- Build for KGC substrate as reference documentation
- Include all four documentation types
- Make docs indexing and link validation runnable

**Structure:**
```
agent-7/docs/
├── index.md
├── tutorial/
│   └── getting_started.md
├── how_to/
│   ├── create_knowledge_store.md
│   ├── verify_receipts.md
│   └── run_multi_agent_demo.md
├── reference/
│   ├── substrate_interfaces.md
│   ├── api.md
│   └── cli.md
└── explanation/
    ├── why_determinism.md
    ├── receipt_chaining.md
    └── composition_laws.md
```

**Deliverables:**
- Complete Diataxis doc structure
- `build_docs.sh` - Script to validate links and build index
- `DESIGN.md` - Documentation strategy
- `RECEIPT.json` - Build proof

**Success Criteria:**
- All markdown files are well-formed
- Link validation passes (no broken references)
- Build script runs without errors
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-7 && ./build_docs.sh`

---

### Agent 8: Performance Harness (No Benchmarks)

**Tranche:** `/integrations/kgc/agent-8/`

**Responsibility:**
- Implement deterministic workload harness (NOT performance benchmarks)
- Record timings for regression detection per commit
- Support replay and comparison
- Make timing results reproducible and actionable

**Key Points:**
- DO NOT make absolute performance claims
- ONLY detect regressions vs baseline
- MUST be reproducible and runnable

**Deliverables:**
- `harness.go` - Workload harness implementation
- `harness_test.go` - Deterministic workload tests
- `baseline.json` - Initial timing baseline
- `DESIGN.md` - Harness design + regression detection strategy
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- Harness runs deterministically (same timings across runs)
- Baseline recorded in JSON
- Regression detection logic is sound
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-8 && go test -v -timeout 30s`

---

### Agent 9: End-to-End Demo

**Tranche:** `/integrations/kgc/agent-9/`

**Responsibility:**
- Wire a minimal end-to-end demo that:
  - Spawns 3+ agents running in parallel
  - Each agent emits receipts
  - Reconciler validates all receipts
  - Produces final global receipt
- Make the entire demo runnable in <10 seconds

**Demo Flow:**
```
1. Initialize knowledge store (Agent 1)
2. Create 3+ concurrent tasks
3. Route tasks deterministically (Agent 6)
4. Allocate resources (Agent 4)
5. Each task produces receipt (Agent 2)
6. Reconciler validates all (Agent 0)
7. Print global receipt
```

**Deliverables:**
- `demo.go` - Demo orchestration
- `demo_test.go` - Demo validation tests
- `DESIGN.md` - Demo workflow + proof targets
- `RECEIPT.json` - Execution proof

**Success Criteria:**
- `go run demo.go` completes in <10 seconds
- All 3+ agents produce receipts
- Final global receipt is valid
- Test command: `cd /home/user/claude-squad/integrations/kgc/agent-9 && go run demo.go`

---

## Execution Protocol

### Phase 1: All Agents Start Immediately (Today)
- Each agent works independently on their tranche
- Parallel execution (no sequential dependencies)
- Target time: 2-4 hours per agent

### Phase 2: Generate Proofs
- Each agent runs their tests and harness
- Produces RECEIPT.json with proof artifacts
- Target time: 30 minutes per agent

### Phase 3: Reconciliation (Agent 0)
- Collect all 9 receipts
- Validate composition rules
- Produce global receipt
- Target time: 15 minutes

### Phase 4: Final Verification
- Run all four proof targets (P1-P4)
- Target time: 5 minutes

---

## Proof Targets

### P1: Deterministic Substrate Build
**Command:** `make proof-p1`
**Success:** Build produces identical artifacts across repeated runs (SHA256 hash match)

### P2: Multi-Agent Patch Integrity
**Command:** `make proof-p2`
**Success:** All 10 agent patches reconcile without conflict; no silent partials

### P3: Receipt-Chain Correctness
**Command:** `make proof-p3`
**Success:** Every receipt verifies; chains are unbroken; tampering is detected

### P4: Cross-Repo Integration Contract
**Command:** `make proof-p4`
**Success:** claude-squad can call unrdf policies; boundary is versioned and testable

---

## Definition of Done

1. ✅ All agents produce:
   - DESIGN.md (typed assumptions, invariants, proofs)
   - Code (implementation behind interface)
   - Tests (deterministic, reproducible)
   - RECEIPT.json (proof of execution)

2. ✅ No file collisions (reconciliation succeeds)

3. ✅ All four proof targets pass

4. ✅ One command runs everything: `make proof-kgc`

5. ✅ Branch is pushed to `claude/kgc-knowledge-substrate-8J5na`

---

## Additional Notes

- Use formal notation (O, μ, Π, Σ, Λ, Q, H) in DESIGN.md files
- Keep interfaces minimal and loose-coupled
- Prefer deterministic output over performance
- Receipts are immutable; include them in git commits
- If you encounter blocking issues, create a BLOCKERS.md in your tranche with:
  - Issue description
  - Expected vs actual behavior
  - Suggested remediation (if known)

---

## Contact & Support

If you get stuck:
1. Check SUBSTRATE_INTERFACES.md for reference
2. Look at DESIGN.md files from other agents for patterns
3. Check BLOCKERS.md files for known issues
4. Ask in the agent coordination channel (if available)

---

**Ready to execute. All 10 agents start now.**
