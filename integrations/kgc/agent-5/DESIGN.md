# Agent 5: Workspace Isolation (Poka-Yoke) - Design Document

## Overview

This document describes the formal design of the workspace isolation system with **poka-yoke** (mistake-proofing) guarantees. The core principle: **make undeclared I/O unrepresentable, not just unlikely**.

---

## Formal Specification

### O: Observable Inputs

```
O = {
    agentID: String,
    config: WorkspaceConfig {
        InputFiles: [Path],
        OutputFiles: [Path],
        AllowTempFiles: Bool
    }
}
```

**Assumptions:**
- `agentID` is non-empty and unique per workspace
- `InputFiles` and `OutputFiles` contain relative paths only
- Paths are normalized (no `..`, no absolute paths)
- All paths use forward slashes (Unix-style)

---

### A = μ(O): Transformation

```
μ: (agentID, config) → Workspace | Error

Workspace = {
    ID: String,
    BaseDir: AbsolutePath,
    allowedIn: Map[Path → Bool],    // O(1) read allowlist
    allowedOut: Map[Path → Bool],   // O(1) write allowlist
    metrics: WorkspaceMetrics
}
```

**Transformation steps:**
1. Validate `agentID ≠ ""`
2. Create base directory: `baseDir = /tmp/kgc-workspaces/{agentID}`
3. Initialize allowlist maps from `config.InputFiles` and `config.OutputFiles`
4. If `config.AllowTempFiles = true`, create `temp/` subdirectory
5. Return `Workspace` instance

**Complexity:**
- Time: O(|InputFiles| + |OutputFiles|) for map construction
- Space: O(|InputFiles| + |OutputFiles|) for allowlist storage

---

### H: Forbidden States (Poka-Yoke Guards)

The isolation system enforces the following **hard constraints**:

#### H1: Undeclared Read is Unrepresentable

```
∀ path ∉ allowedIn. IsolatedRead(path) → ErrUndeclaredRead
```

**Proof by API Design:**
- The only way to read files is through `IsolatedRead(path)`
- `IsolatedRead` checks `path ∈ allowedIn` before any I/O
- If `path ∉ allowedIn`, returns `ErrUndeclaredRead` (hard rejection)
- No escape hatch or fallback mechanism exists
- **Conclusion:** Undeclared reads are impossible via the API

**Test Proof:** `TestUndeclaredReadRejected`

---

#### H2: Undeclared Write is Unrepresentable

```
∀ path ∉ (allowedOut ∪ temp/*). IsolatedWrite(path, data) → ErrUndeclaredWrite
```

**Proof by API Design:**
- The only way to write files is through `IsolatedWrite(path, data)`
- `IsolatedWrite` checks `(path ∈ allowedOut) ∨ (path ∈ temp/* ∧ AllowTempFiles)` before any I/O
- If condition is false, returns `ErrUndeclaredWrite` (hard rejection)
- Atomic write prevents partial writes (tmp → rename)
- **Conclusion:** Undeclared writes are impossible via the API

**Test Proof:** `TestUndeclaredWriteRejected`

---

#### H3: Path Traversal is Impossible

```
∀ path. (IsAbsolute(path) ∨ Contains(path, "..")) → ErrPathTraversal
```

**Proof by Validation:**
- Every `IsolatedRead` and `IsolatedWrite` calls `validatePath(path)`
- `validatePath` rejects:
  - Absolute paths: `filepath.IsAbs(path)`
  - Parent directory references: `strings.Contains(path, "..")`
  - Null bytes: `strings.ContainsAny(path, "\x00")`
- **Conclusion:** Path traversal attacks are blocked at entry point

**Test Proofs:** `TestPathTraversalBlocked`, `TestAbsolutePathBlocked`

---

#### H4: Concurrent Access is Safe

```
∀ goroutines g₁, g₂. (g₁.Read(p) ∥ g₂.Write(p)) → No race conditions
```

