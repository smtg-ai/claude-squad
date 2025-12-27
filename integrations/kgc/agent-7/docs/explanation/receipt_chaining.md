# Receipt Chaining

This document explains how cryptographic receipt chains provide tamper-proof audit trails for multi-agent code generation.

## What is Receipt Chaining?

Receipt chaining creates an immutable sequence of cryptographic proofs, where each receipt's output becomes the next receipt's input.

### Visual Representation

```
Receipt 1                Receipt 2                Receipt 3
┌──────────────┐        ┌──────────────┐        ┌──────────────┐
│ Input:  hash0│        │ Input:  hash1│────┐   │ Input:  hash2│
│ ↓            │        │ ↓            │    │   │ ↓            │
│ Operation A  │        │ Operation B  │    │   │ Operation C  │
│ ↓            │        │ ↓            │    │   │ ↓            │
│ Output: hash1│────────│ Output: hash2│────────│ Output: hash3│
└──────────────┘        └──────────────┘        └──────────────┘
```

**Key Property:** Receipt N's `OutputHash` = Receipt N+1's `InputHash`

This creates an **unbreakable chain** where:

- You cannot insert receipts (breaks continuity)
- You cannot remove receipts (breaks continuity)
- You cannot modify receipts (changes hash, breaks continuity)

## Why Chain Receipts?

### Problem: Isolated Proofs Are Insufficient

Without chaining, receipts are isolated:

```
Receipt A: before=hash1, after=hash2
Receipt B: before=hash5, after=hash6

Question: Did Receipt B happen after Receipt A?
Answer: UNKNOWN (no relationship)
```

### Solution: Chain Creates Total Order

With chaining, relationships are cryptographically proven:

```
Receipt A: before=hash1, after=hash2
Receipt B: before=hash2, after=hash3

Fact: Receipt B MUST have happened after Receipt A
Proof: hash2 links them
```

## How Receipt Chains Work

### 1. Create First Receipt

```go
initialHash := "sha256:initial_state"
record := Record{Key: "count", Value: "0"}
hash, _ := store.Append(ctx, record)

receipt1 := agent2.CreateReceipt(
    initialHash,      // Input hash
    hash,             // Output hash
    "go run step1.sh",
)
```

**Receipt 1:**

```json
{
  "execution_id": "r-001",
  "input_hash": "sha256:initial_state",
  "output_hash": "sha256:abc123",
  "replay_script": "go run step1.sh"
}
```

### 2. Chain Second Receipt

```go
// Use previous output as current input
previousOutput := receipt1.OutputHash

record2 := Record{Key: "count", Value: "1"}
hash2, _ := store.Append(ctx, record2)

receipt2 := agent2.CreateReceipt(
    previousOutput,   // Chain from R1!
    hash2,
    "go run step2.sh",
)
```

**Receipt 2:**

```json
{
  "execution_id": "r-002",
  "input_hash": "sha256:abc123",  ← Matches R1's output_hash
  "output_hash": "sha256:def456",
  "replay_script": "go run step2.sh"
}
```

### 3. Validate Chain Continuity

```go
func validateChain(receipts []*Receipt) error {
    for i := 1; i < len(receipts); i++ {
        prev := receipts[i-1]
        curr := receipts[i]

        if prev.OutputHash != curr.InputHash {
            return fmt.Errorf(
                "chain broken at index %d: %s != %s",
                i,
                prev.OutputHash,
                curr.InputHash,
            )
        }
    }
    return nil  // Chain is intact
}
```

## Tamper Detection

Receipt chains make tampering **immediately detectable**.

### Attack 1: Modify Receipt

**Attacker tries:**

```json
Original Receipt 2:
{
  "input_hash": "sha256:abc123",
  "output_hash": "sha256:def456"
}

Tampered Receipt 2:
{
  "input_hash": "sha256:abc123",
  "output_hash": "sha256:HACKED"  ← Changed!
}
```

**Detection:**

```go
// Receipt 3 still expects sha256:def456
if receipt2.OutputHash != receipt3.InputHash {
    return errors.New("TAMPER DETECTED: chain broken")
}
```

**Result:** Tampering detected in <1ms

### Attack 2: Insert Receipt

