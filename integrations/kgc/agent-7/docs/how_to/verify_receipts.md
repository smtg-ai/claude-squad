# How to Verify Receipts

This guide shows you how to create, chain, and verify cryptographic receipts that prove your KGC operations are deterministic and tamper-proof.

## Problem

You need to:

- Prove operations are reproducible
- Detect tampering in receipt chains
- Validate multi-agent composition
- Generate audit trails

## Solution

Use the Receipt interface from Agent 2 to create cryptographic proofs with before/after hashes and replay scripts.

## Prerequisites

- Understanding of [KGC core concepts](../tutorial/getting_started.md)
- Basic cryptographic knowledge (SHA256 hashing)

## Step 1: Import Dependencies

```go
import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "log"

    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-2"
)
```

## Step 2: Create a Simple Receipt

```go
func createReceipt(beforeHash, afterHash, replayScript string) *agent2.Receipt {
    receipt := &agent2.Receipt{
        ExecutionID:    generateUUID(),
        AgentID:        "agent-7",
        Timestamp:      time.Now().UnixNano(),
        ToolchainVer:   runtime.Version(),
        InputHash:      beforeHash,
        OutputHash:     afterHash,
        ReplayScript:   replayScript,
        CompositionOp:  "append",
        ConflictPolicy: "fail_fast",
        ProofArtifacts: make(map[string]string),
    }

    return receipt
}
```

## Step 3: Compute Input/Output Hashes

```go
func computeStateHash(state interface{}) string {
    // Serialize to canonical JSON
    data, err := json.Marshal(state)
    if err != nil {
        log.Fatalf("Failed to serialize state: %v", err)
    }

    // Compute SHA256
    hash := sha256.Sum256(data)
    return fmt.Sprintf("sha256:%x", hash)
}
```

## Step 4: Verify a Receipt

```go
func verifyReceipt(receipt *agent2.Receipt) (bool, error) {
    // Validate required fields
    if receipt.ExecutionID == "" {
        return false, fmt.Errorf("missing execution ID")
    }

    if receipt.InputHash == "" || receipt.OutputHash == "" {
        return false, fmt.Errorf("missing hashes")
    }

    if receipt.ReplayScript == "" {
        return false, fmt.Errorf("missing replay script")
    }

    // Use Agent 2's verification
    valid := agent2.VerifyReceipt(receipt)
    if !valid {
        return false, fmt.Errorf("receipt verification failed")
    }

    return true, nil
}
```

## Step 5: Chain Receipts

Receipts form a chain where each receipt's `OutputHash` becomes the next receipt's `InputHash`:

```go
func chainReceipts(receipt1, receipt2 *agent2.Receipt) (*agent2.ChainedReceipt, error) {
    // Verify chain continuity
    if receipt1.OutputHash != receipt2.InputHash {
        return nil, fmt.Errorf(
            "chain broken: %s != %s",
            receipt1.OutputHash,
            receipt2.InputHash,
        )
    }

    // Create chained receipt
    chained := &agent2.ChainedReceipt{
        Receipts: []*agent2.Receipt{receipt1, receipt2},
        ChainHash: computeChainHash(receipt1, receipt2),
    }

    return chained, nil
}

func computeChainHash(receipts ...*agent2.Receipt) string {
    var combined string
    for _, r := range receipts {
        combined += r.OutputHash
    }
    hash := sha256.Sum256([]byte(combined))
    return fmt.Sprintf("sha256:%x", hash)
}
```

## Step 6: Detect Deliberate Tampering

```go
func testTamperDetection() {
    // Create valid receipt
    receipt := createReceipt(
        "sha256:abc123",
        "sha256:def456",
        "go run main.go",
    )

    // Verify original is valid
    valid, _ := verifyReceipt(receipt)
    if !valid {
        log.Fatal("Original receipt should be valid")
    }

    // Tamper with output hash
    tamperedReceipt := *receipt
    tamperedReceipt.OutputHash = "sha256:tampered"

    // Verify tampered receipt fails
    valid, err := verifyReceipt(&tamperedReceipt)
    if valid {
        log.Fatal("Tampered receipt should be invalid")
    }

    fmt.Printf("✓ Tamper detection working: %v\n", err)
}
```

