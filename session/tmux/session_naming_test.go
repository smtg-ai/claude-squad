package tmux

import (
	"claude-squad/cmd"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// mockExecutor for testing session name conflicts
type mockExecutor struct {
	existingSessions map[string]bool
	runCalls         []string
	outputCalls      []string
}

func (m *mockExecutor) Run(cmd *exec.Cmd) error {
	cmdStr := strings.Join(cmd.Args, " ")
	m.runCalls = append(m.runCalls, cmdStr)

	// Mock tmux has-session behavior
	if strings.Contains(cmdStr, "has-session") {
		// Extract session name from -t= flag
		for _, arg := range cmd.Args {
			if strings.HasPrefix(arg, "-t=") {
				sessionName := arg[3:] // Remove "-t="
				if m.existingSessions[sessionName] {
					return nil // Session exists (exit code 0)
				}
				return fmt.Errorf("session not found") // Session doesn't exist (exit code 1)
			}
		}
	}

	// Mock successful execution for other commands
	return nil
}

func (m *mockExecutor) Output(cmd *exec.Cmd) ([]byte, error) {
	cmdStr := strings.Join(cmd.Args, " ")
	m.outputCalls = append(m.outputCalls, cmdStr)
	return []byte("mock output"), nil
}

func newMockExecutor(existingSessions []string) *mockExecutor {
	sessionMap := make(map[string]bool)
	for _, session := range existingSessions {
		sessionMap[session] = true
	}
	return &mockExecutor{
		existingSessions: sessionMap,
		runCalls:         make([]string, 0),
		outputCalls:      make([]string, 0),
	}
}

func TestSessionNameConflictResolution(t *testing.T) {
	tests := []struct {
		name             string
		baseName         string
		existingSessions []string
		expectConflict   bool
		expectUnique     bool
	}{
		{
			name:             "No conflict - unique name",
			baseName:         "claudesquad_myproject",
			existingSessions: []string{},
			expectConflict:   false,
			expectUnique:     true,
		},
		{
			name:             "Conflict resolved with timestamp",
			baseName:         "claudesquad_myproject",
			existingSessions: []string{"claudesquad_myproject"},
			expectConflict:   true,
			expectUnique:     true,
		},
		{
			name:             "Multiple conflicts resolved",
			baseName:         "claudesquad_popular",
			existingSessions: []string{"claudesquad_popular", "claudesquad_popular_1234567890"},
			expectConflict:   true,
			expectUnique:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := newMockExecutor(tt.existingSessions)
			session := &TmuxSession{
				sanitizedName: tt.baseName,
				cmdExec:       mockExec,
			}

			uniqueName := session.generateUniqueSessionName(tt.baseName)

			// Verify uniqueness
			if tt.expectUnique && mockExec.existingSessions[uniqueName] {
				t.Errorf("Generated name %s is not unique", uniqueName)
			}

			// Verify conflict detection
			if tt.expectConflict && uniqueName == tt.baseName {
				t.Errorf("Expected conflict resolution but got same name: %s", uniqueName)
			}

			if !tt.expectConflict && uniqueName != tt.baseName {
				t.Errorf("Expected no conflict but name changed: %s -> %s", tt.baseName, uniqueName)
			}

			// Verify name format
			if !strings.HasPrefix(uniqueName, tt.baseName) {
				t.Errorf("Generated name %s doesn't start with base name %s", uniqueName, tt.baseName)
			}
		})
	}
}

func TestGenerateUniqueSessionNameEdgeCases(t *testing.T) {
	t.Run("Extreme conflict scenario", func(t *testing.T) {
		// Create many existing sessions to test fallback logic
		existingSessions := []string{"claudesquad_test"}

		// Add timestamp-based conflicts (simulate rapid creation)
		timestamp := time.Now().Unix()
		for i := 0; i < 5; i++ {
			existingSessions = append(existingSessions, fmt.Sprintf("claudesquad_test_%d", timestamp))
		}

		// Add timestamp + random conflicts
		for i := 0; i < 10; i++ {
			existingSessions = append(existingSessions, fmt.Sprintf("claudesquad_test_%d_%d", timestamp, i))
		}

		mockExec := newMockExecutor(existingSessions)
		session := &TmuxSession{
			sanitizedName: "claudesquad_test",
			cmdExec:       mockExec,
		}

		uniqueName := session.generateUniqueSessionName("claudesquad_test")

		// Should still generate a unique name
		if mockExec.existingSessions[uniqueName] {
			t.Errorf("Failed to generate unique name even in extreme conflict scenario: %s", uniqueName)
		}

		// Should use final fallback format
		if !strings.Contains(uniqueName, "_") {
			t.Errorf("Expected fallback name format with underscores, got: %s", uniqueName)
		}
	})
}

func TestDoesSessionExistByName(t *testing.T) {
	tests := []struct {
		name             string
		sessionName      string
		existingSessions []string
		expected         bool
	}{
		{
			name:             "Session exists",
			sessionName:      "claudesquad_existing",
			existingSessions: []string{"claudesquad_existing"},
			expected:         true,
		},
		{
			name:             "Session does not exist",
			sessionName:      "claudesquad_missing",
			existingSessions: []string{"claudesquad_other"},
			expected:         false,
		},
		{
			name:             "Empty sessions list",
			sessionName:      "claudesquad_test",
			existingSessions: []string{},
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := newMockExecutor(tt.existingSessions)
			session := &TmuxSession{
				cmdExec: mockExec,
			}

			result := session.doesSessionExistByName(tt.sessionName)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v for session %s", tt.expected, result, tt.sessionName)
			}

			// Verify correct tmux command was called
			expectedCmd := fmt.Sprintf("tmux has-session -t=%s", tt.sessionName)
			found := false
			for _, call := range mockExec.runCalls {
				if strings.Contains(call, expectedCmd) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected tmux has-session command not found in calls: %v", mockExec.runCalls)
			}
		})
	}
}

func TestTmuxSessionStartWithConflictResolution(t *testing.T) {
	t.Run("Start with name conflict", func(t *testing.T) {
		// Mock existing session
		existingSessions := []string{"claudesquad_myproject"}
		mockExec := newMockExecutor(existingSessions)

		// Mock PTY factory that doesn't actually create PTY
		mockPtyFactory := &mockPtyFactory{}

		session := newTmuxSession("myproject", "claude", mockPtyFactory, mockExec)

		// Before start, session name should be base name
		if session.sanitizedName != "claudesquad_myproject" {
			t.Errorf("Expected base name, got: %s", session.sanitizedName)
		}

		// Note: We can't fully test Start() without mocking more components,
		// but we can test the name generation logic which is the core functionality
		uniqueName := session.generateUniqueSessionName(session.sanitizedName)

		// Should generate a different name due to conflict
		if uniqueName == "claudesquad_myproject" {
			t.Errorf("Expected unique name due to conflict, got same name: %s", uniqueName)
		}

		// Should start with base name
		if !strings.HasPrefix(uniqueName, "claudesquad_myproject") {
			t.Errorf("Generated name should start with base name, got: %s", uniqueName)
		}
	})
}

// Mock PTY factory for testing
type mockPtyFactory struct{}

func (m *mockPtyFactory) Start(cmd *exec.Cmd) (*os.File, error) {
	// Return a mock file descriptor - in real tests this would need proper mocking
	// For now, return an error to avoid actually creating processes
	return nil, fmt.Errorf("mock PTY factory - not implemented for testing")
}

func (m *mockPtyFactory) Close() {}

// Verify the mock executor implements the cmd.Executor interface
var _ cmd.Executor = (*mockExecutor)(nil)
