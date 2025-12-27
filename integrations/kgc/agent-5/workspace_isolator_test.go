package agent5

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWorkspaceCreation verifies basic workspace initialization
func TestWorkspaceCreation(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles:  []string{"input/data.txt"},
		OutputFiles: []string{"output/result.txt"},
	}

	ws, err := CreateWorkspace("test-agent-1", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	if ws.ID != "test-agent-1" {
		t.Errorf("Expected ID=test-agent-1, got %s", ws.ID)
	}

	if ws.BaseDir == "" {
		t.Error("BaseDir should not be empty")
	}

	// Verify allowlist maps are populated
	if len(ws.allowedIn) != 1 {
		t.Errorf("Expected 1 allowed input, got %d", len(ws.allowedIn))
	}

	if len(ws.allowedOut) != 1 {
		t.Errorf("Expected 1 allowed output, got %d", len(ws.allowedOut))
	}
}

// TestAllowedReadSucceeds proves that declared reads work
func TestAllowedReadSucceeds(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles: []string{"input/data.txt"},
	}

	ws, err := CreateWorkspace("test-agent-read", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Create the input file directly (setup phase)
	inputPath := filepath.Join(ws.BaseDir, "input/data.txt")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
		t.Fatalf("Failed to create input dir: %v", err)
	}
	testData := []byte("test input data")
	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Now test IsolatedRead
	data, err := ws.IsolatedRead("input/data.txt")
	if err != nil {
		t.Fatalf("IsolatedRead failed for allowed path: %v", err)
	}

	if string(data) != "test input data" {
		t.Errorf("Expected 'test input data', got '%s'", string(data))
	}
}

