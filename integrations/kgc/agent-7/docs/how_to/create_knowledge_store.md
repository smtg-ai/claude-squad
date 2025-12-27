# How to Create a Knowledge Store

This guide shows you how to create and configure a production-ready KnowledgeStore instance with proper error handling, logging, and persistence.

## Problem

You need a deterministic, append-only knowledge store that:

- Persists data across restarts
- Validates all writes
- Produces reproducible snapshots
- Integrates with receipt chains

## Solution

Use the KnowledgeStore implementation from Agent 1 with proper configuration and error handling.

## Prerequisites

- Go 1.21 or later
- Understanding of [KGC core concepts](../tutorial/getting_started.md)

## Step 1: Import Dependencies

```go
import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-1"
)
```

## Step 2: Configure Storage Backend

Choose a storage backend based on your needs:

### In-Memory (Development/Testing)

```go
config := agent1.KnowledgeStoreConfig{
    Backend:      "memory",
    MaxRecords:   10000,
    Deterministic: true,
}
```

### File-Based (Production)

```go
config := agent1.KnowledgeStoreConfig{
    Backend:       "file",
    StoragePath:   "/var/lib/kgc/knowledge.db",
    MaxRecords:    1000000,
    Deterministic: true,
    SyncMode:      "fsync", // Ensure durability
}
```

## Step 3: Create the Store

```go
store, err := agent1.NewKnowledgeStoreWithConfig(config)
if err != nil {
    log.Fatalf("Failed to create knowledge store: %v", err)
}
defer store.Close()
```

## Step 4: Append Records with Validation

```go
func appendRecord(ctx context.Context, store *agent1.KnowledgeStore, key, value string) (string, error) {
    // Validate inputs
    if key == "" {
        return "", fmt.Errorf("key cannot be empty")
    }

    record := agent1.Record{
        Key:   key,
        Value: value,
    }

    // Append with timeout
    hash, err := store.Append(ctx, record)
    if err != nil {
        return "", fmt.Errorf("append failed: %w", err)
    }

    log.Printf("Appended record %s with hash %s", key, hash[:16]+"...")
    return hash, nil
}
```

## Step 5: Implement Snapshot Checkpoints

```go
func createCheckpoint(ctx context.Context, store *agent1.KnowledgeStore, path string) error {
    // Take snapshot
    hash, data, err := store.Snapshot(ctx)
    if err != nil {
        return fmt.Errorf("snapshot failed: %w", err)
    }

    // Write to checkpoint file
    checkpointFile := fmt.Sprintf("%s/snapshot_%s.bin", path, hash[:16])
    if err := os.WriteFile(checkpointFile, data, 0600); err != nil {
        return fmt.Errorf("write checkpoint failed: %w", err)
    }

    log.Printf("Checkpoint saved: %s (hash: %s)", checkpointFile, hash[:16]+"...")
    return nil
}
```

## Step 6: Verify Snapshot Integrity

```go
func verifyCheckpoint(ctx context.Context, store *agent1.KnowledgeStore, expectedHash string) error {
    valid, err := store.Verify(ctx, expectedHash)
    if err != nil {
        return fmt.Errorf("verification failed: %w", err)
    }

    if !valid {
        return fmt.Errorf("snapshot hash mismatch: expected %s", expectedHash)
    }

    log.Printf("✓ Snapshot verified: %s", expectedHash[:16]+"...")
    return nil
}
```

## Step 7: Implement Replay for Disaster Recovery

```go
func replayFromLog(ctx context.Context, store *agent1.KnowledgeStore, logPath string) error {
    // Read event log
    events, err := agent1.LoadEventsFromFile(logPath)
    if err != nil {
        return fmt.Errorf("load events failed: %w", err)
    }

    // Replay events deterministically
    hash, err := store.Replay(ctx, events)
    if err != nil {
        return fmt.Errorf("replay failed: %w", err)
    }

    log.Printf("Replayed %d events (final hash: %s)", len(events), hash[:16]+"...")
    return nil
}
```

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-1"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Configure store
    config := agent1.KnowledgeStoreConfig{
        Backend:       "file",
        StoragePath:   "./kgc_store.db",
        MaxRecords:    100000,
        Deterministic: true,
    }

    // Create store
    store, err := agent1.NewKnowledgeStoreWithConfig(config)
    if err != nil {
        log.Fatalf("Failed to create store: %v", err)
    }
    defer store.Close()

    // Append records
    records := []struct{ key, value string }{
        {"user:1", "alice@example.com"},
        {"user:2", "bob@example.com"},
        {"config:version", "1.0.0"},
    }

    for _, r := range records {
        if _, err := appendRecord(ctx, store, r.key, r.value); err != nil {
            log.Fatalf("Append failed: %v", err)
        }
    }

    // Take snapshot
    hash, _, err := store.Snapshot(ctx)
    if err != nil {
        log.Fatalf("Snapshot failed: %v", err)
    }

    // Verify snapshot
    if err := verifyCheckpoint(ctx, store, hash); err != nil {
        log.Fatalf("Verification failed: %v", err)
    }

    fmt.Println("✓ Knowledge store created and verified successfully")
}

