package agent1

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestSnapshotDeterminism verifies Π₁: ∀ O. Snapshot(O) == Snapshot(O)
// Proof: Multiple snapshots of the same state produce identical hashes
func TestSnapshotDeterminism(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	// Append some records
	records := []Record{
		{ID: "rec-001", Timestamp: 1000, Content: []byte("data1")},
		{ID: "rec-002", Timestamp: 2000, Content: []byte("data2")},
		{ID: "rec-003", Timestamp: 3000, Content: []byte("data3")},
	}

	for _, rec := range records {
		_, err := ks.Append(ctx, rec)
		if err != nil {
			t.Fatalf("failed to append record %s: %v", rec.ID, err)
		}
	}

	// Take 10 snapshots
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		hash, _, err := ks.Snapshot(ctx)
		if err != nil {
			t.Fatalf("snapshot %d failed: %v", i, err)
		}
		hashes[i] = hash
	}

	// Verify all hashes are identical
	firstHash := hashes[0]
	for i, hash := range hashes {
		if hash != firstHash {
			t.Errorf("snapshot %d hash mismatch: got %s, want %s", i, hash, firstHash)
		}
	}

	t.Logf("✅ Π₁ PASSED: All 10 snapshots produced identical hash: %s", firstHash)
}

// TestAppendIdempotence verifies Π₂: ∀ x. Append(x) ⊕ Append(x) = Append(x)
// Proof: Duplicate ID append fails with ErrDuplicateID
func TestAppendIdempotence(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	record := Record{
		ID:        "rec-dup",
		Timestamp: 1000,
		Content:   []byte("test data"),
	}

	// First append should succeed
	hash1, err := ks.Append(ctx, record)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Second append with same ID should fail
	hash2, err := ks.Append(ctx, record)
	if err != ErrDuplicateID {
		t.Errorf("second append should fail with ErrDuplicateID, got: %v", err)
	}
	if hash2 != "" {
		t.Errorf("second append should return empty hash, got: %s", hash2)
	}

	// Verify only one record exists
	allRecords := ks.GetAllRecords()
	if len(allRecords) != 1 {
		t.Errorf("expected 1 record, got %d", len(allRecords))
	}

	t.Logf("✅ Π₂ PASSED: Duplicate append rejected, hash=%s", hash1)
}

// TestReplayDeterminism verifies Π₃: ∀ E. Replay(E) produces hash H, Replay(E) again produces H
// Proof: Replaying event log twice produces identical hashes
func TestReplayDeterminism(t *testing.T) {
	ctx := context.Background()

	// Create original store and add records
	original := NewKnowledgeStore()
	records := []Record{
		{ID: "rec-r1", Timestamp: 1000, Content: []byte("replay-data-1")},
		{ID: "rec-r2", Timestamp: 2000, Content: []byte("replay-data-2")},
		{ID: "rec-r3", Timestamp: 3000, Content: []byte("replay-data-3")},
	}

	for _, rec := range records {
		_, err := original.Append(ctx, rec)
		if err != nil {
			t.Fatalf("failed to append to original: %v", err)
		}
	}

	// Get original snapshot
	originalHash, _, err := original.Snapshot(ctx)
	if err != nil {
		t.Fatalf("failed to snapshot original: %v", err)
	}

	// Create event log
	events := make([]Event, len(records))
	for i, rec := range records {
		events[i] = Event{
			Type:      "append",
			Record:    rec,
			Timestamp: rec.Timestamp,
		}
	}

	// First replay
	replay1 := NewKnowledgeStore()
	hash1, err := replay1.Replay(ctx, events)
	if err != nil {
		t.Fatalf("first replay failed: %v", err)
	}

	// Second replay
	replay2 := NewKnowledgeStore()
	hash2, err := replay2.Replay(ctx, events)
	if err != nil {
		t.Fatalf("second replay failed: %v", err)
	}

	// Verify all hashes match
	if hash1 != hash2 {
		t.Errorf("replay hashes differ: replay1=%s, replay2=%s", hash1, hash2)
	}
	if hash1 != originalHash {
		t.Errorf("replay hash differs from original: replay=%s, original=%s", hash1, originalHash)
	}

	t.Logf("✅ Π₃ PASSED: Replay produced identical hash: %s", hash1)
}