**Attacker tries:**

```
Original chain:
R1 (out: abc123) → R2 (in: abc123, out: def456) → R3 (in: def456)

Attacker inserts R1.5:
R1 (out: abc123) → R1.5 (in: ???, out: ???) → R2 (in: abc123)
```

**Problem:** No way to make R1.5 fit without breaking chain

**Detection:**

```go
// R1's output (abc123) doesn't match R1.5's input
// OR
// R1.5's output doesn't match R2's input (abc123)
```

**Result:** Insertion impossible without detection

### Attack 3: Remove Receipt

**Attacker tries:**

```
Original chain:
R1 (out: abc123) → R2 (in: abc123, out: def456) → R3 (in: def456)

Attacker removes R2:
R1 (out: abc123) → ??? → R3 (in: def456)
```

**Detection:**

```go
// R1's output (abc123) doesn't match R3's input (def456)
validateChain([R1, R3])  // Returns error: chain broken
```

**Result:** Removal detected immediately

## Multi-Agent Receipt Chains

When multiple agents work in parallel, each produces a receipt chain:

### Parallel Chains

```
Agent 1:
  R1-A (hash0 → hash1) → R1-B (hash1 → hash2)

Agent 2:
  R2-A (hash0 → hash3) → R2-B (hash3 → hash4)

Agent 3:
  R3-A (hash0 → hash5) → R3-B (hash5 → hash6)
```

**Challenge:** How to reconcile parallel chains?

### Solution: Tree Structure

```
                  ┌─ Agent 1: hash1 → hash2
Initial (hash0) ──┼─ Agent 2: hash3 → hash4
                  └─ Agent 3: hash5 → hash6

                         ↓
                   Reconciler

                  Final (hashF)
                  [Combined proof]
```

The reconciler validates:

1. All chains start from same initial hash
2. No overlapping file modifications
3. All compositions valid
4. Produces global receipt

### Global Receipt

```json
{
  "execution_id": "global-001",
  "input_hash": "sha256:hash0",
  "output_hash": "sha256:hashF",
  "sub_receipts": [
    {"agent": "agent-1", "chain": ["r1-a", "r1-b"]},
    {"agent": "agent-2", "chain": ["r2-a", "r2-b"]},
    {"agent": "agent-3", "chain": ["r3-a", "r3-b"]}
  ],
  "composition_proof": "all chains reconcile without conflict"
}
```

## Receipt Chain Properties

### Property 1: Transitivity

If A → B and B → C, then A → C

```
Receipt A: hash0 → hash1
Receipt B: hash1 → hash2
Receipt C: hash2 → hash3

Conclusion: hash0 → hash3 (via A, B, C)
```

### Property 2: Non-Repudiation

Once a receipt is in the chain, you cannot deny creating it.

**Why?** Receipt includes:

- Execution ID (unique)
- Agent ID (who created it)
- Timestamp (when created)
- Replay script (how to reproduce)

**Example:**

```json
{
  "agent_id": "agent-7",
  "execution_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": 1704106800000000000,
  "replay_script": "#!/bin/bash\ngo run docs.go\n"
}
```

Agent 7 cannot later claim "I didn't create this receipt" because:

- Their agent ID is in the receipt
- Replay script is reproducible (anyone can verify)
- Receipt is chained (removing it breaks chain)

### Property 3: Immutability

Receipts cannot be changed after creation.

**Why?** Changing any field changes the hash:

```
Original receipt hash: sha256:abc123
Modified receipt hash: sha256:xyz789  (different!)

Chain breaks:
  prev.OutputHash (abc123) != modified.InputHash (xyz789)
```

### Property 4: Completeness

Chain contains complete history.

**Implication:** Can replay entire chain to reconstruct final state:

```bash
# Replay entire chain
for receipt in $(cat chain.json | jq -r '.receipts[].replay_script'); do
    bash -c "$receipt"
done

# Final hash should match chain's final output_hash
```

## Real-World Example

### Scenario: 3-Agent Documentation Pipeline

**Agent 1:** Write markdown files

```go
// Agent 1 creates docs
receipt1 := CreateReceipt(
    "sha256:empty",
    "sha256:docs_written",
    "write_docs.sh",
)
```

