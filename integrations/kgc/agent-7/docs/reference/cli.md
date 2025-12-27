# CLI Reference

Command-line tools and utilities for the KGC Knowledge Substrate.

## Overview

The KGC substrate provides CLI tools for:

- Running proof targets
- Validating receipts
- Inspecting knowledge stores
- Running demos
- Debugging issues

## Installation

```bash
# Clone repository
git clone https://github.com/seanchatmangpt/claude-squad.git
cd claude-squad/integrations/kgc

# Build all agents
make build

# Install CLI tools
make install
```

---

## Global Commands

### make proof-kgc

Runs all four proof targets (P1-P4) in sequence.

```bash
make proof-kgc
```

**Output:**

```
Running Proof P1: Deterministic substrate build...
✓ P1 PASS: Build produces identical artifacts

Running Proof P2: Multi-agent patch integrity...
✓ P2 PASS: All 10 agent patches reconcile without conflict

Running Proof P3: Receipt-chain correctness...
✓ P3 PASS: All receipts verify, chains unbroken

Running Proof P4: Cross-repo integration contract...
✓ P4 PASS: claude-squad ↔ unrdf integration verified

All proofs passed: 4/4
```

**Exit Codes:**
- `0` - All proofs passed
- `1` - One or more proofs failed

---

## Proof Commands

### make proof-p1

**Proof:** Deterministic substrate build

Verifies that building the substrate multiple times produces identical artifacts.

```bash
make proof-p1
```

**What it does:**

1. Build substrate twice
2. Compute SHA256 of all binaries
3. Compare hashes

**Success criteria:** Hashes are identical

**Output:**

```
Building substrate (run 1)...
Hash: sha256:abc123def456...

Building substrate (run 2)...
Hash: sha256:abc123def456...

✓ Hashes match: Build is deterministic
```

---

### make proof-p2

**Proof:** Multi-agent patch integrity

Validates that all 10 agent patches compose without conflicts.

```bash
make proof-p2
```

**What it does:**

1. Collect all agent receipts
2. Validate each receipt
3. Check for file collisions
4. Verify composition laws

**Success criteria:** Zero conflicts, all compositions valid

**Output:**

```
Validating agent-0 receipt... ✓
Validating agent-1 receipt... ✓
...
Validating agent-9 receipt... ✓

Checking compositions:
  agent-0 ⊕ agent-1... ✓
  agent-0 ⊕ agent-2... ✓
  ...

✓ All 45 composition pairs valid
✓ Zero file collisions detected
```

---

### make proof-p3

**Proof:** Receipt-chain correctness

Verifies all receipts and chains are valid.

```bash
make proof-p3
```

**What it does:**

1. Load all receipts
2. Verify each receipt independently
3. Validate chain continuity
4. Test deliberate tampering detection

**Success criteria:** All receipts valid, chains intact, tampering detected

**Output:**

```
Loading receipts... 10 found

Verifying receipts:
  receipt-agent-0... ✓
  receipt-agent-1... ✓
  ...

Validating chains:
  r0 → r1... ✓ (output_hash matches input_hash)
  r1 → r2... ✓
  ...

Testing tamper detection:
  Injecting corruption... ✓ Detected in <1ms

✓ All receipts valid
✓ Chain continuity intact
✓ Tamper detection working
```

---

### make proof-p4

**Proof:** Cross-repo integration contract

Validates integration between claude-squad and seanchatmangpt/unrdf.

```bash
make proof-p4
```

**What it does:**

1. Check unrdf repository available
2. Load policy pack
3. Validate KGC operations against policies
4. Measure integration latency

**Success criteria:** Policies load, validation succeeds, latency <100ms

**Output:**

```
Checking unrdf repo... ✓ Found at /tmp/unrdf-integration

Loading policy pack "kgc-validation-v1"... ✓
  - 12 rules loaded
  - Version: 1.0.0

Validating operations:
  KnowledgeStore.Append... ✓ Pass
  Receipt.Create... ✓ Pass
  Reconciler.Reconcile... ✓ Pass

Latency: 87ms (target: <100ms)

✓ Integration contract valid
```

---

## Agent-Specific Commands

