// Package agent3 implements the Policy Pack Bridge for KGC substrate.
// This provides a thin adapter layer to load and validate policy packs
// from external sources (e.g., seanchatmangpt/unrdf) with loose coupling.
package agent3

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// PolicyPackBridge provides the interface for loading and validating policy packs.
// Design: Loose coupling via interface - external policy pack sources can be
// plugged in without deep integration.
type PolicyPackBridge interface {
	// LoadPolicyPack loads a policy pack by name
	// O: packName (string identifier)
	// A = μ(O): Load policy pack definition from configured source
	LoadPolicyPack(packName string) (*PolicyPack, error)

	// ValidateAgainstPolicies validates a patch against loaded policies
	// O: (ctx, patch)
	// A = μ(O): Check if patch conforms to policy constraints
	ValidateAgainstPolicies(ctx context.Context, patch *Delta) error

	// ApplyPolicies applies policies to an agent run
	// O: (ctx, agentRun)
	// A = μ(O): Transform agent run according to policy rules
	ApplyPolicies(ctx context.Context, agent AgentRun) (AgentRun, error)
}

// PolicyPack represents a collection of policies that can be validated against.
type PolicyPack struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Policies    []Policy          `json:"policies"`
	Metadata    map[string]string `json:"metadata"`
}

// Policy represents a single validation rule.
type Policy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // "file_pattern", "content_rule", "metadata_check"
	Rules       []Rule   `json:"rules"`
	Severity    string   `json:"severity"` // "error", "warning", "info"
	Tags        []string `json:"tags"`
}

// Rule represents a specific constraint within a policy.
type Rule struct {
	Constraint string      `json:"constraint"` // e.g., "max_file_size", "allowed_extensions"
	Value      interface{} `json:"value"`
	Message    string      `json:"message"`
}

// Delta represents a patch/change-set that needs validation.
type Delta struct {
	ID            string            `json:"id"`
	Files         []string          `json:"files"`          // Files modified
	BeforeHash    string            `json:"before_hash"`    // State before
	AfterHash     string            `json:"after_hash"`     // State after
	Timestamp     int64             `json:"timestamp"`      // Unix nanoseconds
	Metadata      map[string]string `json:"metadata"`       // Additional context
	ReplayScript  string            `json:"replay_script"`  // How to reproduce
	CompositionOp string            `json:"composition_op"` // "append", "merge", etc.
}

// AgentRun represents an agent execution context.
type AgentRun interface {
	GetID() string
	GetMetadata() map[string]string
	Execute(ctx context.Context, inputs *AgentInput) (*AgentOutput, error)
}

// AgentInput represents input to an agent execution.
type AgentInput struct {
	TaskID   string            `json:"task_id"`
	Params   map[string]string `json:"params"`
	Context  map[string]string `json:"context"`
	Priority int               `json:"priority"`
}

// AgentOutput represents output from an agent execution.
type AgentOutput struct {
	TaskID    string            `json:"task_id"`
	Results   map[string]string `json:"results"`
	Artifacts []string          `json:"artifacts"`
	Success   bool              `json:"success"`
}

// DefaultPolicyBridge is the default implementation of PolicyPackBridge.
// This implementation uses a stub policy loader that can be replaced
// with actual unrdf integration when available.
type DefaultPolicyBridge struct {
	mu           sync.RWMutex
	policyPacks  map[string]*PolicyPack
	policyLoader PolicyLoader
}

// PolicyLoader is the interface for loading policies from external sources.
// This allows loose coupling to unrdf or other policy sources.
type PolicyLoader interface {
	Load(packName string) (*PolicyPack, error)
}

// StubPolicyLoader is a minimal stub implementation for testing.
// Replace with actual unrdf integration when available.
type StubPolicyLoader struct {
	mu    sync.RWMutex
	packs map[string]*PolicyPack
}

