# How to Run Multi-Agent Demo

This guide shows you how to run the end-to-end KGC demo that orchestrates 10 concurrent agents, each producing cryptographic receipts that compose deterministically.

## Problem

You want to see:

- Multiple agents working in parallel
- Deterministic task routing
- Receipt generation and validation
- Conflict-free reconciliation
- Complete end-to-end workflow

## Solution

Run Agent 9's demo that spawns 3+ agents, routes tasks deterministically, and produces a global receipt proving all operations composed correctly.

## Prerequisites

- Completed [Getting Started Tutorial](../tutorial/getting_started.md)
- Go 1.21 or later
- Basic understanding of [receipt chains](verify_receipts.md)

## Quick Start

```bash
# Navigate to demo directory
cd /home/user/claude-squad/integrations/kgc/agent-9

# Run the demo
go run demo.go

# Expected output:
# ✓ Initialized knowledge store
# ✓ Spawned 3 agents
# ✓ Routed 5 tasks
# ✓ All receipts valid
# ✓ Global receipt generated
# Demo completed in 2.3s
```

## What the Demo Does

The demo orchestrates a complete multi-agent workflow:

```
┌─────────────────────────────────────────────────────────┐
│ 1. Initialize Knowledge Store (Agent 1)                │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 2. Create 3+ Concurrent Tasks                          │
│    - Task A: Process data                              │
│    - Task B: Validate schema                           │
│    - Task C: Generate report                           │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 3. Route Tasks Deterministically (Agent 6)             │
│    XOR/AND/OR routing based on predicates              │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 4. Allocate Resources (Agent 4)                        │
│    Round-robin scheduling across agents                │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 5. Execute Tasks in Parallel                           │
│    Each agent produces a receipt                       │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 6. Reconcile All Receipts (Agent 0)                    │
│    Validate composition, detect conflicts              │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 7. Generate Global Receipt                             │
│    Proves all operations composed correctly            │
└─────────────────────────────────────────────────────────┘
```

## Step 1: Understand the Demo Architecture

```go
// Demo workflow
type DemoWorkflow struct {
    KnowledgeStore *agent1.KnowledgeStore
    TaskRouter     *agent6.TaskRouter
    Allocator      *agent4.CapacityAllocator
    Reconciler     *agent0.Reconciler
    Agents         []*DemoAgent
}
```

## Step 2: Run with Verbose Output

```bash
go run demo.go --verbose

# Shows detailed execution:
# [00:00.001] Initializing knowledge store...
# [00:00.050] Knowledge store ready (hash: sha256:abc123...)
# [00:00.051] Spawning agent-1...
# [00:00.052] Spawning agent-2...
# [00:00.053] Spawning agent-3...
# [00:00.100] Routing task-1 → agent-2
# [00:00.101] Routing task-2 → agent-1
# [00:00.102] Routing task-3 → agent-3
# [00:00.500] Agent-2 completed task-1 (receipt: r-001)
# [00:00.501] Agent-1 completed task-2 (receipt: r-002)
# [00:00.502] Agent-3 completed task-3 (receipt: r-003)
# [00:00.600] Reconciling 3 receipts...
# [00:00.650] ✓ All receipts valid
# [00:00.700] Global receipt: gr-demo-20250101-120000
```

## Step 3: Run with Custom Parameters

```bash
# Run with 5 agents and 10 tasks
go run demo.go --agents=5 --tasks=10

# Run with specific routing strategy
go run demo.go --routing=priority

# Run with file-based knowledge store
go run demo.go --storage=file --storage-path=/tmp/demo_store
```

## Step 4: Inspect Generated Receipts

After running the demo, inspect the receipt directory:

```bash
ls -la /tmp/kgc_demo_receipts/

# Output:
# receipt_agent-1_task-2.json
# receipt_agent-2_task-1.json
# receipt_agent-3_task-3.json
# global_receipt.json
```

View a receipt:

```bash
cat /tmp/kgc_demo_receipts/receipt_agent-1_task-2.json

# Output:
{
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_id": "agent-1",
  "timestamp": 1704106800000000000,
  "toolchain_ver": "go1.21.5",
  "input_hash": "sha256:abc123...",
  "output_hash": "sha256:def456...",
  "replay_script": "#!/bin/bash\ngo run task.go --id=task-2\n",
  "composition_op": "append",
  "conflict_policy": "fail_fast",
  "proof_artifacts": {
    "test_log": "all tests passed",
    "execution_time": "500ms"
  }
}
```

## Step 5: Verify Receipt Chain

```bash
go run demo.go --verify-only

# Reads existing receipts and verifies:
# ✓ Receipt r-001 valid
# ✓ Receipt r-002 valid
# ✓ Receipt r-003 valid
# ✓ Chain continuity verified
# ✓ Global receipt valid
```

## Step 6: Test Conflict Detection

Simulate a conflict:

```bash
go run demo.go --inject-conflict

# Expected output:
# ✓ Spawned 3 agents
# ✓ Routed 5 tasks
# ✗ Conflict detected: agent-1 and agent-2 both modified file.txt
# ✓ Reconciler correctly rejected conflicting patches
# Demo completed with conflict detection test: PASS
```