// TestTamperDetection verifies Π₄: ∀ O, O'. O ≠ O' ⟹ hash(O) ≠ hash(O')
// Proof: Modifying state changes snapshot hash
func TestTamperDetection(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	// Add initial record
	_, err := ks.Append(ctx, Record{
		ID:        "rec-t1",
		Timestamp: 1000,
		Content:   []byte("original"),
	})
	if err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	// Get initial snapshot
	hash1, _, err := ks.Snapshot(ctx)
	if err != nil {
		t.Fatalf("failed to snapshot: %v", err)
	}

	// Verify hash matches
	valid, err := ks.Verify(ctx, hash1)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !valid {
		t.Error("verification should succeed for correct hash")
	}

	// Modify state by adding another record
	_, err = ks.Append(ctx, Record{
		ID:        "rec-t2",
		Timestamp: 2000,
		Content:   []byte("modified"),
	})
	if err != nil {
		t.Fatalf("failed to append second record: %v", err)
	}

	// Get new snapshot
	hash2, _, err := ks.Snapshot(ctx)
	if err != nil {
		t.Fatalf("failed to snapshot after modification: %v", err)
	}

	// Verify hashes are different
	if hash1 == hash2 {
		t.Error("hashes should differ after modification")
	}

	// Verify old hash no longer matches
	valid, err = ks.Verify(ctx, hash1)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if valid {
		t.Error("verification should fail for old hash after modification")
	}

	// Verify new hash matches
	valid, err = ks.Verify(ctx, hash2)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !valid {
		t.Error("verification should succeed for new hash")
	}

	t.Logf("✅ Π₄ PASSED: Tamper detected, hash1=%s, hash2=%s", hash1[:16], hash2[:16])
}

// TestHashesAreSHA256 verifies that hashes use SHA256 and are reproducible
func TestHashesAreSHA256(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	record := Record{
		ID:        "rec-sha",
		Timestamp: 1000,
		Content:   []byte("sha256 test"),
	}

	hash, err := ks.Append(ctx, record)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	// SHA256 produces 64 hex characters
	if len(hash) != 64 {
		t.Errorf("expected SHA256 hash (64 chars), got %d chars: %s", len(hash), hash)
	}

	// Verify it's valid hex
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hash contains non-hex character: %c", c)
		}
	}

	// Snapshot hash should also be SHA256
	snapHash, _, err := ks.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if len(snapHash) != 64 {
		t.Errorf("expected SHA256 snapshot hash (64 chars), got %d chars", len(snapHash))
	}

	t.Logf("✅ SHA256 VERIFIED: record hash=%s, snapshot hash=%s", hash[:16], snapHash[:16])
}

// TestConcurrentAppends verifies thread-safety under concurrent access
func TestConcurrentAppends(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	const numGoroutines = 10
	const recordsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*recordsPerGoroutine)

	// Spawn concurrent appends
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < recordsPerGoroutine; i++ {
				record := Record{
					ID:        fmt.Sprintf("rec-g%d-i%d", goroutineID, i),
					Timestamp: time.Now().UnixNano(),
					Content:   []byte(fmt.Sprintf("data from goroutine %d, record %d", goroutineID, i)),
				}
				_, err := ks.Append(ctx, record)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, record %d: %w", goroutineID, i, err)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("concurrent append error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("encountered %d errors during concurrent appends", errorCount)
	}

	// Verify all records were added
	expectedCount := numGoroutines * recordsPerGoroutine
	allRecords := ks.GetAllRecords()
	if len(allRecords) != expectedCount {
		t.Errorf("expected %d records, got %d", expectedCount, len(allRecords))
	}

	t.Logf("✅ CONCURRENCY PASSED: %d concurrent appends completed successfully", expectedCount)
}

// TestInvalidRecords verifies error handling for invalid inputs
func TestInvalidRecords(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	tests := []struct {
		name        string
		record      Record
		expectedErr error
	}{
		{
			name:        "empty ID",
			record:      Record{ID: "", Timestamp: 1000, Content: []byte("data")},
			expectedErr: ErrInvalidRecord,
		},
		{
			name:        "empty content",
			record:      Record{ID: "rec-1", Timestamp: 1000, Content: []byte{}},
			expectedErr: ErrInvalidRecord,
		},
		{
			name:        "nil content",
			record:      Record{ID: "rec-2", Timestamp: 1000, Content: nil},
			expectedErr: ErrInvalidRecord,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ks.Append(ctx, tt.record)
			if err != tt.expectedErr {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}

	t.Logf("✅ VALIDATION PASSED: Invalid records rejected correctly")
}

// TestLargeDataset verifies performance with many records
func TestLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large dataset test in short mode")
	}

	ctx := context.Background()
	ks := NewKnowledgeStore()

	const numRecords = 1000

	// Append many records
	for i := 0; i < numRecords; i++ {
		record := Record{
			ID:        fmt.Sprintf("rec-%06d", i),
			Timestamp: int64(i * 1000),
			Content:   []byte(fmt.Sprintf("data for record %d", i)),
		}
		_, err := ks.Append(ctx, record)
		if err != nil {
			t.Fatalf("failed to append record %d: %v", i, err)
		}
	}

	// Verify count
	allRecords := ks.GetAllRecords()
	if len(allRecords) != numRecords {
		t.Errorf("expected %d records, got %d", numRecords, len(allRecords))
	}

	// Snapshot should still be deterministic
	hash1, _, err := ks.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot 1 failed: %v", err)
	}

	hash2, _, err := ks.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot 2 failed: %v", err)
	}

	if hash1 != hash2 {
		t.Error("snapshots differ for large dataset")
	}

	t.Logf("✅ LARGE DATASET PASSED: %d records, deterministic hash=%s", numRecords, hash1[:16])
}