// NewStubPolicyLoader creates a new stub policy loader with some default policies.
func NewStubPolicyLoader() *StubPolicyLoader {
	loader := &StubPolicyLoader{
		packs: make(map[string]*PolicyPack),
	}

	// Add a default "core" policy pack for testing
	loader.packs["core"] = &PolicyPack{
		Name:        "core",
		Version:     "v0.1.0",
		Description: "Core KGC validation policies",
		Policies: []Policy{
			{
				ID:          "no-edit-outside-tranche",
				Name:        "No edits outside agent tranche",
				Description: "Agents must only edit files in their assigned tranche directory",
				Type:        "file_pattern",
				Rules: []Rule{
					{
						Constraint: "file_path_prefix",
						Value:      "integrations/kgc/agent-",
						Message:    "Files must be within agent tranche directory",
					},
				},
				Severity: "error",
				Tags:     []string{"isolation", "tranche"},
			},
			{
				ID:          "require-receipt",
				Name:        "Require RECEIPT.json",
				Description: "All changes must include a RECEIPT.json",
				Type:        "metadata_check",
				Rules: []Rule{
					{
						Constraint: "required_file",
						Value:      "RECEIPT.json",
						Message:    "RECEIPT.json must be present in patch",
					},
				},
				Severity: "error",
				Tags:     []string{"receipt", "proof"},
			},
		},
		Metadata: map[string]string{
			"source": "stub",
			"env":    "test",
		},
	}

	return loader
}

// Load implements PolicyLoader.
func (s *StubPolicyLoader) Load(packName string) (*PolicyPack, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pack, ok := s.packs[packName]
	if !ok {
		return nil, fmt.Errorf("policy pack not found: %s", packName)
	}

	// Return a copy to prevent external mutation
	packCopy := *pack
	packCopy.Policies = make([]Policy, len(pack.Policies))
	copy(packCopy.Policies, pack.Policies)

	return &packCopy, nil
}

// AddPolicyPack adds a policy pack to the stub loader (for testing).
func (s *StubPolicyLoader) AddPolicyPack(pack *PolicyPack) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packs[pack.Name] = pack
}

// NewDefaultPolicyBridge creates a new DefaultPolicyBridge with a policy loader.
func NewDefaultPolicyBridge(loader PolicyLoader) *DefaultPolicyBridge {
	if loader == nil {
		loader = NewStubPolicyLoader()
	}

	return &DefaultPolicyBridge{
		policyPacks:  make(map[string]*PolicyPack),
		policyLoader: loader,
	}
}

// LoadPolicyPack implements PolicyPackBridge.
// Invariant: Idempotent - loading the same pack twice returns identical result.
func (b *DefaultPolicyBridge) LoadPolicyPack(packName string) (*PolicyPack, error) {
	// Check cache first
	b.mu.RLock()
	cached, ok := b.policyPacks[packName]
	b.mu.RUnlock()

	if ok {
		return cached, nil
	}

	// Load from policy loader
	pack, err := b.policyLoader.Load(packName)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy pack %s: %w", packName, err)
	}

	// Cache the loaded pack
	b.mu.Lock()
	b.policyPacks[packName] = pack
	b.mu.Unlock()

	return pack, nil
}

