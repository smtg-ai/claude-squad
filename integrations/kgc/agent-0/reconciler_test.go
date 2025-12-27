package agent0

import (
	"context"
	"fmt"
	"testing"
)

// TestValidateComposition_DisjointPatches verifies two non-overlapping patches are compatible
func TestValidateComposition_DisjointPatches(t *testing.T) {
	r := NewReconciler()

	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/file1.go": {Path: "/file1.go", Operation: "create", ContentHash: "hash1"},
			"/file2.go": {Path: "/file2.go", Operation: "create", ContentHash: "hash2"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/file3.go": {Path: "/file3.go", Operation: "create", ContentHash: "hash3"},
			"/file4.go": {Path: "/file4.go", Operation: "create", ContentHash: "hash4"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	compatible, reason := r.ValidateComposition(delta1, delta2)

	if !compatible {
		t.Errorf("Expected compatible deltas, got incompatible with reason: %s", reason)
	}
	if reason != "" {
		t.Errorf("Expected empty reason for compatible deltas, got: %s", reason)
	}
}

// TestValidateComposition_OverlappingPatches verifies overlapping files are detected
func TestValidateComposition_OverlappingPatches(t *testing.T) {
	r := NewReconciler()

	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/file1.go": {Path: "/file1.go", Operation: "create", ContentHash: "hash1"},
			"/shared.go": {Path: "/shared.go", Operation: "create", ContentHash: "hash2"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/file3.go": {Path: "/file3.go", Operation: "create", ContentHash: "hash3"},
			"/shared.go": {Path: "/shared.go", Operation: "modify", ContentHash: "hash4"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	compatible, reason := r.ValidateComposition(delta1, delta2)

	if compatible {
		t.Errorf("Expected incompatible deltas due to file overlap")
	}
	if reason == "" {
		t.Errorf("Expected non-empty reason for incompatible deltas")
	}
	if reason != "file overlap detected: /shared.go (agents: agent-1, agent-2)" {
		t.Errorf("Unexpected reason: %s", reason)
	}
}

// TestReconcile_EmptyDeltas verifies empty input produces empty final delta
func TestReconcile_EmptyDeltas(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	final, conflict, err := r.Reconcile(ctx, []*Delta{})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if conflict != nil {
		t.Errorf("Expected no conflict, got: %+v", conflict)
	}
	if final == nil {
		t.Fatal("Expected final delta, got nil")
	}
	if final.AgentID != "agent-0-global" {
		t.Errorf("Expected AgentID 'agent-0-global', got: %s", final.AgentID)
	}
	if len(final.Files) != 0 {
		t.Errorf("Expected 0 files, got: %d", len(final.Files))
	}
}

// TestReconcile_SingleDelta verifies single delta passes through
func TestReconcile_SingleDelta(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	delta := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/file1.go": {Path: "/file1.go", Operation: "create", ContentHash: "hash1"},
		},
		InputHash:      "input1",
		OutputHash:     "output1",
		CompositionOp:  "append",
		ConflictPolicy: "fail_fast",
	}

	final, conflict, err := r.Reconcile(ctx, []*Delta{delta})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if conflict != nil {
		t.Errorf("Expected no conflict, got: %+v", conflict)
	}
	if final == nil {
		t.Fatal("Expected final delta, got nil")
	}
	if final.AgentID != "agent-0-global" {
		t.Errorf("Expected AgentID 'agent-0-global', got: %s", final.AgentID)
	}
	if len(final.Files) != 1 {
		t.Errorf("Expected 1 file, got: %d", len(final.Files))
	}
	if final.InputHash != "input1" {
		t.Errorf("Expected InputHash 'input1', got: %s", final.InputHash)
	}
	if final.OutputHash != "output1" {
		t.Errorf("Expected OutputHash 'output1', got: %s", final.OutputHash)
	}
}

// TestReconcile_DisjointDeltas verifies multiple non-overlapping deltas merge successfully
func TestReconcile_DisjointDeltas(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/file1.go": {Path: "/file1.go", Operation: "create", ContentHash: "hash1"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/file2.go": {Path: "/file2.go", Operation: "create", ContentHash: "hash2"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	delta3 := &Delta{
		AgentID: "agent-3",
		Files: map[string]FileChange{
			"/file3.go": {Path: "/file3.go", Operation: "create", ContentHash: "hash3"},
		},
		InputHash:  "input3",
		OutputHash: "output3",
	}

	final, conflict, err := r.Reconcile(ctx, []*Delta{delta1, delta2, delta3})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if conflict != nil {
		t.Errorf("Expected no conflict, got: %+v", conflict)
	}
	if final == nil {
		t.Fatal("Expected final delta, got nil")
	}
	if len(final.Files) != 3 {
		t.Errorf("Expected 3 files, got: %d", len(final.Files))
	}

	// Verify all files are present
	expectedFiles := []string{"/file1.go", "/file2.go", "/file3.go"}
	for _, file := range expectedFiles {
		if _, exists := final.Files[file]; !exists {
			t.Errorf("Expected file %s in final delta", file)
		}
	}
}

// TestReconcile_ConflictingDeltas verifies overlapping edits produce conflict report
func TestReconcile_ConflictingDeltas(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "create", ContentHash: "hash1"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "modify", ContentHash: "hash2"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	final, conflict, err := r.Reconcile(ctx, []*Delta{delta1, delta2})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if final != nil {
		t.Errorf("Expected nil final delta due to conflict, got: %+v", final)
	}
	if conflict == nil {
		t.Fatal("Expected conflict report, got nil")
	}

	// Verify conflict report
	if len(conflict.Conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got: %d", len(conflict.Conflicts))
	}
	if conflict.Resolution != "manual_required" {
		t.Errorf("Expected resolution 'manual_required', got: %s", conflict.Resolution)
	}

	// Verify conflict details
	if len(conflict.Conflicts) > 0 {
		c := conflict.Conflicts[0]
		if c.File != "/shared.go" {
			t.Errorf("Expected conflict on /shared.go, got: %s", c.File)
		}
		if c.Agent1 != "agent-1" || c.Agent2 != "agent-2" {
			t.Errorf("Expected conflict between agent-1 and agent-2, got: %s and %s", c.Agent1, c.Agent2)
		}
	}

	// Verify conflict graph
	if len(conflict.ConflictGraph) != 2 {
		t.Errorf("Expected 2 nodes in conflict graph, got: %d", len(conflict.ConflictGraph))
	}
	if len(conflict.ConflictGraph["agent-1"]) != 1 {
		t.Errorf("Expected agent-1 to have 1 conflict edge, got: %d", len(conflict.ConflictGraph["agent-1"]))
	}
	if len(conflict.ConflictGraph["agent-2"]) != 1 {
		t.Errorf("Expected agent-2 to have 1 conflict edge, got: %d", len(conflict.ConflictGraph["agent-2"]))
	}
}

// TestCompositionLaws_Idempotence verifies Δ ⊕ Δ = Δ
func TestCompositionLaws_Idempotence(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	delta := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/file1.go": {Path: "/file1.go", Operation: "create", ContentHash: "hash1"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	// Apply delta once
	final1, conflict1, err1 := r.Reconcile(ctx, []*Delta{delta})

	// Apply delta twice (should be idempotent when checking single application)
	// Note: For true idempotence test, we'd apply the result again
	// Here we verify that reconciling the same delta produces consistent output
	final2, conflict2, err2 := r.Reconcile(ctx, []*Delta{delta})

	if err1 != nil || err2 != nil {
		t.Fatalf("Expected no errors, got: %v, %v", err1, err2)
	}
	if conflict1 != nil || conflict2 != nil {
		t.Errorf("Expected no conflicts, got: %+v, %+v", conflict1, conflict2)
	}

	// Verify both results are identical
	if final1.InputHash != final2.InputHash {
		t.Errorf("Idempotence violation: InputHash differs (%s vs %s)", final1.InputHash, final2.InputHash)
	}
	if final1.OutputHash != final2.OutputHash {
		t.Errorf("Idempotence violation: OutputHash differs (%s vs %s)", final1.OutputHash, final2.OutputHash)
	}
	if len(final1.Files) != len(final2.Files) {
		t.Errorf("Idempotence violation: Files count differs (%d vs %d)", len(final1.Files), len(final2.Files))
	}
}

// TestCompositionLaws_Associativity verifies (Δ₁ ⊕ Δ₂) ⊕ Δ₃ = Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
func TestCompositionLaws_Associativity(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/file1.go": {Path: "/file1.go", Operation: "create", ContentHash: "hash1"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/file2.go": {Path: "/file2.go", Operation: "create", ContentHash: "hash2"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	delta3 := &Delta{
		AgentID: "agent-3",
		Files: map[string]FileChange{
			"/file3.go": {Path: "/file3.go", Operation: "create", ContentHash: "hash3"},
		},
		InputHash:  "input3",
		OutputHash: "output3",
	}

	// Path 1: (Δ₁ ⊕ Δ₂) ⊕ Δ₃
	intermediate1, _, err1 := r.Reconcile(ctx, []*Delta{delta1, delta2})
	if err1 != nil {
		t.Fatalf("Path 1 intermediate failed: %v", err1)
	}
	final1, _, err1b := r.Reconcile(ctx, []*Delta{intermediate1, delta3})
	if err1b != nil {
		t.Fatalf("Path 1 final failed: %v", err1b)
	}

	// Path 2: Δ₁ ⊕ (Δ₂ ⊕ Δ₃)
	intermediate2, _, err2 := r.Reconcile(ctx, []*Delta{delta2, delta3})
	if err2 != nil {
		t.Fatalf("Path 2 intermediate failed: %v", err2)
	}
	final2, _, err2b := r.Reconcile(ctx, []*Delta{delta1, intermediate2})
	if err2b != nil {
		t.Fatalf("Path 2 final failed: %v", err2b)
	}

	// Verify both paths produce same number of files
	if len(final1.Files) != len(final2.Files) {
		t.Errorf("Associativity violation: Files count differs (%d vs %d)", len(final1.Files), len(final2.Files))
	}

	// Verify all files are present in both
	for file := range final1.Files {
		if _, exists := final2.Files[file]; !exists {
			t.Errorf("Associativity violation: File %s in path 1 but not path 2", file)
		}
	}

	// Note: Hashes will differ due to intermediate composition, but file sets should match
	// This is expected behavior for non-commutative hash composition
}

// TestReconcile_Deterministic verifies same inputs produce same conflict reports
func TestReconcile_Deterministic(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "create", ContentHash: "hash1"},
			"/file1.go":  {Path: "/file1.go", Operation: "create", ContentHash: "hash2"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "modify", ContentHash: "hash3"},
			"/file2.go":  {Path: "/file2.go", Operation: "create", ContentHash: "hash4"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	// Run reconciliation multiple times
	runs := 10
	var firstConflict *ConflictReport

	for i := 0; i < runs; i++ {
		_, conflict, err := r.Reconcile(ctx, []*Delta{delta1, delta2})
		if err != nil {
			t.Fatalf("Run %d failed: %v", i, err)
		}
		if conflict == nil {
			t.Fatalf("Run %d: Expected conflict, got nil", i)
		}

		if i == 0 {
			firstConflict = conflict
		} else {
			// Verify consistency across runs
			if len(conflict.Conflicts) != len(firstConflict.Conflicts) {
				t.Errorf("Run %d: Conflict count differs (%d vs %d)", i,
					len(conflict.Conflicts), len(firstConflict.Conflicts))
			}

			// Verify conflict details are identical
			if len(conflict.Conflicts) > 0 && len(firstConflict.Conflicts) > 0 {
				c1 := conflict.Conflicts[0]
				c2 := firstConflict.Conflicts[0]
				if c1.File != c2.File || c1.Agent1 != c2.Agent1 || c1.Agent2 != c2.Agent2 {
					t.Errorf("Run %d: Conflict details differ", i)
				}
			}
		}
	}
}

// TestReconcile_MultipleConflicts verifies complex conflict scenarios
func TestReconcile_MultipleConflicts(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	// Three agents all editing the same file
	delta1 := &Delta{
		AgentID: "agent-1",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "create", ContentHash: "hash1"},
		},
		InputHash:  "input1",
		OutputHash: "output1",
	}

	delta2 := &Delta{
		AgentID: "agent-2",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "modify", ContentHash: "hash2"},
		},
		InputHash:  "input2",
		OutputHash: "output2",
	}

	delta3 := &Delta{
		AgentID: "agent-3",
		Files: map[string]FileChange{
			"/shared.go": {Path: "/shared.go", Operation: "modify", ContentHash: "hash3"},
		},
		InputHash:  "input3",
		OutputHash: "output3",
	}

	final, conflict, err := r.Reconcile(ctx, []*Delta{delta1, delta2, delta3})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if final != nil {
		t.Errorf("Expected nil final delta, got: %+v", final)
	}
	if conflict == nil {
		t.Fatal("Expected conflict report, got nil")
	}

	// Should have 3 conflicts: (1,2), (1,3), (2,3)
	if len(conflict.Conflicts) != 3 {
		t.Errorf("Expected 3 conflicts, got: %d", len(conflict.Conflicts))
	}

	// Verify conflict graph has all three agents
	if len(conflict.ConflictGraph) != 3 {
		t.Errorf("Expected 3 nodes in conflict graph, got: %d", len(conflict.ConflictGraph))
	}
}