**Agent 2:** Validate links

```go
// Agent 2 validates (chains from Agent 1)
receipt2 := CreateReceipt(
    "sha256:docs_written",  // ← Chains from R1
    "sha256:docs_validated",
    "validate_links.sh",
)
```

**Agent 3:** Build HTML

```go
// Agent 3 builds (chains from Agent 2)
receipt3 := CreateReceipt(
    "sha256:docs_validated",  // ← Chains from R2
    "sha256:html_built",
    "build_html.sh",
)
```

**Chain:**

```
R1: empty → docs_written
  ↓
R2: docs_written → docs_validated
  ↓
R3: docs_validated → html_built
```

**Verification:**

```bash
# Verify chain
kgc-receipt chain --files=r1.json,r2.json,r3.json

✓ Chain is valid
✓ Continuity verified
✓ All receipts valid
```

**Replay:**

```bash
# Reproduce entire pipeline
bash r1_replay.sh  # Writes docs
bash r2_replay.sh  # Validates links
bash r3_replay.sh  # Builds HTML

# Final hash should be sha256:html_built
```

## Advanced: Fork Detection

Receipt chains can detect unauthorized forks.

### Legitimate Linear Chain

```
R1 → R2 → R3 → R4
```

### Unauthorized Fork

```
      ┌─ R3a (unauthorized)
R1 → R2
      └─ R3b (authorized)
```

**Detection:**

Both R3a and R3b have same input (R2's output), creating a fork.

```go
func detectFork(receipts []*Receipt) error {
    inputsSeen := make(map[string]bool)

    for _, r := range receipts {
        if inputsSeen[r.InputHash] {
            return fmt.Errorf("FORK DETECTED: multiple receipts with input %s", r.InputHash)
        }
        inputsSeen[r.InputHash] = true
    }

    return nil
}
```

## Performance Considerations

### Chain Verification Cost

| Chain Length | Verification Time |
|--------------|-------------------|
| 10 receipts  | < 1ms            |
| 100 receipts | < 10ms           |
| 1000 receipts| < 100ms          |

**Why fast?** Only need to check:

```go
for i := 1; i < len(receipts); i++ {
    if receipts[i-1].OutputHash != receipts[i].InputHash {
        return error
    }
}
```

### Storage Cost

Receipts are lightweight:

```json
{
  "execution_id": "...",      // 36 bytes (UUID)
  "input_hash": "sha256:...", // 71 bytes
  "output_hash": "sha256:...",// 71 bytes
  "replay_script": "...",     // ~500 bytes
  // ... other fields
}
```

**Total:** ~1 KB per receipt

**For 10,000 receipts:** ~10 MB (negligible)

## Best Practices

### 1. Always Validate Chains

```go
if err := validateChain(receipts); err != nil {
    log.Fatalf("Chain broken: %v", err)
}
```

### 2. Store Receipts Persistently

```bash
mkdir -p /var/lib/kgc/receipts
# Save each receipt as JSON file
```

### 3. Include Replay Scripts

```go
receipt.ReplayScript = "#!/bin/bash\nset -euo pipefail\ngo test -v\n"
```

### 4. Use Unique Execution IDs

```go
receipt.ExecutionID = uuid.New().String()  // Guaranteed unique
```

### 5. Regular Chain Audits

```bash
# Audit chain weekly
kgc-receipt verify-chain --start=2024-01-01 --end=2024-01-07
```

## Conclusion

Receipt chaining provides:

- ✅ **Tamper-proof audit trails**
- ✅ **Total ordering of operations**
- ✅ **Non-repudiation**
- ✅ **Immutability**
- ✅ **Completeness**

**Core Principle:**

> "Each receipt's output becomes the next receipt's input, creating an unbreakable cryptographic chain."

## Next Steps

- [Composition Laws](composition_laws.md) - How chains compose in multi-agent systems
- [Why Determinism Matters](why_determinism.md) - Foundation of receipt chains
- [How to Verify Receipts](../how_to/verify_receipts.md) - Practical guide

## See Also

- [API Reference](../reference/api.md)
- [Substrate Interfaces](../reference/substrate_interfaces.md)
- [Getting Started Tutorial](../tutorial/getting_started.md)
