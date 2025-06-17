package ui

import (
	"claude-squad/session"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Enhanced prompt styles
	promptUserStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#22c55e", Dark: "#22c55e"}).
			Bold(true)
	promptPathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#3b82f6", Dark: "#60a5fa"}).
			Bold(true)
	promptGitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#f59e0b", Dark: "#fbbf24"})
	promptArrowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"})
	promptErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#ef4444", Dark: "#f87171"}).
				Bold(true)
	promptSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#22c55e", Dark: "#4ade80"}).
				Bold(true)
)

type promptInfo struct {
	user      string
	hostname  string
	directory string
	gitBranch string
	gitDirty  bool
}

// buildEnhancedPrompt creates a simple, clean prompt
func buildEnhancedPrompt(instance *session.Instance) string {
	if instance == nil {
		return promptArrowStyle.Render("❯ ")
	}

	prompt := getPromptInfo(instance)
	var parts []string

	// Git branch only (if available)
	if prompt.gitBranch != "" {
		gitInfo := prompt.gitBranch
		if prompt.gitDirty {
			gitInfo += "*" // Add asterisk for dirty repo
		}
		parts = append(parts, promptGitStyle.Render(fmt.Sprintf("(%s)", gitInfo)))
	} else if instance.Started() && instance.Title != "" {
		// Fallback: show instance title if no git branch
		parts = append(parts, promptGitStyle.Render(fmt.Sprintf("(%s)", instance.Title)))
	}

	// Join parts with spaces
	promptLine := strings.Join(parts, " ")

	// Add arrow and final prompt symbol
	if promptLine != "" {
		promptLine += " "
	}
	promptLine += promptArrowStyle.Render("❯ ")

	return promptLine
}

// getPromptInfo extracts context information for the prompt
func getPromptInfo(instance *session.Instance) promptInfo {
	info := promptInfo{}

	// Get user info
	if currentUser, err := user.Current(); err == nil {
		info.user = currentUser.Username
	}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		info.hostname = hostname
	}

	// Get current directory from instance's worktree
	if instance.Started() {
		if worktree, err := instance.GetGitWorktree(); err == nil {
			info.directory = worktree.GetWorktreePath()

			// Get git branch
			info.gitBranch = worktree.GetBranchName()

			// Check if git repo is dirty
			if dirty, err := worktree.IsDirty(); err == nil {
				info.gitDirty = dirty
			}
		}
	}

	return info
}

// enhanceContent adds enhanced prompt to content
func enhanceContent(content string, instance *session.Instance) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var enhancedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// More precise prompt detection: look for user@host pattern followed by $ or %
		isPromptLine := false

		// Pattern 1: user@host ... $ or %
		if strings.Contains(line, "@") && (strings.HasSuffix(trimmed, "$") || strings.HasSuffix(trimmed, "%")) {
			isPromptLine = true
		}

		// Pattern 2: lines that end with " $ " or " % " (with trailing space)
		if strings.HasSuffix(line, " $ ") || strings.HasSuffix(line, " % ") {
			isPromptLine = true
		}

		if isPromptLine {
			// Replace with our enhanced prompt
			enhancedPrompt := buildEnhancedPrompt(instance)
			enhancedLines = append(enhancedLines, enhancedPrompt)
		} else {
			enhancedLines = append(enhancedLines, line)
		}
	}

	return strings.Join(enhancedLines, "\n")
}
