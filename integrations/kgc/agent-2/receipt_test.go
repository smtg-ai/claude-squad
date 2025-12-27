package agent2

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestCreateReceipt validates basic receipt creation
func TestCreateReceipt(t *testing.T) {
	before := []byte("initial state")
	after := []byte("modified state")
	replayScript := "#!/bin/bash\necho 'replay'"
	agentID := "agent-2"

	receipt, err := CreateReceipt(before, after, replayScript, agentID)
	if err != nil {
		t.Fatalf("CreateReceipt failed: %v", err)
	}

	// Verify all required fields are populated
	if receipt.ExecutionID == "" {
		t.Error("ExecutionID is empty")
	}
	if receipt.AgentID != agentID {
		t.Errorf("AgentID = %s, want %s", receipt.AgentID, agentID)
	}
	if receipt.InputHash == "" {
		t.Error("InputHash is empty")
	}
	if receipt.OutputHash == "" {
		t.Error("OutputHash is empty")
	}
	if receipt.ReplayScript != replayScript {
		t.Errorf("ReplayScript = %s, want %s", receipt.ReplayScript, replayScript)
	}
	if receipt.Timestamp == 0 {
		t.Error("Timestamp is zero")
	}

	// Verify hashes are different (before != after)
	if receipt.InputHash == receipt.OutputHash {
		t.Error("InputHash and OutputHash should be different for different states")
	}

	// Verify hash lengths (SHA256 = 64 hex chars)
	if len(receipt.InputHash) != 64 {
		t.Errorf("InputHash length = %d, want 64", len(receipt.InputHash))
	}
	if len(receipt.OutputHash) != 64 {
		t.Errorf("OutputHash length = %d, want 64", len(receipt.OutputHash))
	}
}

// TestCreateReceiptDeterminism verifies that same inputs produce same hashes
func TestCreateReceiptDeterminism(t *testing.T) {
	before := []byte("state A")
	after := []byte("state B")
	script := "echo determinism"
	agentID := "agent-2"

	r1, err1 := CreateReceipt(before, after, script, agentID)
	if err1 != nil {
		t.Fatalf("CreateReceipt 1 failed: %v", err1)
	}

	// Small delay to ensure different timestamp
	time.Sleep(1 * time.Millisecond)

	r2, err2 := CreateReceipt(before, after, script, agentID)
	if err2 != nil {
		t.Fatalf("CreateReceipt 2 failed: %v", err2)
	}

	// Hashes should be identical (deterministic)
	if r1.InputHash != r2.InputHash {
		t.Errorf("InputHash not deterministic: %s != %s", r1.InputHash, r2.InputHash)
	}
	if r1.OutputHash != r2.OutputHash {
		t.Errorf("OutputHash not deterministic: %s != %s", r1.OutputHash, r2.OutputHash)
	}

	// Timestamps should be different (not deterministic for metadata)
	if r1.Timestamp == r2.Timestamp {
		t.Error("Timestamps should differ between executions")
	}
}