## Step 7: Store Receipts Persistently

```go
func saveReceipt(receipt *agent2.Receipt, path string) error {
    // Serialize to JSON
    data, err := json.MarshalIndent(receipt, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal failed: %w", err)
    }

    // Write to file
    filename := fmt.Sprintf("%s/receipt_%s.json", path, receipt.ExecutionID)
    if err := os.WriteFile(filename, data, 0600); err != nil {
        return fmt.Errorf("write failed: %w", err)
    }

    log.Printf("Receipt saved: %s", filename)
    return nil
}

func loadReceipt(path string) (*agent2.Receipt, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read failed: %w", err)
    }

    var receipt agent2.Receipt
    if err := json.Unmarshal(data, &receipt); err != nil {
        return nil, fmt.Errorf("unmarshal failed: %w", err)
    }

    return &receipt, nil
}
```

## Complete Example

```go
package main

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "runtime"
    "time"

    "github.com/google/uuid"
    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-2"
)

func main() {
    // Simulate operation with before/after states
    beforeState := map[string]string{"count": "0"}
    afterState := map[string]string{"count": "1"}

    beforeHash := computeStateHash(beforeState)
    afterHash := computeStateHash(afterState)

    // Create receipt
    receipt := createReceipt(
        beforeHash,
        afterHash,
        "go run main.go",
    )

    // Add proof artifacts
    receipt.ProofArtifacts["test_log"] = "all tests passed"
    receipt.ProofArtifacts["build_hash"] = "sha256:build123"

    // Verify receipt
    valid, err := verifyReceipt(receipt)
    if err != nil {
        log.Fatalf("Verification failed: %v", err)
    }

    fmt.Printf("✓ Receipt valid: %v\n", valid)
    fmt.Printf("  Execution ID: %s\n", receipt.ExecutionID)
    fmt.Printf("  Input hash:   %s\n", receipt.InputHash[:24]+"...")
    fmt.Printf("  Output hash:  %s\n", receipt.OutputHash[:24]+"...")

    // Save receipt
    if err := saveReceipt(receipt, "/tmp/kgc_receipts"); err != nil {
        log.Fatalf("Save failed: %v", err)
    }

    // Test tamper detection
    testTamperDetection()

    fmt.Println("\n✓ All receipt operations successful")
}

func createReceipt(beforeHash, afterHash, replayScript string) *agent2.Receipt {
    return &agent2.Receipt{
        ExecutionID:    uuid.New().String(),
        AgentID:        "agent-7",
        Timestamp:      time.Now().UnixNano(),
        ToolchainVer:   runtime.Version(),
        InputHash:      beforeHash,
        OutputHash:     afterHash,
        ReplayScript:   replayScript,
        CompositionOp:  "append",
        ConflictPolicy: "fail_fast",
        ProofArtifacts: make(map[string]string),
    }
}

func computeStateHash(state interface{}) string {
    data, _ := json.Marshal(state)
    hash := sha256.Sum256(data)
    return fmt.Sprintf("sha256:%x", hash)
}

func verifyReceipt(receipt *agent2.Receipt) (bool, error) {
    if receipt.ExecutionID == "" || receipt.InputHash == "" || receipt.OutputHash == "" {
        return false, fmt.Errorf("missing required fields")
    }
    return agent2.VerifyReceipt(receipt), nil
}

func saveReceipt(receipt *agent2.Receipt, path string) error {
    os.MkdirAll(path, 0755)
    data, _ := json.MarshalIndent(receipt, "", "  ")
    filename := fmt.Sprintf("%s/receipt_%s.json", path, receipt.ExecutionID)
    return os.WriteFile(filename, data, 0600)
}

func testTamperDetection() {
    receipt := createReceipt("sha256:abc", "sha256:def", "go run main.go")
    tamperedReceipt := *receipt
    tamperedReceipt.OutputHash = "sha256:tampered"

    if valid, _ := verifyReceipt(&tamperedReceipt); valid {
        log.Fatal("Tampered receipt should be invalid")
    }

    fmt.Println("✓ Tamper detection working")
}
```

