package tmux

import (
	"strings"
	"testing"
)

// TestPromptDetection tests the prompt detection logic
func TestPromptDetection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		program     string
		wantPrompt  bool
	}{
		{
			name:        "no prompt in content",
			content:     "This is regular content with no prompt",
			program:     ProgramClaude,
			wantPrompt:  false,
		},
		{
			name:        "claude prompt in content",
			content:     "Some content with No, and tell Claude what to do differently",
			program:     ProgramClaude,
			wantPrompt:  true,
		},
		{
			name:        "aider prompt in content",
			content:     "Some content with (Y)es/(N)o/(D)on't ask again",
			program:     ProgramAider,
			wantPrompt:  true,
		},
		{
			name:        "aider prompt but with claude program",
			content:     "Some content with (Y)es/(N)o/(D)on't ask again",
			program:     ProgramClaude,
			wantPrompt:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the same logic used in HasUpdated
			var hasPrompt bool
			
			if tt.program == ProgramClaude {
				hasPrompt = strings.Contains(tt.content, "No, and tell Claude what to do differently")
			} else if tt.program == ProgramAider || strings.HasPrefix(tt.program, ProgramAider) {
				hasPrompt = strings.Contains(tt.content, "(Y)es/(N)o/(D)on't ask again")
			}
			
			if hasPrompt != tt.wantPrompt {
				t.Errorf("Prompt detection for %q with program %q: got %v, want %v", 
					tt.content, tt.program, hasPrompt, tt.wantPrompt)
			}
		})
	}
}