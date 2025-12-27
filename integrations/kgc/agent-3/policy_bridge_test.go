package agent3

import (
	"context"
	"testing"
	"time"
)

// TestPolicyPackBridge_InterfaceCompilation verifies the interface compiles.
func TestPolicyPackBridge_InterfaceCompilation(t *testing.T) {
	var _ PolicyPackBridge = (*DefaultPolicyBridge)(nil)
}

// TestPolicyLoader_InterfaceCompilation verifies the PolicyLoader interface compiles.
func TestPolicyLoader_InterfaceCompilation(t *testing.T) {
	var _ PolicyLoader = (*StubPolicyLoader)(nil)
}

// TestNewStubPolicyLoader verifies stub loader initialization.
func TestNewStubPolicyLoader(t *testing.T) {
	loader := NewStubPolicyLoader()
	if loader == nil {
		t.Fatal("NewStubPolicyLoader returned nil")
	}

	// Verify default "core" pack is loaded
	pack, err := loader.Load("core")
	if err != nil {
		t.Fatalf("Failed to load core policy pack: %v", err)
	}

	if pack.Name != "core" {
		t.Errorf("Expected pack name 'core', got '%s'", pack.Name)
	}

	if len(pack.Policies) == 0 {
		t.Error("Core pack should have at least one policy")
	}
}

// TestStubPolicyLoader_Load tests policy pack loading.
func TestStubPolicyLoader_Load(t *testing.T) {
	loader := NewStubPolicyLoader()

	// Test loading existing pack
	pack, err := loader.Load("core")
	if err != nil {
		t.Fatalf("Failed to load core pack: %v", err)
	}

	if pack.Name != "core" {
		t.Errorf("Expected name 'core', got '%s'", pack.Name)
	}

	// Test loading non-existent pack
	_, err = loader.Load("nonexistent")
	if err == nil {
		t.Error("Expected error loading non-existent pack, got nil")
	}
}

// TestStubPolicyLoader_AddPolicyPack tests adding custom policy packs.
func TestStubPolicyLoader_AddPolicyPack(t *testing.T) {
	loader := NewStubPolicyLoader()

	customPack := &PolicyPack{
		Name:        "custom",
		Version:     "v1.0.0",
		Description: "Custom test policies",
		Policies: []Policy{
			{
				ID:       "test-policy",
				Name:     "Test Policy",
				Type:     "file_pattern",
				Severity: "warning",
			},
		},
	}

	loader.AddPolicyPack(customPack)

	// Verify it can be loaded
	pack, err := loader.Load("custom")
	if err != nil {
		t.Fatalf("Failed to load custom pack: %v", err)
	}

	if pack.Name != "custom" {
		t.Errorf("Expected name 'custom', got '%s'", pack.Name)
	}
}

// TestNewDefaultPolicyBridge tests bridge initialization.
func TestNewDefaultPolicyBridge(t *testing.T) {
	// Test with nil loader (should create stub)
	bridge := NewDefaultPolicyBridge(nil)
	if bridge == nil {
		t.Fatal("NewDefaultPolicyBridge returned nil")
	}

	// Test with custom loader
	loader := NewStubPolicyLoader()
	bridge = NewDefaultPolicyBridge(loader)
	if bridge == nil {
		t.Fatal("NewDefaultPolicyBridge with loader returned nil")
	}
}

// TestDefaultPolicyBridge_LoadPolicyPack tests policy pack loading.
func TestDefaultPolicyBridge_LoadPolicyPack(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Test loading a pack
	pack, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load pack: %v", err)
	}

	if pack.Name != "core" {
		t.Errorf("Expected pack name 'core', got '%s'", pack.Name)
	}

	// Test idempotence - loading again should return cached version
	pack2, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load pack second time: %v", err)
	}

	if pack != pack2 {
		t.Error("Expected cached pack to be returned (same pointer)")
	}
}

