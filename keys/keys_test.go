package keys

import (
	"testing"
)

func TestUpdateKeyMappings(t *testing.T) {
	// Save original state
	originalKeyStringsMap := make(map[string]KeyName)
	for k, v := range GlobalKeyStringsMap {
		originalKeyStringsMap[k] = v
	}
	originalKeyBindings := make(map[KeyName]interface{})
	for k, v := range GlobalkeyBindings {
		originalKeyBindings[k] = v.Help().Key
	}

	defer func() {
		// Restore original state after tests
		GlobalKeyStringsMap = originalKeyStringsMap
		// Note: GlobalkeyBindings restoration would be complex, so we'll rely on test isolation
	}()

	tests := []struct {
		name            string
		userMappings    map[string][]string
		expectedStrings map[string]KeyName
		expectedBindings map[KeyName]string
	}{
		{
			name:         "nil user mappings",
			userMappings: nil,
			expectedStrings: map[string]KeyName{
				"up": KeyUp,
				"k":  KeyUp,
				"q":  KeyQuit,
			},
			expectedBindings: map[KeyName]string{
				KeyUp:   "↑/k",
				KeyQuit: "q",
			},
		},
		{
			name: "single key override - quit",
			userMappings: map[string][]string{
				"quit": {"Q"},
			},
			expectedStrings: map[string]KeyName{
				"up": KeyUp,  // unchanged
				"k":  KeyUp,  // unchanged
				"Q":  KeyQuit, // new
				// "q" should NOT exist (removed)
			},
			expectedBindings: map[KeyName]string{
				KeyUp:   "↑/k", // unchanged
				KeyQuit: "Q",   // updated
			},
		},
		{
			name: "multiple key override",
			userMappings: map[string][]string{
				"quit": {"Q", "esc"},
			},
			expectedStrings: map[string]KeyName{
				"Q":   KeyQuit,
				"esc": KeyQuit,
				// "q" should NOT exist
			},
			expectedBindings: map[KeyName]string{
				KeyQuit: "Q/esc",
			},
		},
		{
			name: "multiple actions override",
			userMappings: map[string][]string{
				"quit":     {"Q"},
				"checkout": {"C"},
			},
			expectedStrings: map[string]KeyName{
				"Q": KeyQuit,
				"C": KeyCheckout,
				// "q" and "c" should NOT exist
			},
			expectedBindings: map[KeyName]string{
				KeyQuit:     "Q",
				KeyCheckout: "C",
			},
		},
		{
			name: "real user config test",
			userMappings: map[string][]string{
				"quit":     {"Q"},
				"checkout": {"C"},
				"kill":     {"D"},
				"submit":   {"P"},
			},
			expectedStrings: map[string]KeyName{
				"up": KeyUp, // unchanged default
				"k":  KeyUp, // unchanged default
				"Q":  KeyQuit,
				"C":  KeyCheckout,
				"D":  KeyKill,
				"P":  KeySubmit,
				// "q", "c", "p" should NOT exist (overridden)
			},
			expectedBindings: map[KeyName]string{
				KeyUp:       "↑/k", // unchanged
				KeyQuit:     "Q",
				KeyCheckout: "C",
				KeyKill:     "D",
				KeySubmit:   "P",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset to known state before each test
			GlobalKeyStringsMap = map[string]KeyName{
				"up":         KeyUp,
				"k":          KeyUp,
				"down":       KeyDown,
				"j":          KeyDown,
				"shift+up":   KeyShiftUp,
				"shift+down": KeyShiftDown,
				"N":          KeyPrompt,
				"enter":      KeyEnter,
				"o":          KeyEnter,
				"n":          KeyNew,
				"D":          KeyKill,
				"q":          KeyQuit,
				"tab":        KeyTab,
				"c":          KeyCheckout,
				"r":          KeyResume,
				"p":          KeySubmit,
				"?":          KeyHelp,
			}

			UpdateKeyMappings(tt.userMappings)

			// Check expected keys exist
			for expectedKey, expectedKeyName := range tt.expectedStrings {
				if actualKeyName, exists := GlobalKeyStringsMap[expectedKey]; !exists {
					t.Errorf("Expected key %q not found in GlobalKeyStringsMap", expectedKey)
				} else if actualKeyName != expectedKeyName {
					t.Errorf("Key %q maps to %v, want %v", expectedKey, actualKeyName, expectedKeyName)
				}
			}

			// Check that overridden default keys are removed
			if tt.userMappings != nil {
				for action, _ := range tt.userMappings {
					var defaultKeys []string
					switch action {
					case "quit":
						defaultKeys = []string{"q"}
					case "checkout":
						defaultKeys = []string{"c"}
					case "submit":
						defaultKeys = []string{"p"}
					case "kill":
						defaultKeys = []string{"D"} // D is default for kill
					}

					for _, defaultKey := range defaultKeys {
						// Only check if this default key was actually replaced
						userKeys := tt.userMappings[action]
						replaced := true
						for _, userKey := range userKeys {
							if userKey == defaultKey {
								replaced = false // User kept the default key
								break
							}
						}

						if replaced {
							if _, exists := GlobalKeyStringsMap[defaultKey]; exists {
								t.Errorf("Default key %q should have been removed when %s was overridden", defaultKey, action)
							}
						}
					}
				}
			}

			// Check key bindings
			for keyName, expectedHelpKey := range tt.expectedBindings {
				actualHelpKey := GlobalkeyBindings[keyName].Help().Key
				if actualHelpKey != expectedHelpKey {
					t.Errorf("KeyBinding for %v has help key %q, want %q", keyName, actualHelpKey, expectedHelpKey)
				}
			}
		})
	}
}

