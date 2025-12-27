# Getting Started with KGC Substrate

This tutorial will guide you through creating your first KGC-backed application. You'll learn the core concepts by building a simple, deterministic knowledge store.

## What You'll Learn

By the end of this tutorial, you'll be able to:

- Create a KnowledgeStore instance
- Append records deterministically
- Generate cryptographic receipts
- Verify receipt chains
- Understand snapshot hashing

## Prerequisites

- Go 1.21 or later
- Basic understanding of Go programming
- Git (to clone the repository)

## Step 1: Set Up Your Environment

First, clone the claude-squad repository:

```bash
git clone https://github.com/seanchatmangpt/claude-squad.git
cd claude-squad/integrations/kgc
```

## Step 2: Understand the Core Interface

The KnowledgeStore interface provides four key operations:

```go
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

## Step 3: Create Your First Knowledge Store

Create a new file `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-1"
)

func main() {
    ctx := context.Background()

    // Create a new KnowledgeStore
    store := agent1.NewKnowledgeStore()

    // Append a record
    record := agent1.Record{
        Key:   "greeting",
        Value: "Hello, KGC!",
    }

    hash, err := store.Append(ctx, record)
    if err != nil {
        log.Fatalf("Failed to append: %v", err)
    }

    fmt.Printf("Record appended with hash: %s\n", hash)
}
```

## Step 4: Run Your Program

Execute your program:

```bash
go run main.go
```

You should see output like:

```
Record appended with hash: sha256:a3f8b9c2d1e4f5...
```

## Step 5: Take a Snapshot

Now let's extend the program to create a deterministic snapshot:

```go
// Take a snapshot of current state
snapshotHash, snapshotData, err := store.Snapshot(ctx)
if err != nil {
    log.Fatalf("Failed to snapshot: %v", err)
}

fmt.Printf("Snapshot hash: %s\n", snapshotHash)
fmt.Printf("Snapshot size: %d bytes\n", len(snapshotData))
```

Run again:

```bash
go run main.go
```

## Step 6: Verify Determinism

The key property of KGC is **determinism**. Run your program multiple times:

```bash
go run main.go
go run main.go
go run main.go
```

**Important**: The snapshot hash should be **identical** every time. This proves:

```
∀ O. Snapshot(O) = Snapshot(O)
```

## Step 7: Generate a Receipt

Receipts prove that operations are reproducible. Let's create one:

```go
import "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-2"

// Generate receipt for this execution
receipt := agent2.CreateReceipt(
    "initial_state_hash",
    snapshotHash,
    "go run main.go",
)

fmt.Printf("Receipt ID: %s\n", receipt.ExecutionID)
fmt.Printf("Input hash: %s\n", receipt.InputHash)
fmt.Printf("Output hash: %s\n", receipt.OutputHash)
```

## Step 8: Verify a Receipt

Now verify the receipt is valid:

```go
valid := agent2.VerifyReceipt(receipt)
if valid {
    fmt.Println("✓ Receipt is valid")
} else {
    fmt.Println("✗ Receipt is invalid")
}
```

## Step 9: Test Idempotence

KGC operations are **idempotent**. Appending the same record twice produces the same result:

```go
// Append same record twice
hash1, _ := store.Append(ctx, record)
hash2, _ := store.Append(ctx, record)

if hash1 == hash2 {
    fmt.Println("✓ Idempotence verified: Append(x) = Append(Append(x))")
}
```

## Complete Example

Here's the full program:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-1"
    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-2"
)

func main() {
    ctx := context.Background()

    // Create KnowledgeStore
    store := agent1.NewKnowledgeStore()
    fmt.Println("Step 1: Created KnowledgeStore")

    // Append record
    record := agent1.Record{
        Key:   "greeting",
        Value: "Hello, KGC!",
    }
    hash, err := store.Append(ctx, record)
    if err != nil {
        log.Fatalf("Failed to append: %v", err)
    }
    fmt.Printf("Step 2: Appended record (hash: %s)\n", hash[:16]+"...")

    // Take snapshot
    snapshotHash, _, err := store.Snapshot(ctx)
    if err != nil {
        log.Fatalf("Failed to snapshot: %v", err)
    }
    fmt.Printf("Step 3: Created snapshot (hash: %s)\n", snapshotHash[:16]+"...")

    // Generate receipt
    receipt := agent2.CreateReceipt(
        "initial",
        snapshotHash,
        "go run main.go",
    )
    fmt.Printf("Step 4: Generated receipt (ID: %s)\n", receipt.ExecutionID)

    // Verify receipt
    if agent2.VerifyReceipt(receipt) {
        fmt.Println("Step 5: ✓ Receipt verified")
    }

    // Test idempotence
    hash2, _ := store.Append(ctx, record)
    if hash == hash2 {
        fmt.Println("Step 6: ✓ Idempotence confirmed")
    }

    fmt.Println("\nCongratulations! You've completed the KGC tutorial.")
}
```

## What You've Learned

- KGC operations are **deterministic** (same inputs → same outputs)
- Snapshots are **hash-stable** (reproducible across runs)
- Receipts provide **cryptographic proof** of execution
- Operations are **idempotent** (safe to retry)

## Next Steps

Now that you understand the basics:

- [Create a Knowledge Store](../how_to/create_knowledge_store.md) - Production patterns
- [Verify Receipts](../how_to/verify_receipts.md) - Deep dive into receipt validation
- [Run Multi-Agent Demo](../how_to/run_multi_agent_demo.md) - See 10 agents working together

## Troubleshooting

### Hash is different on each run

This means non-determinism has been introduced. Common causes:

- Using `time.Now()` or `rand.Random()` without seeding
- Iterating over Go maps (use sorted keys)
- Reading from external sources (network, filesystem)

### Receipt verification fails

Check that:

- Input/output hashes match actual state
- Replay script is executable
- No external dependencies changed

## Further Reading

- [Why Determinism Matters](../explanation/why_determinism.md)
- [Receipt Chaining](../explanation/receipt_chaining.md)
- [API Reference](../reference/api.md)