## Best Practices

### 1. Always Include Replay Scripts

```go
receipt.ReplayScript = "#!/bin/bash\nset -e\ngo test -v ./...\n"
```

### 2. Use Canonical JSON Serialization

```go
// Ensure deterministic JSON
data, _ := json.Marshal(state)
// Sort keys if using map
```

### 3. Validate Chain Continuity

```go
for i := 1; i < len(receipts); i++ {
    if receipts[i-1].OutputHash != receipts[i].InputHash {
        return fmt.Errorf("chain broken at index %d", i)
    }
}
```

### 4. Include Proof Artifacts

```go
receipt.ProofArtifacts = map[string]string{
    "test_output":    testLog,
    "build_hash":     buildHash,
    "lint_result":    "pass",
    "coverage":       "87.3%",
}
```

### 5. Set Appropriate Composition Policies

```go
// For independent operations
receipt.CompositionOp = "append"
receipt.ConflictPolicy = "fail_fast"

// For mergeable changes
receipt.CompositionOp = "merge"
receipt.ConflictPolicy = "merge"
```

## Verification Checklist

Use this checklist when verifying receipts:

- [ ] ExecutionID is unique and non-empty
- [ ] InputHash and OutputHash are valid SHA256 hashes
- [ ] ReplayScript is executable and deterministic
- [ ] Timestamp is reasonable (not in future, not too old)
- [ ] ToolchainVer matches expected version
- [ ] ProofArtifacts contain required evidence
- [ ] CompositionOp and ConflictPolicy are valid values
- [ ] Receipt chain continuity is intact

## Testing Receipt Chains

```go
func TestReceiptChain(t *testing.T) {
    // Create chain of 3 receipts
    r1 := createReceipt("hash0", "hash1", "step1.sh")
    r2 := createReceipt("hash1", "hash2", "step2.sh")
    r3 := createReceipt("hash2", "hash3", "step3.sh")

    // Verify chain
    receipts := []*agent2.Receipt{r1, r2, r3}
    for i := 1; i < len(receipts); i++ {
        if receipts[i-1].OutputHash != receipts[i].InputHash {
            t.Errorf("Chain broken at index %d", i)
        }
    }
}
```

## Common Pitfalls

### Non-Deterministic Hashing

```go
// ❌ BAD: Includes timestamp
hash := sha256.Sum256([]byte(fmt.Sprintf("%v%d", data, time.Now().Unix())))

// ✅ GOOD: Deterministic
hash := sha256.Sum256([]byte(fmt.Sprintf("%v", data)))
```

### Broken Chains

```go
// ❌ BAD: Gaps in chain
r1 := createReceipt("hash0", "hash1", "...")
r2 := createReceipt("hash5", "hash6", "...") // Gap!

// ✅ GOOD: Continuous chain
r1 := createReceipt("hash0", "hash1", "...")
r2 := createReceipt("hash1", "hash2", "...")
```

### Missing Replay Scripts

```go
// ❌ BAD: Empty replay script
receipt.ReplayScript = ""

// ✅ GOOD: Executable script
receipt.ReplayScript = "#!/bin/bash\nset -euo pipefail\ngo run main.go\n"
```

## Next Steps

- [Run Multi-Agent Demo](run_multi_agent_demo.md) - See receipts in action
- [Receipt Chaining](../explanation/receipt_chaining.md) - Deep dive into theory
- [API Reference](../reference/api.md) - Complete Receipt API

## See Also

- [Getting Started Tutorial](../tutorial/getting_started.md)
- [Create a Knowledge Store](create_knowledge_store.md)
- [Why Determinism Matters](../explanation/why_determinism.md)