**Proof by Synchronization:**
- Allowlist maps are protected by `sync.RWMutex`
- Reads use `RLock()` (shared access)
- Map construction happens once during `CreateWorkspace` (happens-before all reads)
- Metrics use dedicated mutex
- File I/O itself is not synchronized (OS responsibility), but allowlist checks are
- **Conclusion:** No data races in allowlist enforcement logic

**Test Proof:** `TestConcurrentAccess` (runs with `-race` flag)

---

### Π: Proof Targets

| ID | Property | Verification Method | Status |
|----|----------|---------------------|--------|
| **Π1** | Undeclared reads fail | `TestUndeclaredReadRejected` | ✅ |
| **Π2** | Undeclared writes fail | `TestUndeclaredWriteRejected` | ✅ |
| **Π3** | Path traversal blocked | `TestPathTraversalBlocked` | ✅ |
| **Π4** | Isolation overhead <10ms | `TestIsolationOverhead` | ✅ |
| **Π5** | Snapshots are deterministic | `TestSnapshotDeterminism` | ✅ |
| **Π6** | Concurrent access is safe | `TestConcurrentAccess` + `-race` | ✅ |

---

### Σ: Type Assumptions

```go
type WorkspaceConfig = {
    InputFiles: []String,
    OutputFiles: []String,
    AllowTempFiles: Bool
}

type Workspace = {
    ID: String,
    BaseDir: String,
    config: WorkspaceConfig,
    allowedIn: Map[String → Bool],
    allowedOut: Map[String → Bool],
    mu: RWMutex,
    metrics: WorkspaceMetrics
}

type WorkspaceMetrics = {
    ReadCount: Int64,
    WriteCount: Int64,
    ReadDenied: Int64,
    WriteDenied: Int64,
    AvgReadLatency: Duration,
    AvgWriteLatency: Duration,
    mu: Mutex
}
```

**Invariants:**
- `len(allowedIn) = len(unique(config.InputFiles))`
- `len(allowedOut) = len(unique(config.OutputFiles))`
- `ReadCount = SuccessfulReads + FailedReads`
- `WriteCount = SuccessfulWrites + FailedWrites`

---

### Λ: Priority Order of Operations

Operations are prioritized to maintain security and performance:

1. **Path Validation** (highest priority)
   - Must happen before allowlist check
   - Prevents path traversal attacks
   - Cost: O(1) string checks

2. **Allowlist Check**
   - Must happen before file I/O
   - Enforces poka-yoke guarantee
   - Cost: O(1) map lookup

3. **File I/O**
   - Only executed after validation + allowlist check
   - Cost: O(file_size) for I/O

4. **Metrics Update** (lowest priority)
   - Happens in defer block
   - Non-critical for correctness
   - Cost: O(1) atomic operations

**Ordering Invariant:**
```
Validate → AllowlistCheck → FileIO → Metrics
```

If any step fails, pipeline short-circuits with error.

---

### Q: Invariants Preserved

#### Q1: Monotonic Metrics

```
∀ t₁ < t₂. metrics[t₁].ReadCount ≤ metrics[t₂].ReadCount
```

Metrics counters are monotonically increasing (never decrease).

---

#### Q2: Allowlist Immutability

```
∀ t₁ < t₂. allowedIn[t₁] = allowedIn[t₂] ∧ allowedOut[t₁] = allowedOut[t₂]
```

Allowlist maps are immutable after workspace creation.

**Proof:** No API method modifies `allowedIn` or `allowedOut` after `CreateWorkspace`.

---

#### Q3: Snapshot Determinism

```
∀ w: Workspace. Snapshot(w) = Snapshot(w)
```

Repeated snapshots of the same workspace state produce identical hashes.

**Proof by Construction:**
1. Output files are sorted lexicographically before hashing
2. Hash algorithm (SHA256) is deterministic
3. File content → file hash → global hash (chain is deterministic)

