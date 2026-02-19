package session

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Activity represents what an agent is currently doing.
type Activity struct {
	// Action is the type of activity (e.g. "editing", "running", "reading", "searching", "working").
	Action string
	// Detail provides additional context (e.g. filename or command).
	Detail string
	// Timestamp is when this activity was detected.
	Timestamp time.Time
}

// ansiRegex strips ANSI escape codes from terminal output.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// claudeSpinnerAction matches Claude's spinner lines like "⠙ Editing src/auth.go"
// The spinner character varies, so we match any single character before the action word.
var claudeEditingRegex = regexp.MustCompile(`(?:Editing|Writing)\s+(.+)`)
var claudeReadingRegex = regexp.MustCompile(`Reading\s+(.+)`)
var claudeRunningRegex = regexp.MustCompile(`Running\s+(.+)`)
var claudeSearchingRegex = regexp.MustCompile(`Searching`)
var claudeShellCmdRegex = regexp.MustCompile(`\$\s+(.+)`)

// aiderEditingRegex matches Aider's editing pattern.
var aiderEditingRegex = regexp.MustCompile(`Editing\s+(.+)`)

// ParseActivity parses the pane content to extract the current activity.
// It scans the last ~30 lines for known patterns. program is the agent name
// (e.g. "claude", "aider"). Returns nil if no activity is detected.
func ParseActivity(content string, program string) *Activity {
	clean := ansiRegex.ReplaceAllString(content, "")

	lines := strings.Split(clean, "\n")

	// Only scan the last 30 lines for performance.
	start := len(lines) - 30
	if start < 0 {
		start = 0
	}
	tail := lines[start:]

	// Scan from the bottom up so we find the most recent activity first.
	for i := len(tail) - 1; i >= 0; i-- {
		line := strings.TrimSpace(tail[i])
		if line == "" {
			continue
		}

		if strings.Contains(strings.ToLower(program), "claude") {
			if a := parseClaudeLine(line); a != nil {
				return a
			}
		} else if strings.Contains(strings.ToLower(program), "aider") {
			if a := parseAiderLine(line); a != nil {
				return a
			}
		}

		// Generic patterns that work for any agent.
		if a := parseGenericLine(line); a != nil {
			return a
		}
	}

	return nil
}

func parseClaudeLine(line string) *Activity {
	if m := claudeEditingRegex.FindStringSubmatch(line); m != nil {
		return &Activity{
			Action:    "editing",
			Detail:    truncateDetail(cleanFilename(m[1]), 40),
			Timestamp: time.Now(),
		}
	}
	if m := claudeReadingRegex.FindStringSubmatch(line); m != nil {
		return &Activity{
			Action:    "reading",
			Detail:    truncateDetail(cleanFilename(m[1]), 40),
			Timestamp: time.Now(),
		}
	}
	if m := claudeRunningRegex.FindStringSubmatch(line); m != nil {
		return &Activity{
			Action:    "running",
			Detail:    truncateDetail(strings.TrimSpace(m[1]), 40),
			Timestamp: time.Now(),
		}
	}
	if claudeSearchingRegex.MatchString(line) {
		return &Activity{
			Action:    "searching",
			Detail:    "",
			Timestamp: time.Now(),
		}
	}
	if m := claudeShellCmdRegex.FindStringSubmatch(line); m != nil {
		return &Activity{
			Action:    "running",
			Detail:    truncateDetail(strings.TrimSpace(m[1]), 40),
			Timestamp: time.Now(),
		}
	}
	return nil
}

func parseAiderLine(line string) *Activity {
	if m := aiderEditingRegex.FindStringSubmatch(line); m != nil {
		return &Activity{
			Action:    "editing",
			Detail:    truncateDetail(cleanFilename(m[1]), 40),
			Timestamp: time.Now(),
		}
	}
	return nil
}

func parseGenericLine(line string) *Activity {
	// Try to detect shell commands from common prompt patterns.
	if m := claudeShellCmdRegex.FindStringSubmatch(line); m != nil {
		return &Activity{
			Action:    "running",
			Detail:    truncateDetail(strings.TrimSpace(m[1]), 40),
			Timestamp: time.Now(),
		}
	}
	return nil
}

// cleanFilename extracts just the basename if the detail looks like a file path.
func cleanFilename(s string) string {
	s = strings.TrimSpace(s)
	// If it contains path separators, use only the base name for brevity.
	if strings.Contains(s, "/") {
		return filepath.Base(s)
	}
	return s
}

// truncateDetail truncates a string to maxLen characters, appending "..." if truncated.
func truncateDetail(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