// TestReplayFromEvents verifies complete event log replay
func TestReplayFromEvents(t *testing.T) {
	ctx := context.Background()

	// Original store
	original := NewKnowledgeStore()
	// Note: Not setting metadata here because it's not part of event replay

	records := []Record{
		{ID: "rec-e1", Timestamp: 1000, Content: []byte("event-1"), Metadata: map[string]string{"type": "test"}},
		{ID: "rec-e2", Timestamp: 2000, Content: []byte("event-2")},
		{ID: "rec-e3", Timestamp: 3000, Content: []byte("event-3")},
		{ID: "rec-e4", Timestamp: 4000, Content: []byte("event-4")},
		{ID: "rec-e5", Timestamp: 5000, Content: []byte("event-5")},
	}

	events := make([]Event, len(records))
	for i, rec := range records {
		_, err := original.Append(ctx, rec)
		if err != nil {
			t.Fatalf("failed to append to original: %v", err)
		}
		events[i] = Event{
			Type:      "append",
			Record:    rec,
			Timestamp: rec.Timestamp,
		}
	}

	// Get original hash
	originalHash, _, err := original.Snapshot(ctx)
	if err != nil {
		t.Fatalf("failed to snapshot original: %v", err)
	}

	// Replay into new store
	replayed := NewKnowledgeStore()
	replayHash, err := replayed.Replay(ctx, events)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Verify hash matches
	if replayHash != originalHash {
		t.Errorf("replay hash mismatch: got %s, want %s", replayHash, originalHash)
	}

	// Verify record count
	replayedRecords := replayed.GetAllRecords()
	if len(replayedRecords) != len(records) {
		t.Errorf("replayed record count mismatch: got %d, want %d", len(replayedRecords), len(records))
	}

	t.Logf("✅ REPLAY PASSED: %d events replayed, hash=%s", len(events), replayHash[:16])
}

// TestMonotonicity verifies Q₁: version counter only increases
func TestMonotonicity(t *testing.T) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	previousVersion := ks.GetVersion()
	if previousVersion != 0 {
		t.Errorf("initial version should be 0, got %d", previousVersion)
	}

	for i := 1; i <= 10; i++ {
		record := Record{
			ID:        fmt.Sprintf("rec-m%d", i),
			Timestamp: int64(i * 1000),
			Content:   []byte(fmt.Sprintf("data-%d", i)),
		}
		_, err := ks.Append(ctx, record)
		if err != nil {
			t.Fatalf("append %d failed: %v", i, err)
		}

		currentVersion := ks.GetVersion()
		if currentVersion <= previousVersion {
			t.Errorf("version did not increase: previous=%d, current=%d", previousVersion, currentVersion)
		}
		if currentVersion != int64(i) {
			t.Errorf("version mismatch at iteration %d: got %d, want %d", i, currentVersion, i)
		}
		previousVersion = currentVersion
	}

	t.Logf("✅ Q₁ MONOTONICITY PASSED: version increased from 0 to %d", previousVersion)
}

// BenchmarkAppend measures append operation performance
func BenchmarkAppend(b *testing.B) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record := Record{
			ID:        fmt.Sprintf("bench-rec-%d", i),
			Timestamp: int64(i),
			Content:   []byte("benchmark data"),
		}
		_, err := ks.Append(ctx, record)
		if err != nil {
			b.Fatalf("append failed: %v", err)
		}
	}
}

// BenchmarkSnapshot measures snapshot operation performance
func BenchmarkSnapshot(b *testing.B) {
	ctx := context.Background()
	ks := NewKnowledgeStore()

	// Pre-populate with records
	for i := 0; i < 1000; i++ {
		record := Record{
			ID:        fmt.Sprintf("rec-%d", i),
			Timestamp: int64(i),
			Content:   []byte(fmt.Sprintf("data-%d", i)),
		}
		_, _ = ks.Append(ctx, record)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := ks.Snapshot(ctx)
		if err != nil {
			b.Fatalf("snapshot failed: %v", err)
		}
	}
}
