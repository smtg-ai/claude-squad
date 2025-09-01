package git

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// PatternVariables holds the available variables for worktree pattern substitution
type PatternVariables struct {
	RepoRoot    string
	RepoName    string
	IssueNumber string
	Title       string
	Timestamp   string
}

// parseWorktreePattern substitutes variables in the pattern with actual values
func parseWorktreePattern(pattern string, vars PatternVariables) string {
	if pattern == "" {
		return ""
	}

	// If timestamp is empty, generate it
	if vars.Timestamp == "" {
		vars.Timestamp = fmt.Sprintf("%x", time.Now().UnixNano())
	}

	// Convert the pattern from {variable} format to {{.Variable}} format for text/template
	templatePattern := convertToTemplateFormat(pattern)

	// Create and parse the template
	tmpl, err := template.New("worktree").Parse(templatePattern)
	if err != nil {
		// If template parsing fails, fallback to original pattern
		return pattern
	}

	// Execute the template with the variables
	var result strings.Builder
	err = tmpl.Execute(&result, vars)
	if err != nil {
		// If template execution fails, fallback to original pattern
		return pattern
	}

	output := result.String()

	// Clean up delimiters from empty variables
	output = cleanupDelimiters(output)

	// Expand tilde to home directory
	if strings.HasPrefix(output, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			output = filepath.Join(homeDir, output[2:])
		}
	}

	// Clean up the path
	output = filepath.Clean(output)

	return output
}

// convertToTemplateFormat converts {variable} format to {{.Variable}} format for text/template
func convertToTemplateFormat(pattern string) string {
	// Map of old format to new format
	replacements := map[string]string{
		"{repo_root}":    "{{.RepoRoot}}",
		"{repo_name}":    "{{.RepoName}}",
		"{issue_number}": "{{.IssueNumber}}",
		"{title}":        "{{.Title}}",
		"{timestamp}":    "{{.Timestamp}}",
	}

	result := pattern
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// cleanupDelimiters removes unnecessary delimiters left by empty variables
func cleanupDelimiters(s string) string {
	// Common delimiters to clean up
	delimiters := "-_.:"

	// Remove leading delimiters
	s = strings.TrimLeft(s, delimiters)

	// Remove trailing delimiters
	s = strings.TrimRight(s, delimiters)

	// Replace multiple consecutive delimiters with a single one
	// We need to handle each delimiter type separately to preserve the original delimiter
	for _, delim := range delimiters {
		delimStr := string(delim)
		multiple := delimStr + delimStr
		for strings.Contains(s, multiple) {
			s = strings.ReplaceAll(s, multiple, delimStr)
		}
	}

	// Special case: remove delimiter before or after path separator
	// e.g., "/-" -> "/", "-/" -> "/"
	for _, delim := range delimiters {
		s = strings.ReplaceAll(s, "/"+string(delim), "/")
		s = strings.ReplaceAll(s, string(delim)+"/", "/")
	}

	return s
}

// extractIssueNumber attempts to extract an issue number from the session name
func extractIssueNumber(sessionName string) string {
	// Look for patterns like "#123", "issue-123", "issue/123", etc.
	patterns := []string{
		`#(\d+)`,
		`issue[-/](\d+)`,
		`(\d+)[-_]`,
		`^(\d+)$`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(sessionName)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}
