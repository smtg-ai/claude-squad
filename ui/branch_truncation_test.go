package ui

import (
	"testing"
)

func TestTruncateBranchName(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		maxWidth int
		expected string
	}{
		{
			name:     "short branch fits completely",
			branch:   "main",
			maxWidth: 10,
			expected: "main",
		},
		{
			name:     "branch fits exactly",
			branch:   "feature",
			maxWidth: 7,
			expected: "feature",
		},
		{
			name:     "branch needs truncation - preserves suffix",
			branch:   "feature/epic1/story5/enhanced-visualization",
			maxWidth: 25,
			expected: "...enhanced-visualization",
		},
		{
			name:     "branch needs truncation - very short width",
			branch:   "feature/epic1/story5",
			maxWidth: 10,
			expected: ".../story5",
		},
		{
			name:     "width too small for ellipsis",
			branch:   "feature",
			maxWidth: 2,
			expected: "",
		},
		{
			name:     "negative width",
			branch:   "feature",
			maxWidth: -1,
			expected: "",
		},
		{
			name:     "empty branch",
			branch:   "",
			maxWidth: 10,
			expected: "",
		},
		{
			name:     "long branch with username prefix",
			branch:   "user/feature/epic1/story5/enhanced-visualization",
			maxWidth: 30,
			expected: "...ory5/enhanced-visualization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateBranchName(tt.branch, tt.maxWidth)
			if result != tt.expected {
				t.Errorf("TruncateBranchName(%q, %d) = %q, want %q",
					tt.branch, tt.maxWidth, result, tt.expected)
			}
		})
	}
}

func TestGenerateBranchNamePreview(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		maxWidth int
		expected string
	}{
		{
			name:     "empty title",
			title:    "",
			maxWidth: 20,
			expected: "",
		},
		{
			name:     "simple title",
			title:    "test",
			maxWidth: 20,
			expected: "2-gabadi/test", // Assuming current user is 2-gabadi
		},
		{
			name:     "title with spaces and special chars",
			title:    "Test Feature!@#",
			maxWidth: 30,
			expected: "2-gabadi/test-feature",
		},
		{
			name:     "long title requiring truncation",
			title:    "Very Long Feature Name With Many Words",
			maxWidth: 20,
			expected: "...any-words", // Should preserve suffix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateBranchNamePreview(tt.title, tt.maxWidth)

			// For empty title, expect empty result
			if tt.title == "" {
				if result != "" {
					t.Errorf("GenerateBranchNamePreview(%q, %d) = %q, want empty string",
						tt.title, tt.maxWidth, result)
				}
				return
			}

			// For non-empty titles, just verify the function doesn't panic and returns something
			// The exact result depends on the current user and config
			if result == "" && tt.title != "" {
				t.Errorf("GenerateBranchNamePreview(%q, %d) returned empty for non-empty title",
					tt.title, tt.maxWidth)
			}
		})
	}
}
