#!/bin/bash
set -euo pipefail

# Agent 4 (Resource Allocation & Capacity) - Replay Script
# This script reproduces the exact build and test execution for KGC Agent 4

echo "=== Agent 4: Resource Allocation & Capacity - Replay ==="
echo "Execution ID: agent-4-kgc-capacity-allocator-489afd21f20665bbc2739ed397d43897"
echo "Timestamp: 1766796038060839950"
echo ""

# Navigate to agent-4 directory
cd /home/user/claude-squad/integrations/kgc/agent-4

# Verify file hashes (optional but recommended)
echo "Step 1: Verifying file integrity..."
expected_design_hash="8d721f4d567abb111972fed6e740753766693416e7f9f70beb39a563f2ead786"
expected_impl_hash="f1a00661cc3b489b732798e7dd332d7374d4266aa837f7b566a66f1c445e98dd"
expected_test_hash="856a5908b17042083a056eee27145b2e81c4d997027716a49b94a0dc1c3443a7"

actual_design_hash=$(sha256sum DESIGN.md | cut -d' ' -f1)
actual_impl_hash=$(sha256sum capacity_allocator.go | cut -d' ' -f1)
actual_test_hash=$(sha256sum capacity_allocator_test.go | cut -d' ' -f1)

if [ "$actual_design_hash" != "$expected_design_hash" ]; then
  echo "ERROR: DESIGN.md hash mismatch"
  echo "  Expected: $expected_design_hash"
  echo "  Actual:   $actual_design_hash"
  exit 1
fi

if [ "$actual_impl_hash" != "$expected_impl_hash" ]; then
  echo "ERROR: capacity_allocator.go hash mismatch"
  echo "  Expected: $expected_impl_hash"
  echo "  Actual:   $actual_impl_hash"
  exit 1
fi

if [ "$actual_test_hash" != "$expected_test_hash" ]; then
  echo "ERROR: capacity_allocator_test.go hash mismatch"
  echo "  Expected: $expected_test_hash"
  echo "  Actual:   $actual_test_hash"
  exit 1
fi

echo "✓ File integrity verified"
echo ""

# Step 2: Initialize Go module (if not already done)
echo "Step 2: Initializing Go module..."
if [ ! -f go.mod ]; then
  go mod init github.com/seanchatmangpt/claude-squad/integrations/kgc/agent-4
  echo "✓ Go module initialized"
else
  echo "✓ Go module already exists"
fi
echo ""

# Step 3: Build the package
echo "Step 3: Building capacity allocator..."
go build .
if [ $? -eq 0 ]; then
  echo "✓ Build succeeded"
else
  echo "✗ Build failed"
  exit 1
fi
echo ""

# Step 4: Run all tests with verbose output
echo "Step 4: Running comprehensive test suite..."
go test -v
if [ $? -eq 0 ]; then
  echo "✓ All tests passed"
else
  echo "✗ Tests failed"
  exit 1
fi
echo ""

# Step 5: Verify determinism (run tests multiple times)
echo "Step 5: Verifying determinism..."
for i in {1..3}; do
  echo "  Run $i/3..."
  go test -run TestDeterminism > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "✗ Determinism verification failed on run $i"
    exit 1
  fi
done
echo "✓ Determinism verified (3 consecutive runs)"
echo ""

# Step 6: Verify fairness properties
echo "Step 6: Verifying fairness properties..."
go test -run Fairness -v
if [ $? -eq 0 ]; then
  echo "✓ Fairness properties verified"
else
  echo "✗ Fairness verification failed"
  exit 1
fi
echo ""

# Step 7: Verify priority ordering
echo "Step 7: Verifying priority ordering..."
go test -run Priority -v
if [ $? -eq 0 ]; then
  echo "✓ Priority ordering verified"
else
  echo "✗ Priority ordering verification failed"
  exit 1
fi
echo ""

# Step 8: Verify resource conservation
echo "Step 8: Verifying resource conservation..."
go test -run ResourceConservation -v
if [ $? -eq 0 ]; then
  echo "✓ Resource conservation verified"
else
  echo "✗ Resource conservation verification failed"
  exit 1
fi
echo ""

# Step 9: Verify exhaustion detection
echo "Step 9: Verifying exhaustion detection..."
go test -run Exhaustion -v
if [ $? -eq 0 ]; then
  echo "✓ Exhaustion detection verified"
else
  echo "✗ Exhaustion detection verification failed"
  exit 1
fi
echo ""

# Final summary
echo "======================================"
echo "Agent 4 Replay: SUCCESS"
echo "======================================"
echo "All operations completed successfully:"
echo "  ✓ File integrity verified"
echo "  ✓ Go module initialized"
echo "  ✓ Build succeeded"
echo "  ✓ All 17 tests passed"
echo "  ✓ Determinism verified"
echo "  ✓ Fairness verified"
echo "  ✓ Priority ordering verified"
echo "  ✓ Resource conservation verified"
echo "  ✓ Exhaustion detection verified"
echo ""
echo "This execution is reproducible and deterministic."
echo "Execution ID: agent-4-kgc-capacity-allocator-489afd21f20665bbc2739ed397d43897"
