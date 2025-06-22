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

func TestGenerateFallbackName(t *testing.T) {
	config := &NameGeneratorConfig{MaxLength: 32}

	tests := []struct {
		prompt   string
		expected string
	}{
		{
			"Fix authentication bug in login API",
			"fix-authentication-bug",
		},
		{
			"Implement ticket ABC-123 user validation",
			"ABC-123-implement-user",
		},
		{
			"Add new feature for dashboard",
			"add-feature-dashboard",
		},
		{
			"Very short prompt",
			"very-short-prompt",
		},
		{
			"Random text with no meaningful keywords here",
			"random-text-meaningful",
		},
	}

	for _, test := range tests {
		result := generateFallbackName(test.prompt, config)
		if len(result) > config.MaxLength {
			t.Errorf("generateFallbackName(%q) returned name too long: %q (length %d > %d)",
				test.prompt, result, len(result), config.MaxLength)
		}
		if len(result) == 0 {
			t.Errorf("generateFallbackName(%q) returned empty name", test.prompt)
		}
		// Check that it contains only valid characters
		if !isValidSessionName(result) {
			t.Errorf("generateFallbackName(%q) returned invalid name: %q", test.prompt, result)
		}
	}
}

func TestGenerateSessionNameFallback(t *testing.T) {
	// Test with empty API keys to trigger fallback
	config := &NameGeneratorConfig{
		AnthropicAPIKey: "",
		OpenAIAPIKey:    "",
		MaxRetries:      3,
		MaxLength:       32,
	}

	name, err := GenerateSessionName("Fix user authentication bug", config)
	if err != nil {
		t.Errorf("GenerateSessionName should not return error with fallback: %v", err)
	}

	if len(name) == 0 {
		t.Error("GenerateSessionName should return non-empty name with fallback")
	}

	if len(name) > config.MaxLength {
		t.Errorf("Generated name too long: %q (length %d > %d)", name, len(name), config.MaxLength)
	}
}

// Helper function to validate session name format
func isValidSessionName(name string) bool {
	// Should only contain alphanumeric characters, hyphens, and underscores
	// Should not start or end with hyphens
	if len(name) == 0 {
		return false
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_') {
			return false
		}
	}
	return true
}