// TestDefaultPolicyBridge_ValidateAgainstPolicies tests patch validation.
func TestDefaultPolicyBridge_ValidateAgainstPolicies(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Load core policies
	_, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load core pack: %v", err)
	}

	tests := []struct {
		name      string
		patch     *Delta
		wantError bool
		errMsg    string
	}{
		{
			name:      "nil patch",
			patch:     nil,
			wantError: true,
			errMsg:    "patch cannot be nil",
		},
		{
			name: "valid patch with RECEIPT.json",
			patch: &Delta{
				ID:    "test-1",
				Files: []string{"integrations/kgc/agent-3/RECEIPT.json"},
			},
			wantError: false,
		},
		{
			name: "missing RECEIPT.json",
			patch: &Delta{
				ID:    "test-2",
				Files: []string{"integrations/kgc/agent-3/test.go"},
			},
			wantError: true,
			errMsg:    "RECEIPT.json must be present",
		},
		{
			name: "file outside tranche",
			patch: &Delta{
				ID:    "test-3",
				Files: []string{"some/other/path/file.go"},
			},
			wantError: true,
			errMsg:    "Files must be within agent tranche directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := bridge.ValidateAgainstPolicies(ctx, tt.patch)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestDefaultPolicyBridge_ValidateWithTimeout tests validation with context timeout.
func TestDefaultPolicyBridge_ValidateWithTimeout(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Load core policies
	_, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load core pack: %v", err)
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	patch := &Delta{
		ID:    "test",
		Files: []string{"integrations/kgc/agent-3/test.go"},
	}

	err = bridge.ValidateAgainstPolicies(ctx, patch)
	if err == nil {
		t.Error("Expected error due to cancelled context, got nil")
	}
}

// TestDefaultPolicyBridge_ValidateWithResult tests detailed validation results.
func TestDefaultPolicyBridge_ValidateWithResult(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Load core policies
	_, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load core pack: %v", err)
	}

	tests := []struct {
		name  string
		patch *Delta
		valid bool
	}{
		{
			name: "valid patch",
			patch: &Delta{
				ID:    "test-1",
				Files: []string{"integrations/kgc/agent-3/RECEIPT.json"},
			},
			valid: true,
		},
		{
			name: "invalid patch - missing RECEIPT",
			patch: &Delta{
				ID:    "test-2",
				Files: []string{"integrations/kgc/agent-3/test.go"},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := bridge.ValidateWithResult(ctx, tt.patch)

			if result == nil {
				t.Fatal("ValidateWithResult returned nil")
			}

			if result.Valid != tt.valid {
				t.Errorf("Expected Valid=%v, got %v", tt.valid, result.Valid)
			}

			if result.PolicyHash == "" {
				t.Error("PolicyHash should not be empty")
			}

			if result.Timestamp == 0 {
				t.Error("Timestamp should not be zero")
			}

			if !tt.valid && len(result.Violations) == 0 {
				t.Error("Expected violations for invalid patch")
			}
		})
	}
}

// TestDefaultPolicyBridge_ApplyPolicies tests policy application.
func TestDefaultPolicyBridge_ApplyPolicies(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Create a mock agent run
	mockAgent := &MockAgentRun{
		id: "test-agent",
		metadata: map[string]string{
			"version": "v1.0.0",
		},
	}

	ctx := context.Background()
	result, err := bridge.ApplyPolicies(ctx, mockAgent)
	if err != nil {
		t.Fatalf("ApplyPolicies failed: %v", err)
	}

	if result == nil {
		t.Fatal("ApplyPolicies returned nil")
	}

	// Currently policies don't transform, so should return same agent
	if result.GetID() != mockAgent.GetID() {
		t.Error("ApplyPolicies should return same agent (no transformation yet)")
	}
}

// TestDefaultPolicyBridge_ApplyPoliciesWithTimeout tests policy application with timeout.
func TestDefaultPolicyBridge_ApplyPoliciesWithTimeout(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	mockAgent := &MockAgentRun{id: "test-agent"}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := bridge.ApplyPolicies(ctx, mockAgent)
	if err == nil {
		t.Error("Expected error due to cancelled context, got nil")
	}
}

// TestDefaultPolicyBridge_ApplyPoliciesNilAgent tests handling of nil agent.
func TestDefaultPolicyBridge_ApplyPoliciesNilAgent(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	ctx := context.Background()
	_, err := bridge.ApplyPolicies(ctx, nil)
	if err == nil {
		t.Error("Expected error for nil agent, got nil")
	}
}

// TestPolicyPackImmutability tests that loaded packs are not mutated.
func TestPolicyPackImmutability(t *testing.T) {
	loader := NewStubPolicyLoader()

	// Load the same pack twice
	pack1, err := loader.Load("core")
	if err != nil {
		t.Fatalf("Failed to load pack: %v", err)
	}

	pack2, err := loader.Load("core")
	if err != nil {
		t.Fatalf("Failed to load pack second time: %v", err)
	}

	// Modify pack1's policies
	pack1.Policies = append(pack1.Policies, Policy{
		ID:   "modified",
		Name: "Modified Policy",
	})

	// Verify pack2 is unaffected
	for _, policy := range pack2.Policies {
		if policy.ID == "modified" {
			t.Error("Pack2 should not be affected by modifications to pack1")
		}
	}
}

// TestDeterministicValidation tests that validation is deterministic.
func TestDeterministicValidation(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Load policies
	_, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load pack: %v", err)
	}

	patch := &Delta{
		ID:    "test",
		Files: []string{"integrations/kgc/agent-3/RECEIPT.json"},
	}

	ctx := context.Background()

	// Run validation multiple times
	results := make([]*ValidationResult, 5)
	for i := 0; i < 5; i++ {
		results[i] = bridge.ValidateWithResult(ctx, patch)
	}

	// All results should have the same validity
	for i := 1; i < len(results); i++ {
		if results[i].Valid != results[0].Valid {
			t.Errorf("Validation result %d differs from result 0", i)
		}
	}

	// All results should have the same policy hash (same policies applied)
	for i := 1; i < len(results); i++ {
		if results[i].PolicyHash != results[0].PolicyHash {
			t.Errorf("PolicyHash %d differs from hash 0", i)
		}
	}
}