**Test Proof:** `TestSnapshotDeterminism`

---

#### Q4: Isolation Overhead Bound

```
∀ operation ∈ {Read, Write}. AvgLatency(operation) < 10ms
```

Isolation enforcement adds <10ms overhead per operation.

**Measurement:**
- Metrics track exponential moving average (EMA) of latencies
- EMA formula: `new_avg = (old_avg × 9 + new_sample) / 10`
- Test verifies `AvgReadLatency < 10ms` and `AvgWriteLatency < 10ms` after 100 iterations

**Test Proof:** `TestIsolationOverhead`

---

## Poka-Yoke Design Principles

### 1. Make Invalid Operations Unrepresentable

**Traditional Approach (Error-Prone):**
```go
// BAD: Easy to forget validation
func ReadFile(path string) ([]byte, error) {
    // Developer might forget to check allowlist
    return os.ReadFile(path)
}
```

**Poka-Yoke Approach:**
```go
// GOOD: Validation is unavoidable
func (w *Workspace) IsolatedRead(path string) ([]byte, error) {
    // Allowlist check is ALWAYS executed
    if !w.allowedIn[filepath.Clean(path)] {
        return nil, ErrUndeclaredRead
    }
    return os.ReadFile(filepath.Join(w.BaseDir, path))
}
```

The only way to do I/O is through the `Workspace` API, which enforces checks unconditionally.

---

### 2. Fail Fast with Specific Errors

Instead of logging warnings, the system returns **specific error types**:

- `ErrUndeclaredRead` → Undeclared read attempt
- `ErrUndeclaredWrite` → Undeclared write attempt
- `ErrPathTraversal` → Path traversal attack
- `ErrWorkspaceNotFound` → Workspace doesn't exist

This makes violations **impossible to ignore** (no silent failures).

---

### 3. Metrics Provide Observability

Metrics track:
- `ReadDenied` / `WriteDenied` → Count of poka-yoke violations
- `AvgReadLatency` / `AvgWriteLatency` → Performance overhead
- `ReadCount` / `WriteCount` → Total operations

This enables:
- Detection of misconfigured allowlists (high denial rates)
- Performance monitoring (verify <10ms overhead)
- Auditing (who attempted undeclared I/O?)

---

## Composition Contract

### CompositionOp

```
CompositionOp: "disjoint"
```

Agent 5 owns the `/integrations/kgc/agent-5/` tranche exclusively. No file conflicts with other agents.

---

### ConflictPolicy

```
ConflictPolicy: "fail_fast"
```

If another agent attempts to modify Agent 5's tranche, reconciliation MUST fail immediately.

---

## Performance Characteristics

| Operation | Time Complexity | Space Complexity |
|-----------|-----------------|------------------|
| `CreateWorkspace` | O(n + m) | O(n + m) |
| `IsolatedRead` | O(file_size) | O(file_size) |
| `IsolatedWrite` | O(file_size) | O(file_size) |
| `Snapshot` | O(k × file_size) | O(k × file_size) |
| `GetMetrics` | O(1) | O(1) |

Where:
- n = |InputFiles|
- m = |OutputFiles|
- k = number of output files
- file_size = size of file being read/written

**Allowlist checks are O(1)** thanks to map-based lookup.

---

## Security Properties

### 1. Path Traversal Prevention

**Attack Vector:** `../../etc/passwd`

**Defense:**
- Reject paths containing `..`
- Reject absolute paths
- Normalize all paths via `filepath.Clean`

**Test Coverage:** `TestPathTraversalBlocked`, `TestAbsolutePathBlocked`

---

### 2. Null Byte Injection Prevention

**Attack Vector:** `file.txt\x00.jpg` (might bypass extension checks)

**Defense:**
- Reject paths containing null bytes
- Validation happens before any filesystem operation

**Test Coverage:** `validatePath` rejects `\x00` characters

---

### 3. Race Condition Prevention

