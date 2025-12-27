package agent0

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Delta represents a patch from a single agent
type Delta struct {
	AgentID        string                 `json:"agent_id"`
	Files          map[string]FileChange  `json:"files"`
	InputHash      string                 `json:"input_hash"`
	OutputHash     string                 `json:"output_hash"`
	CompositionOp  string                 `json:"composition_op"`  // "append" | "merge" | "replace" | "extend"
	ConflictPolicy string                 `json:"conflict_policy"` // "fail_fast" | "merge" | "skip"
	Receipt        *Receipt               `json:"receipt,omitempty"`
}

// FileChange represents a modification to a single file
type FileChange struct {
	Path        string `json:"path"`
	Operation   string `json:"operation"`    // "create" | "modify" | "delete"
	ContentHash string `json:"content_hash"` // SHA256 of new content
}

// ConflictReport details all detected conflicts
type ConflictReport struct {
	Conflicts     []Conflict         `json:"conflicts"`
	ConflictGraph map[string][]string `json:"conflict_graph"` // adjacency list
	Resolution    string              `json:"resolution"`     // "manual_required" | "auto_merge" | "abort"
}

// Conflict represents a single file collision between two agents
type Conflict struct {
	File   string `json:"file"`
	Agent1 string `json:"agent1"`
	Agent2 string `json:"agent2"`
	Reason string `json:"reason"`
}

// Receipt represents proof of execution (from Agent 2 interface)
type Receipt struct {
	ExecutionID    string            `json:"execution_id"`
	AgentID        string            `json:"agent_id"`
	Timestamp      int64             `json:"timestamp"`
	ToolchainVer   string            `json:"toolchain_ver"`
	InputHash      string            `json:"input_hash"`
	OutputHash     string            `json:"output_hash"`
	ProofArtifacts map[string]string `json:"proof_artifacts"`
	ReplayScript   string            `json:"replay_script"`
	CompositionOp  string            `json:"composition_op"`
	ConflictPolicy string            `json:"conflict_policy"`
}

// Reconciler validates composition of all agent patches
type Reconciler interface {
	// Reconcile: [Δ] → Δ_final | ConflictReport
	Reconcile(ctx context.Context, deltas []*Delta) (*Delta, *ConflictReport, error)

	// ValidateComposition: Δ₁ ⊕ Δ₂ → bool
	ValidateComposition(delta1, delta2 *Delta) (compatible bool, reason string)
}

// reconciler is the concrete implementation
type reconciler struct{}

// NewReconciler creates a new Reconciler instance
func NewReconciler() Reconciler {
	return &reconciler{}
}