### Agent 1: Knowledge Store

#### kgc-store create

Creates a new knowledge store.

```bash
kgc-store create --backend=file --path=/var/lib/kgc/store.db
```

**Options:**
- `--backend` - Backend type (`memory` | `file`) [default: `memory`]
- `--path` - Storage path (required if backend=file)
- `--max-records` - Maximum records [default: `100000`]
- `--deterministic` - Enforce determinism [default: `true`]

**Output:**

```
Creating knowledge store...
Backend: file
Path: /var/lib/kgc/store.db
Max records: 100000

✓ Store created successfully
Store ID: ks-abc123
```

---

#### kgc-store append

Appends a record to the store.

```bash
kgc-store append --key="user:123" --value="alice@example.com"
```

**Options:**
- `--key` - Record key (required)
- `--value` - Record value (required)
- `--store-id` - Store ID [default: use default store]

**Output:**

```
Appending record...
Key: user:123
Value: alice@example.com

✓ Record appended
Hash: sha256:abc123def456...
```

---

#### kgc-store snapshot

Takes a snapshot of the store.

```bash
kgc-store snapshot --output=/tmp/snapshot.bin
```

**Options:**
- `--store-id` - Store ID [default: use default store]
- `--output` - Output file path [default: stdout]

**Output:**

```
Taking snapshot...

✓ Snapshot created
Hash: sha256:abc123def456...
Size: 1024 bytes
Output: /tmp/snapshot.bin
```

---

#### kgc-store verify

Verifies a snapshot hash.

```bash
kgc-store verify --hash=sha256:abc123def456...
```

**Options:**
- `--hash` - Expected hash (required)
- `--store-id` - Store ID [default: use default store]

**Output:**

```
Verifying snapshot...
Expected: sha256:abc123def456...
Actual:   sha256:abc123def456...

✓ Verification successful
```

---

### Agent 2: Receipt

#### kgc-receipt create

Creates a new receipt.

```bash
kgc-receipt create \
  --before=sha256:abc123 \
  --after=sha256:def456 \
  --script="go test -v"
```

**Options:**
- `--before` - Input hash (required)
- `--after` - Output hash (required)
- `--script` - Replay script (required)
- `--agent-id` - Agent ID [default: `agent-0`]
- `--output` - Output file [default: stdout]

**Output:**

```
Creating receipt...

✓ Receipt created
Execution ID: 550e8400-e29b-41d4-a716-446655440000
Agent ID: agent-0
Input hash: sha256:abc123
Output hash: sha256:def456
```

---

#### kgc-receipt verify

Verifies a receipt.

```bash
kgc-receipt verify --file=/tmp/receipt.json
```

**Options:**
- `--file` - Receipt file path (required)

**Output:**

```
Loading receipt...
Execution ID: 550e8400-e29b-41d4-a716-446655440000

Verifying receipt...
  ✓ Execution ID valid
  ✓ Hashes present
  ✓ Replay script present
  ✓ Composition op valid
  ✓ Conflict policy valid

✓ Receipt is valid
```

---

#### kgc-receipt chain

Chains multiple receipts.

```bash
kgc-receipt chain --files=r1.json,r2.json,r3.json --output=chained.json
```

**Options:**
- `--files` - Comma-separated receipt files (required)
- `--output` - Output file [default: stdout]

**Output:**

```
Loading receipts... 3 found

Validating chain continuity...
  r1 → r2... ✓
  r2 → r3... ✓

Creating chained receipt...

✓ Chain created
Chain hash: sha256:abc123def456...
Output: chained.json
```

---

### Agent 9: Demo

#### kgc-demo run

Runs the multi-agent demo.

```bash
kgc-demo run --agents=3 --tasks=5
```

**Options:**
- `--agents` - Number of agents [default: `3`]
- `--tasks` - Number of tasks [default: `5`]
- `--verbose` - Verbose output [default: `false`]
- `--routing` - Routing strategy (`round-robin` | `priority`) [default: `round-robin`]
- `--storage` - Storage backend (`memory` | `file`) [default: `memory`]

**Output:**