// TestAllowedWriteSucceeds proves that declared writes work
func TestAllowedWriteSucceeds(t *testing.T) {
	config := WorkspaceConfig{
		OutputFiles: []string{"output/result.txt"},
	}

	ws, err := CreateWorkspace("test-agent-write", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	testData := []byte("test output data")
	err = ws.IsolatedWrite("output/result.txt", testData)
	if err != nil {
		t.Fatalf("IsolatedWrite failed for allowed path: %v", err)
	}

	// Verify file was actually written
	absPath := filepath.Join(ws.BaseDir, "output/result.txt")
	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(data) != "test output data" {
		t.Errorf("Expected 'test output data', got '%s'", string(data))
	}
}

// TestUndeclaredReadRejected - POKA-YOKE TEST
//
// This test PROVES that undeclared reads are impossible.
// If this test passes, the system has poka-yoke guarantee for reads.
func TestUndeclaredReadRejected(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles: []string{"allowed/input.txt"},
	}

	ws, err := CreateWorkspace("test-agent-read-deny", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Create a file that exists but is NOT in allowlist
	undeclaredPath := filepath.Join(ws.BaseDir, "forbidden/secret.txt")
	if err := os.MkdirAll(filepath.Dir(undeclaredPath), 0755); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := os.WriteFile(undeclaredPath, []byte("secret data"), 0644); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Attempt to read undeclared file
	data, err := ws.IsolatedRead("forbidden/secret.txt")
	if err == nil {
		t.Fatalf("POKA-YOKE VIOLATION: Undeclared read succeeded! Got data: %s", string(data))
	}

	// Verify it's the correct error type
	if !errors.Is(err, ErrUndeclaredRead) {
		t.Errorf("Expected ErrUndeclaredRead, got: %v", err)
	}

	// Verify metrics tracked the denial
	metrics := ws.GetMetrics()
	if metrics.ReadDenied != 1 {
		t.Errorf("Expected ReadDenied=1, got %d", metrics.ReadDenied)
	}
}

// TestUndeclaredWriteRejected - POKA-YOKE TEST
//
// This test PROVES that undeclared writes are impossible.
// If this test passes, the system has poka-yoke guarantee for writes.
func TestUndeclaredWriteRejected(t *testing.T) {
	config := WorkspaceConfig{
		OutputFiles: []string{"allowed/output.txt"},
	}

	ws, err := CreateWorkspace("test-agent-write-deny", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Attempt to write to undeclared path
	testData := []byte("forbidden data")
	err = ws.IsolatedWrite("forbidden/output.txt", testData)
	if err == nil {
		t.Fatalf("POKA-YOKE VIOLATION: Undeclared write succeeded!")
	}

	// Verify it's the correct error type
	if !errors.Is(err, ErrUndeclaredWrite) {
		t.Errorf("Expected ErrUndeclaredWrite, got: %v", err)
	}

	// Verify file was NOT created
	absPath := filepath.Join(ws.BaseDir, "forbidden/output.txt")
	_, err = os.ReadFile(absPath)
	if err == nil {
		t.Error("POKA-YOKE VIOLATION: Forbidden file was created!")
	}

	// Verify metrics tracked the denial
	metrics := ws.GetMetrics()
	if metrics.WriteDenied != 1 {
		t.Errorf("Expected WriteDenied=1, got %d", metrics.WriteDenied)
	}
}

// TestPathTraversalBlocked verifies security against path traversal attacks
func TestPathTraversalBlocked(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles:  []string{"../etc/passwd"},
		OutputFiles: []string{"../../tmp/malicious.txt"},
	}

	ws, err := CreateWorkspace("test-agent-traversal", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Attempt path traversal read
	_, err = ws.IsolatedRead("../etc/passwd")
	if err == nil {
		t.Fatal("Path traversal read should be blocked")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Errorf("Expected ErrPathTraversal, got: %v", err)
	}

	// Attempt path traversal write
	err = ws.IsolatedWrite("../../tmp/malicious.txt", []byte("bad"))
	if err == nil {
		t.Fatal("Path traversal write should be blocked")
	}
	if !errors.Is(err, ErrPathTraversal) {
		t.Errorf("Expected ErrPathTraversal, got: %v", err)
	}
}

// TestAbsolutePathBlocked verifies absolute paths are rejected
func TestAbsolutePathBlocked(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles:  []string{"/etc/passwd"},
		OutputFiles: []string{"/tmp/output.txt"},
	}

	ws, err := CreateWorkspace("test-agent-absolute", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Absolute paths should be rejected even if in allowlist
	_, err = ws.IsolatedRead("/etc/passwd")
	if err == nil {
		t.Fatal("Absolute path read should be blocked")
	}

	err = ws.IsolatedWrite("/tmp/output.txt", []byte("data"))
	if err == nil {
		t.Fatal("Absolute path write should be blocked")
	}
}

// TestTempFilesAllowed verifies AllowTempFiles feature
func TestTempFilesAllowed(t *testing.T) {
	config := WorkspaceConfig{
		OutputFiles:    []string{"output/result.txt"},
		AllowTempFiles: true,
	}

	ws, err := CreateWorkspace("test-agent-temp", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Writing to temp/ should succeed without declaration
	err = ws.IsolatedWrite("temp/intermediate.txt", []byte("temp data"))
	if err != nil {
		t.Fatalf("Temp file write should succeed: %v", err)
	}

	// Verify file exists
	absPath := filepath.Join(ws.BaseDir, "temp/intermediate.txt")
	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if string(data) != "temp data" {
		t.Errorf("Expected 'temp data', got '%s'", string(data))
	}

	// But non-temp undeclared writes should still fail
	err = ws.IsolatedWrite("other/file.txt", []byte("data"))
	if err == nil {
		t.Fatal("Non-temp undeclared write should fail")
	}
}

// TestTempFilesDisabled verifies temp/ is blocked when AllowTempFiles=false
func TestTempFilesDisabled(t *testing.T) {
	config := WorkspaceConfig{
		OutputFiles:    []string{"output/result.txt"},
		AllowTempFiles: false,
	}

	ws, err := CreateWorkspace("test-agent-no-temp", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Writing to temp/ should fail when AllowTempFiles=false
	err = ws.IsolatedWrite("temp/file.txt", []byte("data"))
	if err == nil {
		t.Fatal("Temp file write should fail when AllowTempFiles=false")
	}
	if !errors.Is(err, ErrUndeclaredWrite) {
		t.Errorf("Expected ErrUndeclaredWrite, got: %v", err)
	}
}

// TestIsolationOverhead verifies that isolation adds <10ms overhead per operation
func TestIsolationOverhead(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles:  []string{"input/data.txt"},
		OutputFiles: []string{"output/result.txt"},
	}

	ws, err := CreateWorkspace("test-agent-perf", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Setup: Create input file
	inputPath := filepath.Join(ws.BaseDir, "input/data.txt")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0755); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	testData := []byte("performance test data")
	if err := os.WriteFile(inputPath, testData, 0644); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Perform multiple reads and writes
	iterations := 100
	for i := 0; i < iterations; i++ {
		_, err := ws.IsolatedRead("input/data.txt")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		err = ws.IsolatedWrite("output/result.txt", testData)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Check metrics
	metrics := ws.GetMetrics()

	if metrics.ReadCount != int64(iterations) {
		t.Errorf("Expected %d reads, got %d", iterations, metrics.ReadCount)
	}

	if metrics.WriteCount != int64(iterations) {
		t.Errorf("Expected %d writes, got %d", iterations, metrics.WriteCount)
	}

	// Verify overhead is <10ms per operation
	maxAllowedLatency := 10 * time.Millisecond

	if metrics.AvgReadLatency > maxAllowedLatency {
		t.Errorf("Read overhead too high: %v (max allowed: %v)", metrics.AvgReadLatency, maxAllowedLatency)
	}

	if metrics.AvgWriteLatency > maxAllowedLatency {
		t.Errorf("Write overhead too high: %v (max allowed: %v)", metrics.AvgWriteLatency, maxAllowedLatency)
	}

	t.Logf("Performance metrics: AvgReadLatency=%v, AvgWriteLatency=%v", metrics.AvgReadLatency, metrics.AvgWriteLatency)
}

// TestSnapshotDeterminism verifies that snapshots are deterministic
func TestSnapshotDeterminism(t *testing.T) {
	config := WorkspaceConfig{
		OutputFiles: []string{
			"output/file1.txt",
			"output/file2.txt",
			"output/file3.txt",
		},
	}

	ws, err := CreateWorkspace("test-agent-snapshot", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Write files in random order
	ws.IsolatedWrite("output/file3.txt", []byte("data3"))
	ws.IsolatedWrite("output/file1.txt", []byte("data1"))
	ws.IsolatedWrite("output/file2.txt", []byte("data2"))

	// Take multiple snapshots
	hash1, files1, err := ws.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot 1 failed: %v", err)
	}

	hash2, files2, err := ws.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot 2 failed: %v", err)
	}

	// Hashes must be identical (determinism)
	if hash1 != hash2 {
		t.Errorf("Snapshots are not deterministic: hash1=%s, hash2=%s", hash1, hash2)
	}

	// Verify all files are included
	if len(files1) != 3 {
		t.Errorf("Expected 3 files in snapshot, got %d", len(files1))
	}

	// Verify file hashes match
	for path, hash := range files1 {
		if files2[path] != hash {
			t.Errorf("File hash mismatch for %s", path)
		}
	}
}

// TestConcurrentAccess verifies thread-safety
func TestConcurrentAccess(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles:  []string{"input/data.txt"},
		OutputFiles: []string{"output/result.txt"},
	}

	ws, err := CreateWorkspace("test-agent-concurrent", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Setup input file
	inputPath := filepath.Join(ws.BaseDir, "input/data.txt")
	os.MkdirAll(filepath.Dir(inputPath), 0755)
	os.WriteFile(inputPath, []byte("test data"), 0644)

	// Spawn multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 10; j++ {
				// Mix reads and writes
				ws.IsolatedRead("input/data.txt")
				ws.IsolatedWrite("output/result.txt", []byte("concurrent data"))
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panics occurred and metrics are consistent
	metrics := ws.GetMetrics()
	if metrics.ReadCount != 100 {
		t.Errorf("Expected 100 reads, got %d", metrics.ReadCount)
	}
	if metrics.WriteCount != 100 {
		t.Errorf("Expected 100 writes, got %d", metrics.WriteCount)
	}
}

// TestEmptyAgentID verifies validation
func TestEmptyAgentID(t *testing.T) {
	config := WorkspaceConfig{}
	_, err := CreateWorkspace("", config)
	if err == nil {
		t.Fatal("CreateWorkspace should reject empty agentID")
	}
	if !strings.Contains(err.Error(), "agentID cannot be empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestPathNormalization verifies that paths are normalized correctly
func TestPathNormalization(t *testing.T) {
	config := WorkspaceConfig{
		InputFiles:  []string{"input/./data.txt", "input/data.txt"}, // Duplicates after normalization
		OutputFiles: []string{"output//result.txt"},
	}

	ws, err := CreateWorkspace("test-agent-normalize", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// Both forms should work (normalized to same path)
	inputPath := filepath.Join(ws.BaseDir, "input/data.txt")
	os.MkdirAll(filepath.Dir(inputPath), 0755)
	os.WriteFile(inputPath, []byte("test"), 0644)

	_, err = ws.IsolatedRead("input/./data.txt")
	if err != nil {
		t.Errorf("Normalized read should succeed: %v", err)
	}

	_, err = ws.IsolatedRead("input/data.txt")
	if err != nil {
		t.Errorf("Normalized read should succeed: %v", err)
	}
}

// TestCleanup verifies workspace cleanup
func TestCleanup(t *testing.T) {
	config := WorkspaceConfig{
		OutputFiles: []string{"output/result.txt"},
	}

	ws, err := CreateWorkspace("test-agent-cleanup", config)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	// Write a file
	ws.IsolatedWrite("output/result.txt", []byte("data"))

	// Verify directory exists
	_, err = os.Stat(ws.BaseDir)
	if err != nil {
		t.Fatalf("Workspace dir should exist: %v", err)
	}

	// Cleanup
	err = ws.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify directory is gone
	_, err = os.Stat(ws.BaseDir)
	if !os.IsNotExist(err) {
		t.Error("Workspace dir should be deleted after cleanup")
	}
}

// BenchmarkIsolatedRead benchmarks read performance
func BenchmarkIsolatedRead(b *testing.B) {
	config := WorkspaceConfig{
		InputFiles: []string{"input/data.txt"},
	}

	ws, _ := CreateWorkspace("bench-agent-read", config)
	defer ws.Cleanup()

	// Setup
	inputPath := filepath.Join(ws.BaseDir, "input/data.txt")
	os.MkdirAll(filepath.Dir(inputPath), 0755)
	os.WriteFile(inputPath, []byte("benchmark data"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws.IsolatedRead("input/data.txt")
	}
}

// BenchmarkIsolatedWrite benchmarks write performance
func BenchmarkIsolatedWrite(b *testing.B) {
	config := WorkspaceConfig{
		OutputFiles: []string{"output/result.txt"},
	}

	ws, _ := CreateWorkspace("bench-agent-write", config)
	defer ws.Cleanup()

	testData := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws.IsolatedWrite("output/result.txt", testData)
	}
}
