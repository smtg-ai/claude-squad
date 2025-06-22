package session

import (
	"testing"
)

func TestCleanSessionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple-name", "simple-name"},
		{"  spaced  name  ", "spaced-name"},
		{"Name with CAPS", "Name-with-CAPS"},
		{"special@#$chars", "specialchars"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"--leading-trailing--", "leading-trailing"},
		{"ticket-123-fix", "ticket-123-fix"},
		{"\"quoted name\"", "quoted-name"},
		{"", ""},
	}

	for _, test := range tests {
		result := cleanSessionName(test.input)
		if result != test.expected {
			t.Errorf("cleanSessionName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		prompt   string
		contains []string
	}{
		{
			"Fix authentication issue",
			[]string{"session name", "under 32 characters", "Fix authentication issue"},
		},
		{
			"Implement ticket ABC-123 user login",
			[]string{"ABC-123", "ticket number", "ABC-123-"},
		},
		{
			"Task PROJ-456 add validation",
			[]string{"PROJ-456", "ticket number", "PROJ-456-"},
		},
		{
			"Simple refactor without ticket",
			[]string{"keyword", "refactor-api"},
		},
	}

	for _, test := range tests {
		result := buildSystemPrompt(test.prompt)
		for _, contains := range test.contains {
			if !stringContains(result, contains) {
				t.Errorf("buildSystemPrompt(%q) should contain %q, but result was: %q", test.prompt, contains, result)
			}
		}
	}
}

// Helper function since strings.Contains might not be available in all environments
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewNameGeneratorConfig(t *testing.T) {
	config := NewNameGeneratorConfig()
	
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}
	
	if config.MaxLength != 32 {
		t.Errorf("Expected MaxLength to be 32, got %d", config.MaxLength)
	}
}