func appendRecord(ctx context.Context, store *agent1.KnowledgeStore, key, value string) (string, error) {
    record := agent1.Record{Key: key, Value: value}
    hash, err := store.Append(ctx, record)
    if err != nil {
        return "", fmt.Errorf("append failed: %w", err)
    }
    log.Printf("Appended: %s → %s (hash: %s)", key, value, hash[:16]+"...")
    return hash, nil
}

func verifyCheckpoint(ctx context.Context, store *agent1.KnowledgeStore, expectedHash string) error {
    valid, err := store.Verify(ctx, expectedHash)
    if err != nil {
        return fmt.Errorf("verification failed: %w", err)
    }
    if !valid {
        return fmt.Errorf("snapshot hash mismatch")
    }
    log.Printf("✓ Snapshot verified: %s", expectedHash[:16]+"...")
    return nil
}
```

## Best Practices

### 1. Always Use Context with Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

### 2. Validate Before Append

```go
if key == "" || len(value) > maxValueSize {
    return "", fmt.Errorf("invalid record")
}
```

### 3. Regular Snapshots

```go
// Snapshot every 1000 appends
if store.RecordCount() % 1000 == 0 {
    createCheckpoint(ctx, store, "/var/lib/kgc/checkpoints")
}
```

### 4. Monitor for Non-Determinism

```go
// Take two snapshots and compare
hash1, _, _ := store.Snapshot(ctx)
hash2, _, _ := store.Snapshot(ctx)

if hash1 != hash2 {
    log.Fatal("NON-DETERMINISM DETECTED!")
}
```

### 5. Graceful Shutdown

```go
defer func() {
    if err := store.Close(); err != nil {
        log.Printf("Warning: store close failed: %v", err)
    }
}()
```

## Common Pitfalls

### Non-Deterministic Hashing

Don't include timestamps or random values:

```go
// ❌ BAD
record := Record{
    Key:   "event",
    Value: fmt.Sprintf("happened at %v", time.Now()),
}

// ✅ GOOD
record := Record{
    Key:   "event",
    Value: "happened",
}
```

### Map Iteration Order

Go maps have random iteration order:

```go
// ❌ BAD
for k, v := range myMap {
    store.Append(ctx, Record{Key: k, Value: v})
}

// ✅ GOOD
keys := make([]string, 0, len(myMap))
for k := range myMap {
    keys = append(keys, k)
}
sort.Strings(keys)
for _, k := range keys {
    store.Append(ctx, Record{Key: k, Value: myMap[k]})
}
```

### Forgetting to Sync

For file-based stores, ensure durability:

```go
config := agent1.KnowledgeStoreConfig{
    Backend:  "file",
    SyncMode: "fsync", // ← Important!
}
```

## Testing Your Store

```go
func TestKnowledgeStoreDeterminism(t *testing.T) {
    store := agent1.NewKnowledgeStore()

    record := agent1.Record{Key: "test", Value: "data"}

    hash1, _ := store.Append(context.Background(), record)
    hash2, _ := store.Append(context.Background(), record)

    if hash1 != hash2 {
        t.Errorf("Non-deterministic append: %s != %s", hash1, hash2)
    }
}
```

## Next Steps

- [Verify Receipts](verify_receipts.md) - Add cryptographic proofs
- [API Reference](../reference/api.md) - Complete API documentation
- [Why Determinism Matters](../explanation/why_determinism.md) - Understand the theory

## See Also

- [Getting Started Tutorial](../tutorial/getting_started.md)
- [Composition Laws](../explanation/composition_laws.md)
- [Receipt Chaining](../explanation/receipt_chaining.md)
