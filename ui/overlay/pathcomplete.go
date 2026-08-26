package overlay

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// expandTilde replaces a leading "~" with the given home directory.
func expandTilde(path string, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// ExpandUserPath expands a leading "~" in path using the current user's home
// directory. Returns the path unchanged if the home directory can't be resolved.
func ExpandUserPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return expandTilde(path, home)
}

// CompletePath performs shell-style tab completion for directory paths.
// It completes the last path segment against the directories on disk:
// a unique match extends the input (with a trailing separator), an ambiguous
// match extends to the longest common prefix and returns the sorted candidate
// names, and no match returns the input unchanged. Hidden directories are
// only considered when the segment being completed starts with a dot.
// A leading "~" is honored for the lookup but preserved in the returned path.
func CompletePath(input string) (string, []string) {
	if input == "" {
		return input, nil
	}

	// Split into the directory part (kept verbatim in the result) and the
	// partial segment being completed.
	sep := strings.LastIndex(input, string(filepath.Separator))
	dirPart, prefix := "", input
	if sep >= 0 {
		dirPart, prefix = input[:sep+1], input[sep+1:]
	}

	lookupDir := ExpandUserPath(dirPart)
	if lookupDir == "" {
		lookupDir = "."
	}

	entries, err := os.ReadDir(lookupDir)
	if err != nil {
		return input, nil
	}

	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		matches = append(matches, name)
	}
	sort.Strings(matches)

	switch len(matches) {
	case 0:
		return input, nil
	case 1:
		return dirPart + matches[0] + string(filepath.Separator), nil
	default:
		return dirPart + longestCommonPrefix(matches), matches
	}
}

func longestCommonPrefix(names []string) string {
	prefix := names[0]
	for _, name := range names[1:] {
		for !strings.HasPrefix(name, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}
