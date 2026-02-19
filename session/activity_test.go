package session

import (
	"strings"
	"testing"
)

func TestParseActivity_ClaudeEditing(t *testing.T) {
	content := strings.Join([]string{
		"some previous output",
		"",
		"\x1b[36m⠙\x1b[0m Editing src/auth.go",
		"",
	}, "\n")

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "editing" {
		t.Errorf("expected action 'editing', got %q", a.Action)
	}
	if a.Detail != "auth.go" {
		t.Errorf("expected detail 'auth.go', got %q", a.Detail)
	}
}

func TestParseActivity_ClaudeWriting(t *testing.T) {
	content := "⠙ Writing new_file.ts\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "editing" {
		t.Errorf("expected action 'editing', got %q", a.Action)
	}
	if a.Detail != "new_file.ts" {
		t.Errorf("expected detail 'new_file.ts', got %q", a.Detail)
	}
}

func TestParseActivity_ClaudeReading(t *testing.T) {
	content := "⠙ Reading config/settings.yaml\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "reading" {
		t.Errorf("expected action 'reading', got %q", a.Action)
	}
	if a.Detail != "settings.yaml" {
		t.Errorf("expected detail 'settings.yaml', got %q", a.Detail)
	}
}

func TestParseActivity_ClaudeRunning(t *testing.T) {
	content := "⠙ Running go test ./...\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "running" {
		t.Errorf("expected action 'running', got %q", a.Action)
	}
	if a.Detail != "go test ./..." {
		t.Errorf("expected detail 'go test ./...', got %q", a.Detail)
	}
}

func TestParseActivity_ClaudeSearching(t *testing.T) {
	content := "⠙ Searching for references...\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "searching" {
		t.Errorf("expected action 'searching', got %q", a.Action)
	}
}

func TestParseActivity_ClaudeShellCommand(t *testing.T) {
	content := "$ npm run build\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "running" {
		t.Errorf("expected action 'running', got %q", a.Action)
	}
	if a.Detail != "npm run build" {
		t.Errorf("expected detail 'npm run build', got %q", a.Detail)
	}
}

func TestParseActivity_AiderEditing(t *testing.T) {
	content := "Editing src/components/Button.tsx\n"

	a := ParseActivity(content, "aider")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "editing" {
		t.Errorf("expected action 'editing', got %q", a.Action)
	}
	if a.Detail != "Button.tsx" {
		t.Errorf("expected detail 'Button.tsx', got %q", a.Detail)
	}
}

func TestParseActivity_NoMatch(t *testing.T) {
	content := "Hello world\nNothing interesting here\n"

	a := ParseActivity(content, "claude")
	if a != nil {
		t.Errorf("expected nil, got %+v", a)
	}
}

func TestParseActivity_OnlyScansLast30Lines(t *testing.T) {
	// Put an editing pattern at line 1, then 40 blank lines.
	// The pattern should NOT be found because it's outside the last 30 lines.
	var lines []string
	lines = append(lines, "⠙ Editing old_file.go")
	for i := 0; i < 40; i++ {
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")

	a := ParseActivity(content, "claude")
	if a != nil {
		t.Errorf("expected nil (pattern outside last 30 lines), got %+v", a)
	}
}

func TestParseActivity_BottomUpScanning(t *testing.T) {
	// Two activities: reading first, then editing. Should pick editing (most recent).
	content := strings.Join([]string{
		"⠙ Reading old.go",
		"⠙ Editing new.go",
		"",
	}, "\n")

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "editing" {
		t.Errorf("expected 'editing' (most recent), got %q", a.Action)
	}
	if a.Detail != "new.go" {
		t.Errorf("expected detail 'new.go', got %q", a.Detail)
	}
}

func TestParseActivity_ANSIStripping(t *testing.T) {
	// Content with heavy ANSI codes.
	content := "\x1b[1m\x1b[36m⠙\x1b[0m \x1b[33mEditing\x1b[0m \x1b[4msrc/main.go\x1b[0m\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if a.Action != "editing" {
		t.Errorf("expected action 'editing', got %q", a.Action)
	}
	if a.Detail != "main.go" {
		t.Errorf("expected detail 'main.go', got %q", a.Detail)
	}
}

func TestParseActivity_TruncateLongDetail(t *testing.T) {
	longCmd := strings.Repeat("a", 60)
	content := "$ " + longCmd + "\n"

	a := ParseActivity(content, "claude")
	if a == nil {
		t.Fatal("expected activity, got nil")
	}
	if len(a.Detail) > 40 {
		t.Errorf("expected detail to be truncated to 40 chars, got %d: %q", len(a.Detail), a.Detail)
	}
	if !strings.HasSuffix(a.Detail, "...") {
		t.Errorf("expected truncated detail to end with '...', got %q", a.Detail)
	}
}

func TestTruncateDetail(t *testing.T) {
	tests := []struct {
		input  string
		max    int
		expect string
	}{
		{"short", 40, "short"},
		{"exactly40charsxxxxxxxxxxxxxxxxxxxxxxxxx", 40, "exactly40charsxxxxxxxxxxxxxxxxxxxxxxxxx"},
		{strings.Repeat("x", 50), 40, strings.Repeat("x", 37) + "..."},
		{"ab", 3, "ab"},
		{"abcd", 3, "abc"},
	}

	for _, tc := range tests {
		result := truncateDetail(tc.input, tc.max)
		if result != tc.expect {
			t.Errorf("truncateDetail(%q, %d) = %q, want %q", tc.input, tc.max, result, tc.expect)
		}
	}
}

func TestCleanFilename(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"auth.go", "auth.go"},
		{"src/auth.go", "auth.go"},
		{"src/components/Button.tsx", "Button.tsx"},
		{" auth.go ", "auth.go"},
	}

	for _, tc := range tests {
		result := cleanFilename(tc.input)
		if result != tc.expect {
			t.Errorf("cleanFilename(%q) = %q, want %q", tc.input, result, tc.expect)
		}
	}
}