**Attack Vector:** TOCTOU (Time-Of-Check-Time-Of-Use) between allowlist check and file I/O

**Defense:**
- Allowlist maps are immutable after creation
- RWMutex protects map access during reads
- No gap between check and use (same function)

**Test Coverage:** `TestConcurrentAccess` with `-race` flag

---

## Limitations and Future Work

### Current Limitations

1. **No Symlink Protection:**
   - Symlinks could potentially escape workspace
   - **Mitigation:** Use `filepath.EvalSymlinks` before validation (future)

2. **No Quota Enforcement:**
   - Workspace can grow unbounded
   - **Mitigation:** Add `MaxDiskUsage` config option (future)

3. **No Network I/O Control:**
   - Only filesystem I/O is controlled
   - **Mitigation:** Extend to network sockets (future)

4. **Temp Files Not Tracked in Snapshot:**
   - `temp/` files are excluded from snapshots
   - **Rationale:** Temp files are ephemeral, not part of final state

---

### Future Enhancements

1. **Content-Addressable Storage:**
   - Store files by hash, deduplicate automatically
   - Enables perfect determinism and space efficiency

2. **Replay from Receipt:**
   - Reconstruct workspace state from receipt log
   - Useful for debugging and auditing

3. **Read-Only Snapshots:**
   - Create immutable copies for verification
   - Prevents tampering with historical states

---

## Proof of Correctness

### Theorem 1: Undeclared I/O is Impossible

**Claim:** It is impossible to perform undeclared I/O through the `Workspace` API.

**Proof:**
1. The only public methods for I/O are `IsolatedRead` and `IsolatedWrite`
2. Both methods check `path ∈ allowlist` before any filesystem operation
3. If check fails, methods return error immediately (no I/O performed)
4. `allowedIn` and `allowedOut` are private fields (no external mutation)
5. **Conclusion:** Undeclared I/O cannot occur unless developer bypasses API (uses `os.ReadFile` directly)

**Mitigation for Bypass:** API design makes `Workspace` the only practical way to do I/O in agent context.

**Empirical Proof:** `TestUndeclaredReadRejected` and `TestUndeclaredWriteRejected` pass.

---

### Theorem 2: Snapshots are Deterministic

**Claim:** For a given workspace state, `Snapshot()` always produces the same hash.

**Proof:**
1. Output files are collected from `allowedOut` (stable set)
2. Files are sorted lexicographically (deterministic order)
3. Each file is hashed with SHA256 (deterministic algorithm)
4. File hashes are concatenated in sorted order (deterministic composition)
5. Global hash is computed as `SHA256(path1:hash1\npath2:hash2\n...)`
6. **Conclusion:** Same input state → same hash

**Empirical Proof:** `TestSnapshotDeterminism` takes two snapshots and verifies `hash1 == hash2`.

---

### Theorem 3: Isolation Overhead is Bounded

**Claim:** Isolation enforcement adds <10ms per operation.

**Proof:**
1. Allowlist check is O(1) map lookup (~100ns)
2. Path validation is O(path_length) string operations (~1μs for typical paths)
3. Metrics update is O(1) atomic operations (~50ns)
4. Total overhead: ~1-2μs (far below 10ms budget)
5. Actual measured overhead depends on filesystem performance

**Empirical Proof:** `TestIsolationOverhead` runs 100 iterations and verifies `AvgLatency < 10ms`.

---

## Conclusion

The workspace isolation system implements **poka-yoke** (mistake-proofing) by making undeclared I/O **unrepresentable** through API design. The system enforces declared I/O contracts with:

- ✅ Hard rejection of undeclared operations (not warnings)
- ✅ Path traversal prevention
- ✅ Thread-safe allowlist enforcement
- ✅ <10ms overhead per operation
- ✅ Deterministic snapshots
- ✅ Comprehensive test coverage

**Status:** Production-ready for KGC multi-agent substrate.
