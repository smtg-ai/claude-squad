package overlay

import (
	"os"
	"path/filepath"
	"testing"
)

// mkdirs creates the given directories under root.
func mkdirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.Mkdir(filepath.Join(root, name), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}
}

func TestCompletePathUniqueMatchCompletesWithTrailingSlash(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "projects")

	got, candidates := CompletePath(filepath.Join(root, "proj"))

	want := filepath.Join(root, "projects") + string(filepath.Separator)
	if got != want {
		t.Errorf("CompletePath() = %q, want %q", got, want)
	}
	if candidates != nil {
		t.Errorf("candidates = %v, want nil", candidates)
	}
}

func TestCompletePathAmbiguousExtendsToCommonPrefixAndListsCandidates(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "backend", "backfill")

	got, candidates := CompletePath(filepath.Join(root, "ba"))

	want := filepath.Join(root, "back")
	if got != want {
		t.Errorf("CompletePath() = %q, want %q", got, want)
	}
	if len(candidates) != 2 || candidates[0] != "backend" || candidates[1] != "backfill" {
		t.Errorf("candidates = %v, want [backend backfill]", candidates)
	}
}

func TestCompletePathNoMatchReturnsInputUnchanged(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "src")

	input := filepath.Join(root, "zzz")
	got, candidates := CompletePath(input)

	if got != input {
		t.Errorf("CompletePath() = %q, want %q", got, input)
	}
	if candidates != nil {
		t.Errorf("candidates = %v, want nil", candidates)
	}
}

func TestCompletePathIgnoresFiles(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "notebooks")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got, _ := CompletePath(filepath.Join(root, "not"))

	want := filepath.Join(root, "notebooks") + string(filepath.Separator)
	if got != want {
		t.Errorf("CompletePath() = %q, want %q", got, want)
	}
}

func TestCompletePathTrailingSlashListsAllSubdirectories(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "alpha", "beta")

	input := root + string(filepath.Separator)
	got, candidates := CompletePath(input)

	if got != input {
		t.Errorf("CompletePath() = %q, want %q", got, input)
	}
	if len(candidates) != 2 || candidates[0] != "alpha" || candidates[1] != "beta" {
		t.Errorf("candidates = %v, want [alpha beta]", candidates)
	}
}

func TestCompletePathSkipsHiddenUnlessPrefixIsDotted(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, ".git", "src")

	_, candidates := CompletePath(root + string(filepath.Separator))
	if len(candidates) != 0 {
		// "src" is a unique match for the empty prefix once hidden dirs are
		// skipped, so the input completes with no candidate list.
		t.Errorf("candidates = %v, want none (hidden dirs skipped)", candidates)
	}

	got, _ := CompletePath(filepath.Join(root, ".g"))
	want := filepath.Join(root, ".git") + string(filepath.Separator)
	if got != want {
		t.Errorf("CompletePath() = %q, want %q", got, want)
	}
}

func TestCompletePathEmptyInputUnchanged(t *testing.T) {
	got, candidates := CompletePath("")
	if got != "" || candidates != nil {
		t.Errorf("CompletePath(\"\") = %q, %v; want \"\", nil", got, candidates)
	}
}

func TestCompletePathPreservesTildePrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home, "workspace")

	got, _ := CompletePath("~/works")

	want := "~/workspace" + string(filepath.Separator)
	if got != want {
		t.Errorf("CompletePath() = %q, want %q", got, want)
	}
}

func TestExpandTilde(t *testing.T) {
	tests := []struct {
		name  string
		input string
		home  string
		want  string
	}{
		{"bare tilde", "~", "/home/u", "/home/u"},
		{"tilde with path", "~/code", "/home/u", "/home/u/code"},
		{"no tilde", "/opt/x", "/home/u", "/opt/x"},
		{"tilde mid-path untouched", "/a/~b", "/home/u", "/a/~b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandTilde(tt.input, tt.home); got != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
