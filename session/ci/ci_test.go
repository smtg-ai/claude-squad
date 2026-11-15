package ci

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		entries []rollupEntry
		want    State
		passed  int
		failed  int
		pending int
	}{
		{
			name:    "empty rollup has no verdict",
			entries: nil,
			want:    StateUnknown,
		},
		{
			name: "all conclusions passed",
			entries: []rollupEntry{
				{Status: "COMPLETED", Conclusion: "SUCCESS"},
				{Status: "COMPLETED", Conclusion: "NEUTRAL"},
			},
			want:   StateSuccess,
			passed: 2,
		},
		{
			// The reason the switch is ordered worst-first: a red check is the
			// thing the badge exists to surface, even mid-run.
			name: "a failure outranks checks still running",
			entries: []rollupEntry{
				{Status: "COMPLETED", Conclusion: "FAILURE"},
				{Status: "IN_PROGRESS"},
				{Status: "COMPLETED", Conclusion: "SUCCESS"},
			},
			want:    StateFailure,
			passed:  1,
			failed:  1,
			pending: 1,
		},
		{
			name: "pending outranks success",
			entries: []rollupEntry{
				{Status: "QUEUED"},
				{Status: "COMPLETED", Conclusion: "SUCCESS"},
			},
			want:    StatePending,
			passed:  1,
			pending: 1,
		},
		{
			// Path-filtered jobs and superseded runs are the normal case on a
			// healthy branch; counting them red would leave most branches red.
			name: "skipped and cancelled are ignored, not failures",
			entries: []rollupEntry{
				{Status: "COMPLETED", Conclusion: "SKIPPED"},
				{Status: "COMPLETED", Conclusion: "CANCELLED"},
				{Status: "COMPLETED", Conclusion: "SUCCESS"},
			},
			want:   StateSuccess,
			passed: 1,
		},
		{
			name: "a rollup of nothing but skips has no verdict",
			entries: []rollupEntry{
				{Status: "COMPLETED", Conclusion: "SKIPPED"},
				{Status: "COMPLETED", Conclusion: "CANCELLED"},
			},
			want: StateUnknown,
		},
		{
			name: "timed out and action required count as failures",
			entries: []rollupEntry{
				{Status: "COMPLETED", Conclusion: "TIMED_OUT"},
				{Status: "COMPLETED", Conclusion: "ACTION_REQUIRED"},
			},
			want:   StateFailure,
			failed: 2,
		},
		{
			// Legacy StatusContext entries carry neither conclusion nor status.
			name: "legacy status contexts are read from state",
			entries: []rollupEntry{
				{State: "FAILURE"},
				{State: "SUCCESS"},
			},
			want:   StateFailure,
			passed: 1,
			failed: 1,
		},
		{
			name: "lowercase outcomes are normalized",
			entries: []rollupEntry{
				{Conclusion: "success"},
			},
			want:   StateSuccess,
			passed: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.entries)
			if got.State != tt.want {
				t.Errorf("State = %v, want %v", got.State, tt.want)
			}
			if got.Passed != tt.passed {
				t.Errorf("Passed = %d, want %d", got.Passed, tt.passed)
			}
			if got.Failed != tt.failed {
				t.Errorf("Failed = %d, want %d", got.Failed, tt.failed)
			}
			if got.Pending != tt.pending {
				t.Errorf("Pending = %d, want %d", got.Pending, tt.pending)
			}
		})
	}
}

// A CheckRun reports `status` while running and `conclusion` once done; the
// conclusion must win, or a finished check reads as still pending.
func TestOutcomeOfPrefersConclusion(t *testing.T) {
	got := outcomeOf(rollupEntry{Status: "COMPLETED", Conclusion: "FAILURE", State: "SUCCESS"})
	if got != "FAILURE" {
		t.Errorf("outcomeOf = %q, want FAILURE", got)
	}

	got = outcomeOf(rollupEntry{Status: "IN_PROGRESS", State: "SUCCESS"})
	if got != "IN_PROGRESS" {
		t.Errorf("outcomeOf = %q, want IN_PROGRESS", got)
	}
}

// Get must answer instantly from cache; a network round trip on the 500ms
// metadata tick would stall the diff stats computed alongside it.
func TestGetIsNonBlockingAndSkipsBlankInput(t *testing.T) {
	if s := Get("", "/worktree", "branch"); s.State != StateUnknown {
		t.Errorf("blank repo path: State = %v, want StateUnknown", s.State)
	}
	if s := Get("/repo", "", "branch"); s.State != StateUnknown {
		t.Errorf("blank worktree path: State = %v, want StateUnknown", s.State)
	}
}

// A merged PR's checks are all green, so without this the row would look exactly
// like an open branch that is passing -- and an abandoned PR would too.
func TestTerminalState(t *testing.T) {
	tests := []struct {
		prState string
		want    State
	}{
		{"MERGED", StateMerged},
		{"merged", StateMerged},
		{"CLOSED", StateClosed},
		{"closed", StateClosed},
		{"OPEN", StateUnknown},
		{"", StateUnknown},
	}

	for _, tt := range tests {
		if got := terminalState(tt.prState); got != tt.want {
			t.Errorf("terminalState(%q) = %v, want %v", tt.prState, got, tt.want)
		}
	}
}
