package session

import (
	"claude-squad/log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	defer log.Close()
	os.Exit(m.Run())
}

func TestInPlaceRepoName(t *testing.T) {
	inst := &Instance{
		Path:    "/some/path/myproject",
		started: true,
		inPlace: true,
	}
	name, err := inst.RepoName()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "myproject" {
		t.Fatalf("expected 'myproject', got '%s'", name)
	}
}

func TestInPlaceUpdateDiffStatsNoOp(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
	}
	if err := inst.UpdateDiffStats(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.diffStats != nil {
		t.Error("expected nil diffStats for in-place session")
	}
}

func TestInPlaceGetGitWorktreeReturnsNil(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
	}
	wt, err := inst.GetGitWorktree()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt != nil {
		t.Error("expected nil worktree for in-place session")
	}
}

func TestInPlacePauseDoesNotPanic(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
		Status:  Running,
	}
	// Pause will fail because tmuxSession is nil, but it must NOT panic
	// on nil gitWorktree access
	err := inst.Pause()
	if err == nil {
		t.Error("expected error due to nil tmux session")
	}
	if err.Error() != "tmux session is nil" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInPlaceResumeDoesNotPanic(t *testing.T) {
	inst := &Instance{
		started: true,
		inPlace: true,
		Status:  Paused,
	}
	// Resume will fail because tmuxSession is nil, but it must NOT panic
	// on nil gitWorktree access
	err := inst.Resume()
	if err == nil {
		t.Error("expected error due to nil tmux session")
	}
	if err.Error() != "tmux session is nil" {
		t.Fatalf("unexpected error: %v", err)
	}
}
