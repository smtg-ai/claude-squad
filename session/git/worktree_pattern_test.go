package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWorktreePattern(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		pattern  string
		vars     PatternVariables
		expected string
	}{
		{
			name:    "all variables",
			pattern: "{repo_root}/worktree/{issue_number}-{title}",
			vars: PatternVariables{
				RepoRoot:    "/home/user/projects/myrepo",
				RepoName:    "myrepo",
				IssueNumber: "123",
				Title:       "fix-bug",
				Timestamp:   "1234567890",
			},
			expected: "/home/user/projects/myrepo/worktree/123-fix-bug",
		},
		{
			name:    "with timestamp",
			pattern: "{repo_root}/worktree/{title}_{timestamp}",
			vars: PatternVariables{
				RepoRoot:  "/home/user/projects/myrepo",
				RepoName:  "myrepo",
				Title:     "new-feature",
				Timestamp: "1234567890",
			},
			expected: "/home/user/projects/myrepo/worktree/new-feature_1234567890",
		},
		{
			name:    "repo name pattern",
			pattern: "/tmp/worktrees/{repo_name}/{issue_number}",
			vars: PatternVariables{
				RepoRoot:    "/home/user/projects/myrepo",
				RepoName:    "myrepo",
				IssueNumber: "456",
			},
			expected: "/tmp/worktrees/myrepo/456",
		},
		{
			name:    "empty pattern",
			pattern: "",
			vars: PatternVariables{
				RepoRoot: "/home/user/projects/myrepo",
				RepoName: "myrepo",
			},
			expected: "",
		},
		{
			name:     "no variables",
			pattern:  "/fixed/path/worktree",
			vars:     PatternVariables{},
			expected: "/fixed/path/worktree",
		},
		{
			name:    "missing variables",
			pattern: "{repo_root}/worktree/{issue_number}-{title}",
			vars: PatternVariables{
				RepoRoot: "/home/user/projects/myrepo",
				Title:    "feature",
			},
			expected: "/home/user/projects/myrepo/worktree/feature",
		},
		{
			name:    "tilde expansion",
			pattern: "~/src/worktree/{issue_number}-{title}",
			vars: PatternVariables{
				IssueNumber: "789",
				Title:       "test-feature",
			},
			expected: filepath.Join(homeDir, "src/worktree/789-test-feature"),
		},
		{
			name:    "empty issue number with delimiter",
			pattern: "{repo_root}/worktree/{issue_number}-{title}",
			vars: PatternVariables{
				RepoRoot:    "/home/user/projects/myrepo",
				IssueNumber: "",
				Title:       "feature",
			},
			expected: "/home/user/projects/myrepo/worktree/feature",
		},
		{
			name:    "empty title with delimiter",
			pattern: "{repo_root}/worktree/{issue_number}-{title}",
			vars: PatternVariables{
				RepoRoot:    "/home/user/projects/myrepo",
				IssueNumber: "123",
				Title:       "",
			},
			expected: "/home/user/projects/myrepo/worktree/123",
		},
		{
			name:    "multiple empty variables",
			pattern: "{repo_root}/worktree/{issue_number}-{title}_{timestamp}",
			vars: PatternVariables{
				RepoRoot:    "/home/user/projects/myrepo",
				IssueNumber: "",
				Title:       "",
				Timestamp:   "abc123",
			},
			expected: "/home/user/projects/myrepo/worktree/abc123",
		},
		{
			name:    "path separator preservation",
			pattern: "{repo_root}/worktree/{issue_number}/{title}",
			vars: PatternVariables{
				RepoRoot:    "/home/user/projects/myrepo",
				IssueNumber: "",
				Title:       "feature",
			},
			expected: "/home/user/projects/myrepo/worktree/feature",
		},
		{
			name:    "tilde with empty issue number",
			pattern: "~/src/worktree/{issue_number}-{title}",
			vars: PatternVariables{
				IssueNumber: "",
				Title:       "test-feature",
			},
			expected: filepath.Join(homeDir, "src/worktree/test-feature"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseWorktreePattern(tt.pattern, tt.vars)
			if result != tt.expected {
				t.Errorf("parseWorktreePattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractIssueNumber(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		expected    string
	}{
		{
			name:        "hash prefix",
			sessionName: "#123-fix-bug",
			expected:    "123",
		},
		{
			name:        "issue dash prefix",
			sessionName: "issue-456",
			expected:    "456",
		},
		{
			name:        "issue slash prefix",
			sessionName: "issue/789",
			expected:    "789",
		},
		{
			name:        "number with dash",
			sessionName: "111-feature",
			expected:    "111",
		},
		{
			name:        "number with underscore",
			sessionName: "222_feature",
			expected:    "222",
		},
		{
			name:        "just number",
			sessionName: "333",
			expected:    "333",
		},
		{
			name:        "no number",
			sessionName: "new-feature",
			expected:    "",
		},
		{
			name:        "multiple patterns",
			sessionName: "issue-123-#456",
			expected:    "456", // #456 pattern matches first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIssueNumber(tt.sessionName)
			if result != tt.expected {
				t.Errorf("extractIssueNumber(%s) = %v, want %v", tt.sessionName, result, tt.expected)
			}
		})
	}
}