func TestGetHelpKey(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{
			name:     "empty keys",
			keys:     []string{},
			expected: "",
		},
		{
			name:     "single key",
			keys:     []string{"Q"},
			expected: "Q",
		},
		{
			name:     "two keys",
			keys:     []string{"Q", "esc"},
			expected: "Q/esc",
		},
		{
			name:     "multiple keys",
			keys:     []string{"up", "k", "w"},
			expected: "up/k/w",
		},
		{
			name:     "special characters",
			keys:     []string{"ctrl+c", "shift+q"},
			expected: "ctrl+c/shift+q",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHelpKey(tt.keys)
			if result != tt.expected {
				t.Errorf("getHelpKey(%v) = %q, want %q", tt.keys, result, tt.expected)
			}
		})
	}
}

func TestUpdateKeyMappingsPreservesUnchangedDefaults(t *testing.T) {
	// Test that keys not mentioned in user config remain as defaults
	userMappings := map[string][]string{
		"quit": {"Q"},
	}

	// Reset to known state
	GlobalKeyStringsMap = map[string]KeyName{
		"up": KeyUp,
		"k":  KeyUp,
		"q":  KeyQuit,
		"c":  KeyCheckout,
	}

	UpdateKeyMappings(userMappings)

	// Check that non-overridden keys remain
	if GlobalKeyStringsMap["up"] != KeyUp {
		t.Errorf("Expected 'up' to remain KeyUp, got %v", GlobalKeyStringsMap["up"])
	}
	if GlobalKeyStringsMap["k"] != KeyUp {
		t.Errorf("Expected 'k' to remain KeyUp, got %v", GlobalKeyStringsMap["k"])
	}
	if GlobalKeyStringsMap["c"] != KeyCheckout {
		t.Errorf("Expected 'c' to remain KeyCheckout, got %v", GlobalKeyStringsMap["c"])
	}

	// Check that overridden key is updated
	if GlobalKeyStringsMap["Q"] != KeyQuit {
		t.Errorf("Expected 'Q' to be KeyQuit, got %v", GlobalKeyStringsMap["Q"])
	}

	// Check that original key for overridden action is removed
	if _, exists := GlobalKeyStringsMap["q"]; exists {
		t.Errorf("Expected 'q' to be removed when quit was overridden")
	}
}

func TestUpdateKeyMappingsIgnoresUnknownActions(t *testing.T) {
	userMappings := map[string][]string{
		"quit":           {"Q"},
		"unknown_action": {"X"},
		"invalid":        {"Y"},
	}

	UpdateKeyMappings(userMappings)

	// Unknown actions should be ignored
	if _, exists := GlobalKeyStringsMap["X"]; exists {
		t.Errorf("Unknown action 'unknown_action' should have been ignored")
	}
	if _, exists := GlobalKeyStringsMap["Y"]; exists {
		t.Errorf("Unknown action 'invalid' should have been ignored")
	}

	// Valid action should be processed
	if GlobalKeyStringsMap["Q"] != KeyQuit {
		t.Errorf("Valid action 'quit' should have been processed")
	}
}

// Integration test that verifies the bug fix
func TestBugFix_CustomKeysMustReplaceDefaults(t *testing.T) {
	// This test verifies the specific bug reported by the user:
	// User sets "quit": ["Q"] but both Q and q work (should only be Q)

	userMappings := map[string][]string{
		"quit": {"Q"},
	}

	// Reset to clean state
	GlobalKeyStringsMap = map[string]KeyName{
		"q": KeyQuit,
		"c": KeyCheckout,
	}

	UpdateKeyMappings(userMappings)

	// After update, ONLY Q should trigger quit, not q
	if keyName, exists := GlobalKeyStringsMap["Q"]; !exists || keyName != KeyQuit {
		t.Errorf("Custom key 'Q' should trigger quit, got exists=%v, keyName=%v", exists, keyName)
	}

	if _, exists := GlobalKeyStringsMap["q"]; exists {
		t.Errorf("Default key 'q' should be removed when user configures custom quit key")
	}

	// Verify help text shows custom key
	quitBinding := GlobalkeyBindings[KeyQuit]
	if quitBinding.Help().Key != "Q" {
		t.Errorf("Help text should show 'Q', got %q", quitBinding.Help().Key)
	}
}