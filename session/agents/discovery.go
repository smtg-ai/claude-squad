package agents

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AgentSession represents a saved conversation that can be resumed.
type AgentSession struct {
	SessionID string
	Summary   string // first prompt or thread name
	WorkDir   string
	UpdatedAt time.Time
	Program   string // "claude" or "codex"
	ResumeCmd string // full command to resume, e.g. "claude --resume <id>"
}

// ListClaudeCodeSessions scans ~/.claude/projects/ for saved sessions.
func ListClaudeCodeSessions(maxResults int) ([]AgentSession, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectsDir := filepath.Join(homeDir, ".claude", "projects")

	var sessions []AgentSession

	// Walk all .jsonl files in projects dir
	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}

		sessionID := strings.TrimSuffix(info.Name(), ".jsonl")

		cwd, summary := parseClaudeSessionFile(path)

		// Truncate summary
		if len(summary) > 80 {
			summary = summary[:77] + "..."
		}
		if summary == "" {
			if len(sessionID) > 8 {
				summary = sessionID[:8] + "..."
			} else {
				summary = sessionID
			}
		}

		sessions = append(sessions, AgentSession{
			SessionID: sessionID,
			Summary:   summary,
			WorkDir:   cwd,
			UpdatedAt: info.ModTime(),
			Program:   "claude",
			ResumeCmd: "claude --resume " + sessionID,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by most recent
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	// Limit results
	if maxResults > 0 && len(sessions) > maxResults {
		sessions = sessions[:maxResults]
	}

	return sessions, nil
}

// parseClaudeSessionFile reads a session JSONL file and extracts the cwd and
// first user message summary.
func parseClaudeSessionFile(path string) (cwd, summary string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase buffer size to handle long lines in JSONL files
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)
		if entryType != "user" {
			continue
		}

		if c, ok := entry["cwd"].(string); ok && cwd == "" {
			cwd = c
		}
		if summary == "" {
			if msg, ok := entry["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok && content != "" {
					summary = content
				} else if contentList, ok := msg["content"].([]interface{}); ok {
					for _, item := range contentList {
						if block, ok := item.(map[string]interface{}); ok {
							if block["type"] == "text" {
								if text, ok := block["text"].(string); ok {
									summary = text
									break
								}
							}
						}
					}
				}
			}
		}
		if cwd != "" && summary != "" {
			break
		}
	}

	return cwd, summary
}

// ListCodexSessions reads ~/.codex/session_index.jsonl for saved sessions.
func ListCodexSessions(maxResults int) ([]AgentSession, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	indexPath := filepath.Join(homeDir, ".codex", "session_index.jsonl")

	f, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	type indexEntry struct {
		ID         string `json:"id"`
		ThreadName string `json:"thread_name"`
		UpdatedAt  string `json:"updated_at"`
	}

	var entries []indexEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry indexEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	var sessions []AgentSession
	for _, entry := range entries {
		updatedAt, _ := time.Parse(time.RFC3339Nano, entry.UpdatedAt)

		// Try to find the CWD from the session data file
		cwd := findCodexSessionCwd(homeDir, entry.ID)

		summary := entry.ThreadName
		if summary == "" {
			if len(entry.ID) > 8 {
				summary = entry.ID[:8] + "..."
			} else {
				summary = entry.ID
			}
		}

		sessions = append(sessions, AgentSession{
			SessionID: entry.ID,
			Summary:   summary,
			WorkDir:   cwd,
			UpdatedAt: updatedAt,
			Program:   "codex",
			ResumeCmd: "codex resume " + entry.ID,
		})
	}

	// Sort by most recent
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	if maxResults > 0 && len(sessions) > maxResults {
		sessions = sessions[:maxResults]
	}

	return sessions, nil
}

// findCodexSessionCwd searches for the session data file and extracts cwd from session_meta.
func findCodexSessionCwd(homeDir, sessionID string) string {
	sessionsDir := filepath.Join(homeDir, ".codex", "sessions")

	var cwd string
	filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || cwd != "" {
			return nil
		}
		if !strings.Contains(info.Name(), sessionID) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		if scanner.Scan() {
			line := scanner.Bytes()
			var entry map[string]interface{}
			if err := json.Unmarshal(line, &entry); err != nil {
				return nil
			}
			if entry["type"] == "session_meta" {
				if payload, ok := entry["payload"].(map[string]interface{}); ok {
					if c, ok := payload["cwd"].(string); ok {
						cwd = c
						return filepath.SkipAll
					}
				}
			}
		}
		return nil
	})

	return cwd
}

// TimeAgo returns a human-readable relative time string.
func TimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return formatInt(m) + " minutes ago"
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return formatInt(h) + " hours ago"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		days := int(d.Hours() / 24)
		return formatInt(days) + " days ago"
	}
}

func formatInt(n int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}
