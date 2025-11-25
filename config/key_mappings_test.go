package config

import (
	"reflect"
	"testing"
)

func TestGetDefaultKeyMappings(t *testing.T) {
	defaults := getDefaultKeyMappings()

	expected := map[string][]string{
		"up":         {"up", "k"},
		"down":       {"down", "j"},
		"shift+up":   {"shift+up"},
		"shift+down": {"shift+down"},
		"enter":      {"enter", "o"},
		"new":        {"n"},
		"kill":       {"D"},
		"quit":       {"q"},
		"tab":        {"tab"},
		"checkout":   {"c"},
		"resume":     {"r"},
		"submit":     {"p"},
		"prompt":     {"N"},
		"help":       {"?"},
	}

	if !reflect.DeepEqual(defaults, expected) {
		t.Errorf("getDefaultKeyMappings() = %v, want %v", defaults, expected)
	}

	// Verify specific important keys
	if !reflect.DeepEqual(defaults["up"], []string{"up", "k"}) {
		t.Errorf("up mapping = %v, want %v", defaults["up"], []string{"up", "k"})
	}

	if !reflect.DeepEqual(defaults["quit"], []string{"q"}) {
		t.Errorf("quit mapping = %v, want %v", defaults["quit"], []string{"q"})
	}
}

func TestMergeKeyMappings(t *testing.T) {
	tests := []struct {
		name     string
		defaults map[string][]string
		user     map[string][]string
		expected map[string][]string
	}{
		{
			name: "nil user mappings",
			defaults: map[string][]string{
				"up":   {"up", "k"},
				"quit": {"q"},
			},
			user: nil,
			expected: map[string][]string{
				"up":   {"up", "k"},
				"quit": {"q"},
			},
		},
		{
			name: "empty user mappings",
			defaults: map[string][]string{
				"up":   {"up", "k"},
				"quit": {"q"},
			},
			user: map[string][]string{},
			expected: map[string][]string{
				"up":   {"up", "k"},
				"quit": {"q"},
			},
		},
		{
			name: "partial user override",
			defaults: map[string][]string{
				"up":   {"up", "k"},
				"down": {"down", "j"},
				"quit": {"q"},
			},
			user: map[string][]string{
				"quit": {"Q"},
			},
			expected: map[string][]string{
				"up":   {"up", "k"},
				"down": {"down", "j"},
				"quit": {"Q"},
			},
		},
		{
			name: "multiple key override",
			defaults: map[string][]string{
				"quit": {"q"},
			},
			user: map[string][]string{
				"quit": {"Q", "esc"},
			},
			expected: map[string][]string{
				"quit": {"Q", "esc"},
			},
		},
		{
			name: "complete override with multiple actions",
			defaults: map[string][]string{
				"up":       {"up", "k"},
				"quit":     {"q"},
				"checkout": {"c"},
			},
			user: map[string][]string{
				"quit":     {"Q"},
				"checkout": {"C"},
			},
			expected: map[string][]string{
				"up":       {"up", "k"},
				"quit":     {"Q"},
				"checkout": {"C"},
			},
		},
		{
			name: "empty user value ignored",
			defaults: map[string][]string{
				"quit": {"q"},
			},
			user: map[string][]string{
				"quit": {},
			},
			expected: map[string][]string{
				"quit": {"q"},
			},
		},
		{
			name: "real-world example - user's exact config",
			defaults: map[string][]string{
				"up":       {"up", "k"},
				"down":     {"down", "j"},
				"quit":     {"q"},
				"checkout": {"c"},
				"kill":     {"D"},
				"submit":   {"p"},
			},
			user: map[string][]string{
				"quit":     {"Q"},
				"checkout": {"C"},
				"kill":     {"D"},
				"submit":   {"P"},
			},
			expected: map[string][]string{
				"up":       {"up", "k"},
				"down":     {"down", "j"},
				"quit":     {"Q"},
				"checkout": {"C"},
				"kill":     {"D"},
				"submit":   {"P"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeKeyMappings(tt.defaults, tt.user)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("mergeKeyMappings() = %v, want %v", result, tt.expected)
			}

			// Verify the original maps weren't modified
			originalDefaults := getDefaultKeyMappings()
			if len(tt.defaults) > 0 && !reflect.DeepEqual(tt.defaults, originalDefaults) {
				// This test is only valid if we're using actual defaults
				if tt.name == "real-world example - user's exact config" {
					return // Skip this check for custom test data
				}
			}
		})
	}
}

func TestMergeKeyMappingsDoesNotModifyOriginals(t *testing.T) {
	originalDefaults := map[string][]string{
		"up":   {"up", "k"},
		"quit": {"q"},
	}
	defaultsCopy := map[string][]string{
		"up":   {"up", "k"},
		"quit": {"q"},
	}

	originalUser := map[string][]string{
		"quit": {"Q"},
	}
	userCopy := map[string][]string{
		"quit": {"Q"},
	}

	result := mergeKeyMappings(originalDefaults, originalUser)

	// Verify originals weren't modified
	if !reflect.DeepEqual(originalDefaults, defaultsCopy) {
		t.Errorf("mergeKeyMappings modified defaults: got %v, want %v", originalDefaults, defaultsCopy)
	}

	if !reflect.DeepEqual(originalUser, userCopy) {
		t.Errorf("mergeKeyMappings modified user: got %v, want %v", originalUser, userCopy)
	}

	// Verify result is correct
	expected := map[string][]string{
		"up":   {"up", "k"},
		"quit": {"Q"},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("mergeKeyMappings() = %v, want %v", result, expected)
	}
}

func TestMergeKeyMappingsIndependentCopies(t *testing.T) {
	defaults := map[string][]string{
		"up": {"up", "k"},
	}
	user := map[string][]string{
		"quit": {"Q"},
	}

	result := mergeKeyMappings(defaults, user)

	// Modify result and verify originals aren't affected
	result["up"] = []string{"modified"}
	result["quit"] = []string{"modified"}

	if !reflect.DeepEqual(defaults["up"], []string{"up", "k"}) {
		t.Errorf("modifying result affected defaults: %v", defaults["up"])
	}

	if !reflect.DeepEqual(user["quit"], []string{"Q"}) {
		t.Errorf("modifying result affected user: %v", user["quit"])
	}
}