// Reconcile validates and composes all deltas into a final state or conflict report
func (r *reconciler) Reconcile(ctx context.Context, deltas []*Delta) (*Delta, *ConflictReport, error) {
	// Handle empty input
	if len(deltas) == 0 {
		return &Delta{
			AgentID:        "agent-0-global",
			Files:          make(map[string]FileChange),
			InputHash:      computeEmptyHash(),
			OutputHash:     computeEmptyHash(),
			CompositionOp:  "merge",
			ConflictPolicy: "fail_fast",
		}, nil, nil
	}

	// Phase 1: Validate inputs (Λ₁) - always validate before processing
	if err := r.validateInputs(deltas); err != nil {
		return nil, nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Handle single delta (after validation)
	if len(deltas) == 1 {
		final := *deltas[0]
		final.AgentID = "agent-0-global"
		return &final, nil, nil
	}

	// Phase 2: Detect conflicts (Λ₂)
	conflicts := r.detectConflicts(deltas)
	if len(conflicts) > 0 {
		report := &ConflictReport{
			Conflicts:     conflicts,
			ConflictGraph: r.buildConflictGraph(conflicts),
			Resolution:    "manual_required",
		}
		return nil, report, nil
	}

	// Phase 3: Validate composition laws (Λ₃) - done implicitly via tests

	// Phase 4: Compose final delta (Λ₄)
	final := r.composeFinal(deltas)

	return final, nil, nil
}

// ValidateComposition checks if two deltas are compatible (disjoint file sets)
func (r *reconciler) ValidateComposition(delta1, delta2 *Delta) (compatible bool, reason string) {
	if delta1 == nil || delta2 == nil {
		return false, "nil delta provided"
	}

	// Check for file overlaps
	for file := range delta1.Files {
		if _, exists := delta2.Files[file]; exists {
			return false, fmt.Sprintf("file overlap detected: %s (agents: %s, %s)",
				file, delta1.AgentID, delta2.AgentID)
		}
	}

	return true, ""
}

// validateInputs ensures all deltas are well-formed (Λ₁)
func (r *reconciler) validateInputs(deltas []*Delta) error {
	for i, delta := range deltas {
		if delta == nil {
			return fmt.Errorf("delta %d is nil", i)
		}
		if delta.AgentID == "" {
			return fmt.Errorf("delta %d has empty AgentID", i)
		}
		if delta.InputHash == "" {
			return fmt.Errorf("delta %d (agent %s) has empty InputHash", i, delta.AgentID)
		}
		if delta.OutputHash == "" {
			return fmt.Errorf("delta %d (agent %s) has empty OutputHash", i, delta.AgentID)
		}
		if delta.Files == nil {
			return fmt.Errorf("delta %d (agent %s) has nil Files map", i, delta.AgentID)
		}
	}
	return nil
}

// detectConflicts identifies all file overlaps between deltas (Λ₂)
func (r *reconciler) detectConflicts(deltas []*Delta) []Conflict {
	// Build file ownership map: file → [agentIDs]
	fileOwners := make(map[string][]string)

	for _, delta := range deltas {
		for file := range delta.Files {
			fileOwners[file] = append(fileOwners[file], delta.AgentID)
		}
	}

	// Detect conflicts (files with multiple owners)
	var conflicts []Conflict

	// Sort files for deterministic ordering
	files := make([]string, 0, len(fileOwners))
	for file := range fileOwners {
		files = append(files, file)
	}
	sort.Strings(files)

	for _, file := range files {
		owners := fileOwners[file]
		if len(owners) > 1 {
			// Sort owners for deterministic conflict reporting
			sort.Strings(owners)

			// Create conflicts for all pairs
			for i := 0; i < len(owners); i++ {
				for j := i + 1; j < len(owners); j++ {
					conflicts = append(conflicts, Conflict{
						File:   file,
						Agent1: owners[i],
						Agent2: owners[j],
						Reason: fmt.Sprintf("overlapping edit to %s", file),
					})
				}
			}
		}
	}

	return conflicts
}

// buildConflictGraph creates adjacency list of conflicting agents (Λ₂)
func (r *reconciler) buildConflictGraph(conflicts []Conflict) map[string][]string {
	graph := make(map[string][]string)

	for _, conflict := range conflicts {
		// Add bidirectional edges
		graph[conflict.Agent1] = append(graph[conflict.Agent1], conflict.Agent2)
		graph[conflict.Agent2] = append(graph[conflict.Agent2], conflict.Agent1)
	}

	// Sort adjacency lists for determinism
	for agent := range graph {
		sort.Strings(graph[agent])
		// Remove duplicates
		graph[agent] = uniqueSorted(graph[agent])
	}

	return graph
}

// composeFinal merges all disjoint deltas into final state (Λ₄)
func (r *reconciler) composeFinal(deltas []*Delta) *Delta {
	final := &Delta{
		AgentID:        "agent-0-global",
		Files:          make(map[string]FileChange),
		CompositionOp:  "merge",
		ConflictPolicy: "fail_fast",
	}

	// Merge all files (no conflicts at this point)
	for _, delta := range deltas {
		for file, change := range delta.Files {
			final.Files[file] = change
		}
	}

	// Compute global InputHash (hash of all input hashes)
	inputHashes := make([]string, len(deltas))
	for i, delta := range deltas {
		inputHashes[i] = delta.InputHash
	}
	final.InputHash = computeCombinedHash(inputHashes)

	// Compute global OutputHash (hash of all output hashes)
	outputHashes := make([]string, len(deltas))
	for i, delta := range deltas {
		outputHashes[i] = delta.OutputHash
	}
	final.OutputHash = computeCombinedHash(outputHashes)

	return final
}

// computeCombinedHash creates deterministic hash from list of hashes
func computeCombinedHash(hashes []string) string {
	// Sort for deterministic ordering
	sorted := make([]string, len(hashes))
	copy(sorted, hashes)
	sort.Strings(sorted)

	// Concatenate and hash
	combined := strings.Join(sorted, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// computeEmptyHash returns hash of empty state
func computeEmptyHash() string {
	hash := sha256.Sum256([]byte(""))
	return hex.EncodeToString(hash[:])
}

// uniqueSorted removes duplicates from sorted slice
func uniqueSorted(s []string) []string {
	if len(s) == 0 {
		return s
	}

	result := make([]string, 0, len(s))
	result = append(result, s[0])

	for i := 1; i < len(s); i++ {
		if s[i] != s[i-1] {
			result = append(result, s[i])
		}
	}

	return result
}
