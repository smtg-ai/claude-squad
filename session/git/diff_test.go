package git

import "testing"

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "empty output",
			input:       "",
			wantAdded:   0,
			wantRemoved: 0,
		},
		{
			name:        "single file",
			input:       "3\t1\tfoo.go\n",
			wantAdded:   3,
			wantRemoved: 1,
		},
		{
			name:        "multiple files sum correctly",
			input:       "3\t1\tfoo.go\n10\t2\tbar/baz.go\n",
			wantAdded:   13,
			wantRemoved: 3,
		},
		{
			name:        "binary files are skipped",
			input:       "5\t0\tfoo.go\n-\t-\timage.png\n2\t2\tbar.go\n",
			wantAdded:   7,
			wantRemoved: 2,
		},
		{
			name:        "path with tabs is preserved via SplitN",
			input:       "4\t4\tpath\twith\ttabs.go\n",
			wantAdded:   4,
			wantRemoved: 4,
		},
		{
			name:        "trailing newlines do not add garbage",
			input:       "1\t0\ta.go\n\n\n",
			wantAdded:   1,
			wantRemoved: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdded, gotRemoved := parseNumstat(tt.input)
			if gotAdded != tt.wantAdded || gotRemoved != tt.wantRemoved {
				t.Errorf("parseNumstat(%q) = (%d, %d), want (%d, %d)",
					tt.input, gotAdded, gotRemoved, tt.wantAdded, tt.wantRemoved)
			}
		})
	}
}
