package ui

import (
	"strings"
	"testing"
)

func TestGradientText_EmptyString(t *testing.T) {
	result := GradientText("", "#F0A868", "#7EC8D8")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestGradientText_SingleChar(t *testing.T) {
	result := GradientText("A", "#F0A868", "#7EC8D8")
	if !strings.Contains(result, "A") {
		t.Errorf("expected result to contain 'A', got %q", result)
	}
	if !strings.Contains(result, "\033[") {
		t.Errorf("expected ANSI escape sequences, got %q", result)
	}
}

func TestGradientText_MultiChar(t *testing.T) {
	result := GradientText("ABCD", "#FF0000", "#0000FF")
	for _, c := range "ABCD" {
		if !strings.Contains(result, string(c)) {
			t.Errorf("expected result to contain %q", string(c))
		}
	}
	if !strings.HasSuffix(result, "\033[0m") {
		t.Errorf("expected ANSI reset at end")
	}
}

func TestGradientText_PreservesNewlines(t *testing.T) {
	result := GradientText("AB\nCD", "#FF0000", "#0000FF")
	if !strings.Contains(result, "\n") {
		t.Errorf("expected newline preserved, got %q", result)
	}
}

func TestParseHex(t *testing.T) {
	r, g, b := parseHex("#F0A868")
	if r != 0xF0 || g != 0xA8 || b != 0x68 {
		t.Errorf("expected (240, 168, 104), got (%d, %d, %d)", r, g, b)
	}
}

func TestGradientBar(t *testing.T) {
	result := GradientBar(10, 5, "#F0A868", "#7EC8D8")
	plain := stripAnsi(result)
	if len([]rune(plain)) != 10 {
		t.Errorf("expected 10 runes, got %d: %q", len([]rune(plain)), plain)
	}
	if !strings.Contains(plain, "█") {
		t.Errorf("expected filled blocks, got %q", plain)
	}
	if !strings.Contains(plain, "░") {
		t.Errorf("expected empty blocks, got %q", plain)
	}
}

func TestGradientBar_ZeroFilled(t *testing.T) {
	result := GradientBar(5, 0, "#F0A868", "#7EC8D8")
	plain := stripAnsi(result)
	if len([]rune(plain)) != 5 {
		t.Errorf("expected 5 runes, got %d", len([]rune(plain)))
	}
}

func TestGradientBar_FullFilled(t *testing.T) {
	result := GradientBar(5, 5, "#F0A868", "#7EC8D8")
	plain := stripAnsi(result)
	if strings.Contains(plain, "░") {
		t.Errorf("expected no empty blocks when fully filled")
	}
}

func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