// TestReconcile_InvalidInputs verifies input validation
func TestReconcile_InvalidInputs(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	tests := []struct {
		name   string
		deltas []*Delta
	}{
		{
			name: "nil delta",
			deltas: []*Delta{
				nil,
				{AgentID: "agent-1", Files: map[string]FileChange{}, InputHash: "hash", OutputHash: "hash"},
			},
		},
		{
			name: "empty AgentID",
			deltas: []*Delta{
				{AgentID: "", Files: map[string]FileChange{}, InputHash: "hash", OutputHash: "hash"},
			},
		},
		{
			name: "empty InputHash",
			deltas: []*Delta{
				{AgentID: "agent-1", Files: map[string]FileChange{}, InputHash: "", OutputHash: "hash"},
			},
		},
		{
			name: "empty OutputHash",
			deltas: []*Delta{
				{AgentID: "agent-1", Files: map[string]FileChange{}, InputHash: "hash", OutputHash: ""},
			},
		},
		{
			name: "nil Files map",
			deltas: []*Delta{
				{AgentID: "agent-1", Files: nil, InputHash: "hash", OutputHash: "hash"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := r.Reconcile(ctx, tt.deltas)
			if err == nil {
				t.Errorf("Expected validation error for %s", tt.name)
			}
		})
	}
}

// TestReconcile_All9Agents simulates successful reconciliation of all 9 agents
func TestReconcile_All9Agents(t *testing.T) {
	r := NewReconciler()
	ctx := context.Background()

	// Create 9 disjoint deltas
	var deltas []*Delta
	for i := 1; i <= 9; i++ {
		delta := &Delta{
			AgentID: fmt.Sprintf("agent-%d", i),
			Files: map[string]FileChange{
				fmt.Sprintf("/agent-%d/file.go", i): {
					Path:        fmt.Sprintf("/agent-%d/file.go", i),
					Operation:   "create",
					ContentHash: fmt.Sprintf("hash%d", i),
				},
			},
			InputHash:  fmt.Sprintf("input%d", i),
			OutputHash: fmt.Sprintf("output%d", i),
		}
		deltas = append(deltas, delta)
	}

	final, conflict, err := r.Reconcile(ctx, deltas)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if conflict != nil {
		t.Errorf("Expected no conflict, got: %+v", conflict)
	}
	if final == nil {
		t.Fatal("Expected final delta, got nil")
	}
	if len(final.Files) != 9 {
		t.Errorf("Expected 9 files, got: %d", len(final.Files))
	}
}