// TestCreateReceiptValidation tests error cases
func TestCreateReceiptValidation(t *testing.T) {
	tests := []struct {
		name         string
		before       []byte
		after        []byte
		replayScript string
		wantErr      bool
	}{
		{
			name:         "empty before state",
			before:       []byte{},
			after:        []byte("after"),
			replayScript: "script",
			wantErr:      true,
		},
		{
			name:         "empty after state",
			before:       []byte("before"),
			after:        []byte{},
			replayScript: "script",
			wantErr:      true,
		},
		{
			name:         "empty replay script",
			before:       []byte("before"),
			after:        []byte("after"),
			replayScript: "",
			wantErr:      true,
		},
		{
			name:         "valid inputs",
			before:       []byte("before"),
			after:        []byte("after"),
			replayScript: "#!/bin/bash",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateReceipt(tt.before, tt.after, tt.replayScript, "agent-2")
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateReceipt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVerifyReceipt validates receipt verification
func TestVerifyReceipt(t *testing.T) {
	// Create a valid receipt
	before := []byte("before")
	after := []byte("after")
	receipt, err := CreateReceipt(before, after, "#!/bin/bash", "agent-2")
	if err != nil {
		t.Fatalf("CreateReceipt failed: %v", err)
	}

	// Verify it's valid
	valid, err := VerifyReceipt(receipt)
	if !valid {
		t.Errorf("VerifyReceipt failed: %v", err)
	}
	if err != nil {
		t.Errorf("VerifyReceipt returned error: %v", err)
	}
}

// TestVerifyReceiptInvalid tests various invalid receipt scenarios
func TestVerifyReceiptInvalid(t *testing.T) {
	// Create a valid receipt first
	validReceipt, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")

	tests := []struct {
		name    string
		receipt *Receipt
		wantErr bool
	}{
		{
			name:    "nil receipt",
			receipt: nil,
			wantErr: true,
		},
		{
			name: "missing execution_id",
			receipt: &Receipt{
				ExecutionID:  "",
				InputHash:    validReceipt.InputHash,
				OutputHash:   validReceipt.OutputHash,
				ReplayScript: "script",
				Timestamp:    time.Now().UnixNano(),
			},
			wantErr: true,
		},
		{
			name: "missing input_hash",
			receipt: &Receipt{
				ExecutionID:  "exec_123",
				InputHash:    "",
				OutputHash:   validReceipt.OutputHash,
				ReplayScript: "script",
				Timestamp:    time.Now().UnixNano(),
			},
			wantErr: true,
		},
		{
			name: "invalid input_hash length",
			receipt: &Receipt{
				ExecutionID:  "exec_123",
				InputHash:    "tooshort",
				OutputHash:   validReceipt.OutputHash,
				ReplayScript: "script",
				Timestamp:    time.Now().UnixNano(),
			},
			wantErr: true,
		},
		{
			name: "non-hex input_hash",
			receipt: &Receipt{
				ExecutionID:  "exec_123",
				InputHash:    "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
				OutputHash:   validReceipt.OutputHash,
				ReplayScript: "script",
				Timestamp:    time.Now().UnixNano(),
			},
			wantErr: true,
		},
		{
			name: "future timestamp",
			receipt: &Receipt{
				ExecutionID:  "exec_123",
				InputHash:    validReceipt.InputHash,
				OutputHash:   validReceipt.OutputHash,
				ReplayScript: "script",
				Timestamp:    time.Now().UnixNano() + 1e12, // Far future
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := VerifyReceipt(tt.receipt)
			if valid {
				t.Error("VerifyReceipt should return false for invalid receipt")
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyReceipt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestChainReceipts validates receipt chaining
func TestChainReceipts(t *testing.T) {
	// Create state progression: A → B → C
	stateA := []byte("state A")
	stateB := []byte("state B")
	stateC := []byte("state C")

	// R1: A → B
	r1, err := CreateReceipt(stateA, stateB, "step1.sh", "agent-2")
	if err != nil {
		t.Fatalf("CreateReceipt R1 failed: %v", err)
	}

	// Small delay for timestamp ordering
	time.Sleep(1 * time.Millisecond)

	// R2: B → C (must chain from R1)
	r2, err := CreateReceipt(stateB, stateC, "step2.sh", "agent-2")
	if err != nil {
		t.Fatalf("CreateReceipt R2 failed: %v", err)
	}

	// Verify R1 → R2 chain (R1.OutputHash should equal R2.InputHash)
	valid, err := ChainReceipts(r1, r2)
	if !valid {
		t.Errorf("ChainReceipts(R1, R2) failed: %v", err)
	}
	if err != nil {
		t.Errorf("ChainReceipts returned error: %v", err)
	}

	// Verify the actual hash equality
	if r1.OutputHash != r2.InputHash {
		t.Errorf("Chain broken: R1.OutputHash (%s) != R2.InputHash (%s)",
			r1.OutputHash[:16], r2.InputHash[:16])
	}
}

// TestChainReceiptsBroken tests that broken chains are detected
func TestChainReceiptsBroken(t *testing.T) {
	// Create two independent receipts that don't chain
	stateA := []byte("state A")
	stateB := []byte("state B")
	stateX := []byte("state X")
	stateY := []byte("state Y")

	// R1: A → B
	r1, _ := CreateReceipt(stateA, stateB, "step1.sh", "agent-2")

	time.Sleep(1 * time.Millisecond)

	// R2: X → Y (independent, doesn't chain from R1)
	r2, _ := CreateReceipt(stateX, stateY, "step2.sh", "agent-2")

	// Verify chain is broken
	valid, err := ChainReceipts(r1, r2)
	if valid {
		t.Error("ChainReceipts should return false for broken chain")
	}
	if err == nil {
		t.Error("ChainReceipts should return error for broken chain")
	}
	if !strings.Contains(err.Error(), "chain broken") {
		t.Errorf("Error should mention broken chain, got: %v", err)
	}
}

// TestChainReceiptsNil tests nil receipt handling
func TestChainReceiptsNil(t *testing.T) {
	r1, _ := CreateReceipt([]byte("a"), []byte("b"), "script", "agent-2")

	tests := []struct {
		name string
		r1   *Receipt
		r2   *Receipt
	}{
		{"both nil", nil, nil},
		{"r1 nil", nil, r1},
		{"r2 nil", r1, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := ChainReceipts(tt.r1, tt.r2)
			if valid {
				t.Error("ChainReceipts should return false for nil receipts")
			}
			if err == nil {
				t.Error("ChainReceipts should return error for nil receipts")
			}
		})
	}
}

// TestSerializeReceipt tests JSON serialization
func TestSerializeReceipt(t *testing.T) {
	receipt, err := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")
	if err != nil {
		t.Fatalf("CreateReceipt failed: %v", err)
	}

	// Serialize
	data, err := SerializeReceipt(receipt)
	if err != nil {
		t.Fatalf("SerializeReceipt failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Serialized data is not valid JSON: %v", err)
	}

	// Verify required fields present
	requiredFields := []string{"execution_id", "agent_id", "input_hash", "output_hash", "replay_script"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Serialized JSON missing field: %s", field)
		}
	}
}

// TestDeserializeReceipt tests JSON deserialization
func TestDeserializeReceipt(t *testing.T) {
	// Create and serialize a receipt
	original, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")
	data, _ := SerializeReceipt(original)

	// Deserialize
	deserialized, err := DeserializeReceipt(data)
	if err != nil {
		t.Fatalf("DeserializeReceipt failed: %v", err)
	}

	// Verify key fields match
	if deserialized.ExecutionID != original.ExecutionID {
		t.Errorf("ExecutionID mismatch: got %s, want %s", deserialized.ExecutionID, original.ExecutionID)
	}
	if deserialized.InputHash != original.InputHash {
		t.Errorf("InputHash mismatch")
	}
	if deserialized.OutputHash != original.OutputHash {
		t.Errorf("OutputHash mismatch")
	}
	if deserialized.ReplayScript != original.ReplayScript {
		t.Errorf("ReplayScript mismatch")
	}
}

// TestDetectTamperNone verifies untampered receipts
func TestDetectTamperNone(t *testing.T) {
	receipt, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")
	originalJSON, _ := SerializeReceipt(receipt)

	tampered, err := DetectTamper(receipt, originalJSON)
	if err != nil {
		t.Fatalf("DetectTamper failed: %v", err)
	}
	if tampered {
		t.Error("DetectTamper should return false for untampered receipt")
	}
}

// TestDetectTamperModified verifies tamper detection
func TestDetectTamperModified(t *testing.T) {
	receipt, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")
	originalJSON, _ := SerializeReceipt(receipt)

	// Tamper with the receipt
	receipt.OutputHash = "0000000000000000000000000000000000000000000000000000000000000000"

	tampered, err := DetectTamper(receipt, originalJSON)
	if err != nil {
		t.Fatalf("DetectTamper failed: %v", err)
	}
	if !tampered {
		t.Error("DetectTamper should return true for tampered receipt")
	}
}

// TestDeliberateCorruption validates deliberate tampering detection
// This is a critical security test - MUST detect ALL tampering
func TestDeliberateCorruption(t *testing.T) {
	original, _ := CreateReceipt([]byte("original state"), []byte("modified state"), "script", "agent-2")
	originalJSON, _ := SerializeReceipt(original)

	tests := []struct {
		name      string
		corruptor func(*Receipt)
	}{
		{
			name: "corrupt input hash",
			corruptor: func(r *Receipt) {
				r.InputHash = "AAAA" + r.InputHash[4:]
			},
		},
		{
			name: "corrupt output hash",
			corruptor: func(r *Receipt) {
				r.OutputHash = "BBBB" + r.OutputHash[4:]
			},
		},
		{
			name: "corrupt replay script",
			corruptor: func(r *Receipt) {
				r.ReplayScript = "#!/bin/bash\nrm -rf /"
			},
		},
		{
			name: "corrupt agent ID",
			corruptor: func(r *Receipt) {
				r.AgentID = "malicious-agent"
			},
		},
		{
			name: "corrupt timestamp",
			corruptor: func(r *Receipt) {
				r.Timestamp = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy and corrupt it
			corrupted := &Receipt{}
			data, _ := SerializeReceipt(original)
			DeserializeReceipt(data) // Re-parse to ensure independence
			*corrupted = *original
			tt.corruptor(corrupted)

			// Detect tampering
			tampered, err := DetectTamper(corrupted, originalJSON)
			if err != nil {
				t.Fatalf("DetectTamper failed: %v", err)
			}
			if !tampered {
				t.Errorf("DetectTamper MUST detect deliberate corruption: %s", tt.name)
			}
		})
	}
}

// TestTamperDetectionPerformance verifies tamper detection is <1ms
func TestTamperDetectionPerformance(t *testing.T) {
	receipt, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")
	originalJSON, _ := SerializeReceipt(receipt)

	// Run multiple iterations to ensure consistent performance
	iterations := 100
	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := DetectTamper(receipt, originalJSON)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("DetectTamper failed: %v", err)
		}

		// Requirement: tamper detection must be <1ms
		if elapsed > 1*time.Millisecond {
			t.Errorf("DetectTamper took %v, must be <1ms", elapsed)
		}
	}
}

// BenchmarkCreateReceipt benchmarks receipt creation
func BenchmarkCreateReceipt(b *testing.B) {
	before := []byte("before state with some content")
	after := []byte("after state with different content")
	script := "#!/bin/bash\necho test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateReceipt(before, after, script, "agent-2")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkVerifyReceipt benchmarks receipt verification
func BenchmarkVerifyReceipt(b *testing.B) {
	receipt, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := VerifyReceipt(receipt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChainReceipts benchmarks receipt chaining
func BenchmarkChainReceipts(b *testing.B) {
	stateA := []byte("state A")
	stateB := []byte("state B")
	stateC := []byte("state C")

	r1, _ := CreateReceipt(stateA, stateB, "step1", "agent-2")
	time.Sleep(1 * time.Millisecond)
	r2, _ := CreateReceipt(stateB, stateC, "step2", "agent-2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ChainReceipts(r1, r2)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDetectTamper benchmarks tamper detection
func BenchmarkDetectTamper(b *testing.B) {
	receipt, _ := CreateReceipt([]byte("before"), []byte("after"), "script", "agent-2")
	originalJSON, _ := SerializeReceipt(receipt)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DetectTamper(receipt, originalJSON)
		if err != nil {
			b.Fatal(err)
		}
	}
}
