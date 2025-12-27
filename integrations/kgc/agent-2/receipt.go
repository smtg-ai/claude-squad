// Package agent2 implements receipt chaining and tamper detection for KGC substrate
package agent2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// Receipt represents a cryptographically verifiable execution record
// Implements the Receipt interface from SUBSTRATE_INTERFACES.md
type Receipt struct {
	// Execution metadata
	ExecutionID    string            `json:"execution_id"`    // UUID for this run
	AgentID        string            `json:"agent_id"`        // Which agent (0-9)
	Timestamp      int64             `json:"timestamp"`       // Unix nanoseconds
	ToolchainVer   string            `json:"toolchain_ver"`   // Go version, etc.

	// Determinism proof
	InputHash      string            `json:"input_hash"`      // SHA256(inputs)
	OutputHash     string            `json:"output_hash"`     // SHA256(outputs)
	ProofArtifacts map[string]string `json:"proof_artifacts"` // test logs, diffs, snapshots

	// Replay instructions
	ReplayScript   string            `json:"replay_script"`   // Bash script that reproduces this exact run

	// Composition law
	CompositionOp  string            `json:"composition_op"`  // How this patches merge with siblings
	ConflictPolicy string            `json:"conflict_policy"` // "fail_fast" | "merge" | "skip"
}

// CreateReceipt generates a new receipt from before/after states and replay script
// This is the primary constructor for receipts
// O → A = μ(O) where μ is the hash chain creation function
func CreateReceipt(before, after []byte, replayScript string, agentID string) (*Receipt, error) {
	if len(before) == 0 {
		return nil, fmt.Errorf("before state cannot be empty")
	}
	if len(after) == 0 {
		return nil, fmt.Errorf("after state cannot be empty")
	}
	if replayScript == "" {
		return nil, fmt.Errorf("replay script cannot be empty")
	}

	// Compute deterministic hashes
	inputHash := computeHash(before)
	outputHash := computeHash(after)

	// Generate receipt
	receipt := &Receipt{
		ExecutionID:    generateExecutionID(),
		AgentID:        agentID,
		Timestamp:      time.Now().UnixNano(),
		ToolchainVer:   "go1.21",
		InputHash:      inputHash,
		OutputHash:     outputHash,
		ReplayScript:   replayScript,
		ProofArtifacts: make(map[string]string),
		CompositionOp:  "append",
		ConflictPolicy: "fail_fast",
	}

	return receipt, nil
}

// VerifyReceipt validates the integrity of a receipt
// Returns true if the receipt structure is valid and hashes are well-formed
// Π: Receipt → bool (proof target: integrity verification)
func VerifyReceipt(receipt *Receipt) (bool, error) {
	if receipt == nil {
		return false, fmt.Errorf("receipt is nil")
	}

	// Verify required fields
	if receipt.ExecutionID == "" {
		return false, fmt.Errorf("missing execution_id")
	}
	if receipt.InputHash == "" {
		return false, fmt.Errorf("missing input_hash")
	}
	if receipt.OutputHash == "" {
		return false, fmt.Errorf("missing output_hash")
	}
	if receipt.ReplayScript == "" {
		return false, fmt.Errorf("missing replay_script")
	}

	// Verify hash format (SHA256 = 64 hex chars)
	if len(receipt.InputHash) != 64 {
		return false, fmt.Errorf("invalid input_hash length: %d", len(receipt.InputHash))
	}
	if len(receipt.OutputHash) != 64 {
		return false, fmt.Errorf("invalid output_hash length: %d", len(receipt.OutputHash))
	}

	// Verify hash characters are hex
	if !isHexString(receipt.InputHash) {
		return false, fmt.Errorf("input_hash is not valid hex")
	}
	if !isHexString(receipt.OutputHash) {
		return false, fmt.Errorf("output_hash is not valid hex")
	}

	// Verify timestamp is reasonable (not in future, not too far in past)
	now := time.Now().UnixNano()
	if receipt.Timestamp > now {
		return false, fmt.Errorf("timestamp is in the future")
	}
	// Allow up to 1 year in the past
	if receipt.Timestamp < now-365*24*60*60*1e9 {
		return false, fmt.Errorf("timestamp is too far in the past")
	}

	return true, nil
}

// ChainReceipts verifies that two receipts can be chained together
// Returns true if R1.OutputHash == R2.InputHash
// This implements the core receipt chaining invariant
// Q: ∀ R1, R2. ChainReceipts(R1, R2) ⟹ R1.OutputHash = R2.InputHash
func ChainReceipts(r1, r2 *Receipt) (bool, error) {
	if r1 == nil || r2 == nil {
		return false, fmt.Errorf("cannot chain nil receipts")
	}

	// Verify both receipts are individually valid
	valid1, err1 := VerifyReceipt(r1)
	if !valid1 {
		return false, fmt.Errorf("receipt 1 is invalid: %v", err1)
	}

	valid2, err2 := VerifyReceipt(r2)
	if !valid2 {
		return false, fmt.Errorf("receipt 2 is invalid: %v", err2)
	}

	// Verify chaining property: R1.OutputHash == R2.InputHash
	if r1.OutputHash != r2.InputHash {
		return false, fmt.Errorf("chain broken: R1.OutputHash (%s...) != R2.InputHash (%s...)",
			r1.OutputHash[:8], r2.InputHash[:8])
	}

	// Verify temporal ordering (R1 must come before R2)
	if r1.Timestamp >= r2.Timestamp {
		return false, fmt.Errorf("temporal violation: R1.Timestamp (%d) >= R2.Timestamp (%d)",
			r1.Timestamp, r2.Timestamp)
	}

	return true, nil
}

// SerializeReceipt converts a receipt to JSON bytes
// Used for storage, transmission, and hashing
func SerializeReceipt(receipt *Receipt) ([]byte, error) {
	if receipt == nil {
		return nil, fmt.Errorf("cannot serialize nil receipt")
	}
	return json.Marshal(receipt)
}

// DeserializeReceipt parses a receipt from JSON bytes
func DeserializeReceipt(data []byte) (*Receipt, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot deserialize empty data")
	}

	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}

	return &receipt, nil
}

// DetectTamper checks if a receipt has been tampered with by re-hashing the content
// For a full tamper check, you would re-execute the ReplayScript and compare hashes
// This is a structural integrity check
func DetectTamper(receipt *Receipt, originalJSON []byte) (bool, error) {
	if receipt == nil {
		return false, fmt.Errorf("receipt is nil")
	}
	if len(originalJSON) == 0 {
		return false, fmt.Errorf("original JSON is empty")
	}

	// Serialize the current receipt
	currentJSON, err := SerializeReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to serialize receipt: %w", err)
	}

	// Compare hashes of original vs current JSON
	originalHash := computeHash(originalJSON)
	currentHash := computeHash(currentJSON)

	// If hashes differ, tampering detected
	tampered := originalHash != currentHash

	return tampered, nil
}

// Helper functions

// computeHash computes SHA256 hash of data and returns hex string
func computeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// generateExecutionID creates a deterministic execution ID
// In production, this would use a proper UUID library
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}

// isHexString checks if a string contains only valid hex characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