// ValidateAgainstPolicies implements PolicyPackBridge.
// Validates a Delta against all loaded policies.
// Returns nil if valid, error describing violation if invalid.
func (b *DefaultPolicyBridge) ValidateAgainstPolicies(ctx context.Context, patch *Delta) error {
	if patch == nil {
		return fmt.Errorf("patch cannot be nil")
	}

	// Check context timeout
	select {
	case <-ctx.Done():
		return fmt.Errorf("validation cancelled: %w", ctx.Err())
	default:
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Validate against all loaded policy packs
	for packName, pack := range b.policyPacks {
		for _, policy := range pack.Policies {
			if err := b.validatePolicy(ctx, patch, &policy); err != nil {
				return fmt.Errorf("policy pack %s: %w", packName, err)
			}
		}
	}

	return nil
}

// validatePolicy validates a single policy against a patch.
func (b *DefaultPolicyBridge) validatePolicy(ctx context.Context, patch *Delta, policy *Policy) error {
	// Check context timeout
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	switch policy.Type {
	case "file_pattern":
		return b.validateFilePattern(patch, policy)
	case "content_rule":
		return b.validateContentRule(patch, policy)
	case "metadata_check":
		return b.validateMetadata(patch, policy)
	default:
		// Unknown policy type - log but don't fail
		return nil
	}
}

// validateFilePattern validates file-related policies.
func (b *DefaultPolicyBridge) validateFilePattern(patch *Delta, policy *Policy) error {
	for _, rule := range policy.Rules {
		if rule.Constraint == "file_path_prefix" {
			prefix, ok := rule.Value.(string)
			if !ok {
				continue
			}

			// Check if all files start with the required prefix
			for _, file := range patch.Files {
				// This is a simplified check - in production would use filepath.HasPrefix
				if len(file) < len(prefix) || file[:len(prefix)] != prefix {
					if policy.Severity == "error" {
						return fmt.Errorf("policy violation [%s]: %s (file: %s)", policy.ID, rule.Message, file)
					}
				}
			}
		}
	}
	return nil
}

// validateContentRule validates content-related policies.
func (b *DefaultPolicyBridge) validateContentRule(patch *Delta, policy *Policy) error {
	// Stub implementation - would require reading file contents in production
	return nil
}

// validateMetadata validates metadata-related policies.
func (b *DefaultPolicyBridge) validateMetadata(patch *Delta, policy *Policy) error {
	for _, rule := range policy.Rules {
		if rule.Constraint == "required_file" {
			requiredFile, ok := rule.Value.(string)
			if !ok {
				continue
			}

			// Check if required file is in the patch
			found := false
			for _, file := range patch.Files {
				// Simple suffix check
				if len(file) >= len(requiredFile) && file[len(file)-len(requiredFile):] == requiredFile {
					found = true
					break
				}
			}

			if !found && policy.Severity == "error" {
				return fmt.Errorf("policy violation [%s]: %s", policy.ID, rule.Message)
			}
		}
	}
	return nil
}

// ApplyPolicies implements PolicyPackBridge.
// Applies policy transformations to an agent run.
// Invariant: Deterministic - same input always produces same output.
func (b *DefaultPolicyBridge) ApplyPolicies(ctx context.Context, agent AgentRun) (AgentRun, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent cannot be nil")
	}

	// Check context timeout
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("policy application cancelled: %w", ctx.Err())
	default:
	}

	// Currently policies are validation-only, so return agent unchanged
	// In future, policies could transform agent behavior
	return agent, nil
}

// ValidationResult captures the outcome of policy validation.
type ValidationResult struct {
	Valid      bool              `json:"valid"`
	Violations []string          `json:"violations,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
	Timestamp  int64             `json:"timestamp"`
	PolicyHash string            `json:"policy_hash"` // Hash of applied policies for audit
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ValidateWithResult performs validation and returns detailed result.
func (b *DefaultPolicyBridge) ValidateWithResult(ctx context.Context, patch *Delta) *ValidationResult {
	result := &ValidationResult{
		Valid:     true,
		Timestamp: time.Now().UnixNano(),
		Metadata:  make(map[string]string),
	}

	// Calculate policy hash for audit trail
	b.mu.RLock()
	policyData, _ := json.Marshal(b.policyPacks)
	b.mu.RUnlock()

	hash := sha256.Sum256(policyData)
	result.PolicyHash = fmt.Sprintf("%x", hash)

	// Perform validation
	err := b.ValidateAgainstPolicies(ctx, patch)
	if err != nil {
		result.Valid = false
		result.Violations = append(result.Violations, err.Error())
	}

	return result
}