## Step 7: Measure Determinism

Run the demo multiple times and verify identical outputs:

```bash
# Run 3 times, save global receipts
for i in {1..3}; do
  go run demo.go --output=/tmp/receipt_$i.json
done

# Compare hashes
sha256sum /tmp/receipt_*.json

# All hashes should be identical:
# abc123... /tmp/receipt_1.json
# abc123... /tmp/receipt_2.json
# abc123... /tmp/receipt_3.json
```

## Understanding the Output

### Normal Execution

```
✓ Initialized knowledge store (hash: sha256:abc123)
✓ Spawned 3 agents
✓ Routed 5 tasks deterministically
✓ Agent-1: completed 2 tasks (receipt: r-001, r-002)
✓ Agent-2: completed 2 tasks (receipt: r-003, r-004)
✓ Agent-3: completed 1 task (receipt: r-005)
✓ Reconciler validated all receipts
✓ Global receipt generated: gr-demo-abc123
✓ Demo completed in 2.3s
```

### With Verbose Logging

```
[DEBUG] Initializing knowledge store with config: {backend: memory, max_records: 10000}
[DEBUG] Knowledge store initialized (address: 0x12345)
[INFO]  Creating agent pool (count: 3)
[DEBUG] Agent-1 spawned (PID: 1001)
[DEBUG] Agent-2 spawned (PID: 1002)
[DEBUG] Agent-3 spawned (PID: 1003)
[INFO]  Routing 5 tasks using round-robin strategy
[DEBUG] Task-1 routed to Agent-2 (predicate: type=validate)
[DEBUG] Task-2 routed to Agent-1 (predicate: type=process)
...
```

### Conflict Detection

```
✓ Spawned 3 agents
✓ Routed 5 tasks
✗ CONFLICT: agent-1 and agent-2 both modified config.json
  - Agent-1: receipt r-001 (hash: abc123)
  - Agent-2: receipt r-003 (hash: def456)
  - Conflict policy: fail_fast
  - Resolution: REJECTED both patches
✓ Reconciler test: PASS (conflict correctly detected)
```

## Advanced Usage

### Custom Task Definitions

```bash
# Create custom tasks
cat > tasks.json <<EOF
{
  "tasks": [
    {"id": "t1", "type": "validate", "priority": 1},
    {"id": "t2", "type": "process", "priority": 2},
    {"id": "t3", "type": "report", "priority": 1}
  ]
}
EOF

go run demo.go --tasks-file=tasks.json
```

### Performance Profiling

```bash
# Run with CPU profiling
go run demo.go --cpuprofile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

### Integration Testing

```bash
# Run demo as part of CI/CD
go run demo.go --ci-mode

# Exits with code 0 on success, non-zero on failure
# Produces machine-readable output:
{
  "status": "success",
  "agents": 3,
  "tasks": 5,
  "receipts_generated": 5,
  "conflicts": 0,
  "execution_time_ms": 2300,
  "global_receipt": "gr-demo-abc123"
}
```

## Troubleshooting

### Demo Fails with "Knowledge Store Not Found"

Ensure Agent 1's code is built:

```bash
cd /home/user/claude-squad/integrations/kgc/agent-1
go build
```

### Receipts Not Generated

Check write permissions:

```bash
mkdir -p /tmp/kgc_demo_receipts
chmod 755 /tmp/kgc_demo_receipts
```

### Non-Deterministic Output

This indicates a bug. Check for:

- Random number generation without seeding
- Timestamp usage in hashes
- Map iteration without sorted keys
- Network/filesystem dependencies

### Conflict Detection Not Working

Verify reconciler is enabled:

```bash
go run demo.go --reconciler=strict
```

## Performance Expectations

Typical performance on modern hardware:

| Metric | Expected Value |
|--------|---------------|
| Demo startup | < 100ms |
| Per-agent spawn | < 50ms |
| Task routing | < 1ms per task |
| Receipt generation | < 10ms per receipt |
| Reconciliation | < 50ms for 10 receipts |
| Total demo time | < 10 seconds |

## Validation Checklist

After running the demo, verify:

- [ ] All agents spawned successfully
- [ ] All tasks routed deterministically
- [ ] All receipts generated with valid hashes
- [ ] Receipt chain continuity intact
- [ ] Reconciler validated all patches
- [ ] Global receipt produced
- [ ] No conflicts detected (unless injected)
- [ ] Demo completed in < 10 seconds

## Next Steps

- [Create a Knowledge Store](create_knowledge_store.md) - Build production stores
- [Verify Receipts](verify_receipts.md) - Deep dive into receipt validation
- [API Reference](../reference/api.md) - Explore the full API

## See Also

- [Getting Started Tutorial](../tutorial/getting_started.md)
- [Composition Laws](../explanation/composition_laws.md)
- [Receipt Chaining](../explanation/receipt_chaining.md)
- [Why Determinism Matters](../explanation/why_determinism.md)