```
KGC Multi-Agent Demo
====================

Initializing knowledge store... ✓
Spawning 3 agents... ✓
Routing 5 tasks... ✓
Executing tasks... ✓
Reconciling receipts... ✓
Generating global receipt... ✓

Demo completed in 2.3s

Summary:
  Agents: 3
  Tasks: 5
  Receipts: 5
  Conflicts: 0
  Global receipt: gr-demo-abc123
```

---

## Debugging Commands

### kgc-debug inspect-store

Inspects a knowledge store.

```bash
kgc-debug inspect-store --path=/var/lib/kgc/store.db
```

**Output:**

```
Knowledge Store Inspector
=========================

Backend: file
Path: /var/lib/kgc/store.db
Records: 1,523
Size: 2.3 MB
Created: 2025-01-15 10:30:00
Last modified: 2025-01-15 12:45:00

Latest snapshot:
  Hash: sha256:abc123def456...
  Timestamp: 2025-01-15 12:45:00
  Size: 2.3 MB

Recent records (last 5):
  1. user:123 → alice@example.com (hash: sha256:...)
  2. user:124 → bob@example.com (hash: sha256:...)
  ...
```

---

### kgc-debug verify-determinism

Tests for non-determinism.

```bash
kgc-debug verify-determinism --runs=10
```

**Options:**
- `--runs` - Number of test runs [default: `10`]

**Output:**

```
Running determinism test (10 runs)...

Run 1: hash=sha256:abc123
Run 2: hash=sha256:abc123
Run 3: hash=sha256:abc123
...
Run 10: hash=sha256:abc123

✓ All hashes identical
✓ Determinism verified
```

---

### kgc-debug trace-receipt

Traces a receipt chain.

```bash
kgc-debug trace-receipt --start=r-001 --depth=5
```

**Options:**
- `--start` - Starting receipt ID (required)
- `--depth` - Maximum chain depth [default: `10`]

**Output:**

```
Tracing receipt chain...

r-001 (agent-1)
  ├─ Input:  sha256:abc123
  ├─ Output: sha256:def456
  └─ Script: go test -v

    └─> r-002 (agent-2)
        ├─ Input:  sha256:def456
        ├─ Output: sha256:ghi789
        └─ Script: go build

            └─> r-003 (agent-3)
                ├─ Input:  sha256:ghi789
                ├─ Output: sha256:jkl012
                └─ Script: go run demo.go

Chain length: 3
✓ Continuity verified
```

---

## Environment Variables

### KGC_STORE_PATH

Default knowledge store path.

```bash
export KGC_STORE_PATH=/var/lib/kgc/store.db
kgc-store snapshot  # Uses $KGC_STORE_PATH
```

### KGC_RECEIPT_DIR

Default directory for receipts.

```bash
export KGC_RECEIPT_DIR=/var/lib/kgc/receipts
kgc-receipt create ...  # Saves to $KGC_RECEIPT_DIR
```

### KGC_UNRDF_PATH

Path to unrdf repository.

```bash
export KGC_UNRDF_PATH=/tmp/unrdf-integration
make proof-p4  # Uses $KGC_UNRDF_PATH
```

### KGC_LOG_LEVEL

Logging level (`debug` | `info` | `warn` | `error`).

```bash
export KGC_LOG_LEVEL=debug
kgc-demo run  # Outputs debug logs
```

---

## Exit Codes

All commands use standard exit codes:

- `0` - Success
- `1` - General error
- `2` - Invalid arguments
- `3` - File not found
- `4` - Verification failed
- `5` - Non-determinism detected

---

## Configuration File

Create `~/.kgcrc` for persistent configuration:

```yaml
# KGC Configuration
store:
  backend: file
  path: /var/lib/kgc/store.db
  max_records: 1000000

receipts:
  directory: /var/lib/kgc/receipts
  auto_verify: true

logging:
  level: info
  output: /var/log/kgc.log

unrdf:
  path: /tmp/unrdf-integration
  auto_sync: false
```

---

## See Also

- [API Reference](api.md) - Programmatic API
- [Substrate Interfaces](substrate_interfaces.md) - Interface specifications
- [Getting Started Tutorial](../tutorial/getting_started.md)
- [How-To Guides](../how_to/)