// TestConcurrentValidation tests thread safety of validation.
func TestConcurrentValidation(t *testing.T) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Load policies
	_, err := bridge.LoadPolicyPack("core")
	if err != nil {
		t.Fatalf("Failed to load pack: %v", err)
	}

	patch := &Delta{
		ID:    "test",
		Files: []string{"integrations/kgc/agent-3/RECEIPT.json"},
	}

	ctx := context.Background()

	// Run 10 concurrent validations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			err := bridge.ValidateAgainstPolicies(ctx, patch)
			if err != nil {
				t.Errorf("Concurrent validation failed: %v", err)
			}
		}()
	}

	// Wait for all to complete with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// OK
		case <-timeout:
			t.Fatal("Concurrent validation timed out")
		}
	}
}

// MockAgentRun is a mock implementation of AgentRun for testing.
type MockAgentRun struct {
	id       string
	metadata map[string]string
}

func (m *MockAgentRun) GetID() string {
	return m.id
}

func (m *MockAgentRun) GetMetadata() map[string]string {
	return m.metadata
}

func (m *MockAgentRun) Execute(ctx context.Context, inputs *AgentInput) (*AgentOutput, error) {
	return &AgentOutput{
		TaskID:  inputs.TaskID,
		Success: true,
	}, nil
}

// BenchmarkValidation benchmarks the validation performance.
func BenchmarkValidation(b *testing.B) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	// Load policies
	_, err := bridge.LoadPolicyPack("core")
	if err != nil {
		b.Fatalf("Failed to load pack: %v", err)
	}

	patch := &Delta{
		ID:    "bench",
		Files: []string{"integrations/kgc/agent-3/RECEIPT.json"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bridge.ValidateAgainstPolicies(ctx, patch)
	}
}

// BenchmarkLoadPolicyPack benchmarks policy pack loading.
func BenchmarkLoadPolicyPack(b *testing.B) {
	loader := NewStubPolicyLoader()
	bridge := NewDefaultPolicyBridge(loader)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = bridge.LoadPolicyPack("core")
	}
}
