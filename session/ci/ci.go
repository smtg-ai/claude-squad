// Package ci reports the CI verdict for a session's branch, so the instance
// list can show whether that branch's pull request is passing without leaving
// the TUI.
//
// Everything here is best-effort by design: cs is useful without it, so a
// missing or unauthenticated gh CLI, a branch with no pull request, or a failing
// lookup all degrade to "no badge" rather than an error the user has to dismiss.
package ci

import (
	"claude-squad/log"
	"claude-squad/session/git"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// State is the overall verdict for a branch's CI checks.
type State int

const (
	// StateUnknown means no verdict is available yet: no lookup has completed,
	// gh is unavailable, or the pull request reported no classifiable checks.
	StateUnknown State = iota
	// StateNoPR means the branch has no open pull request.
	StateNoPR
	// StatePending means at least one check is queued or running, and none failed.
	StatePending
	// StateFailure means at least one check concluded in failure.
	StateFailure
	// StateSuccess means every check that reached a conclusion passed.
	StateSuccess
	// StateMerged means the pull request was merged. Checked before the individual
	// checks, since a merged PR's checks are all green and would otherwise be
	// indistinguishable from an open branch that is merely passing.
	StateMerged
	// StateClosed means the pull request was closed without being merged.
	StateClosed
)

// Status is the CI verdict for one branch, plus the counts it was derived from.
type Status struct {
	State    State
	PRNumber int
	Passed   int
	Failed   int
	Pending  int
	// FetchedAt is when the most recent lookup attempt completed. The zero value
	// means no attempt has completed yet.
	FetchedAt time.Time
}

const (
	// ttl is how long a verdict is served before a refresh is triggered. Chosen
	// to be far slower than the 500ms metadata tick that reads it: a GitHub round
	// trip per instance per tick would be both rate-limit-hostile and pointless,
	// since a workflow run takes minutes.
	ttl = 30 * time.Second
	// fetchTimeout bounds a single gh invocation so a hung network call cannot
	// pin a cache entry in the "fetching" state forever.
	fetchTimeout = 10 * time.Second
)

type entry struct {
	status Status
	// fetching guards against a second refresh being launched for a branch whose
	// first one is still in flight — the reader runs every 500ms and a fetch
	// takes ~1s, so without this every stale entry would spawn a pile of them.
	fetching bool
}

var (
	mu     sync.Mutex
	cache  = make(map[string]entry)
	ghPath string
	// disabled latches once we learn gh cannot answer at all (absent, or not
	// authenticated). Without it a broken auth setup logs a warning per instance
	// every ttl, forever.
	disabled bool
	lookOnce sync.Once
)

// Get returns the last known CI status for a session's worktree, triggering a
// background refresh when the cached verdict is older than ttl.
//
// Keyed on the worktree rather than a branch name, because the branch is resolved
// at refresh time from the worktree's HEAD (see refresh): a session that switched
// or renamed its branch must not keep reporting the old one's verdict, and it must
// not need a second cache entry to stop doing so.
//
// It never blocks on the network or on git. The caller is the UI's metadata tick,
// which also computes git diffs and only re-schedules once complete, so a slow gh
// call here would directly stall diff-stat updates. The first call for a worktree
// therefore returns the zero Status and the badge appears a tick or two later.
func Get(repoPath, worktreePath, fallbackBranch string) Status {
	if repoPath == "" || worktreePath == "" || !available() {
		return Status{}
	}

	key := repoPath + "\x00" + worktreePath

	mu.Lock()
	defer mu.Unlock()
	e := cache[key]
	if time.Since(e.status.FetchedAt) > ttl && !e.fetching {
		e.fetching = true
		cache[key] = e
		go refresh(key, repoPath, worktreePath, fallbackBranch)
	}
	return e.status
}

// refresh resolves the worktree's current branch and performs one lookup.
//
// The branch comes from HEAD rather than from the name the session was created
// with, so that renaming the branch or checking out another one inside the
// worktree is reflected. The recorded name is the fallback for a detached HEAD or
// a worktree that has gone away.
func refresh(key, repoPath, worktreePath, fallbackBranch string) {
	branch, err := git.CurrentBranch(worktreePath)
	if err != nil || branch == "" {
		branch = fallbackBranch
	}

	var status Status
	if branch == "" {
		err = errors.New("no branch to look up")
	} else {
		status, err = fetch(repoPath, branch)
	}

	mu.Lock()
	defer mu.Unlock()
	e := cache[key]
	e.fetching = false
	if err != nil {
		// Keep whatever verdict we had rather than blanking the badge on a
		// transient failure, but stamp the attempt so the next retry waits a
		// full ttl instead of firing on the next tick.
		e.status.FetchedAt = time.Now()
		cache[key] = e
		log.WarningLog.Printf("could not fetch CI status for branch %q: %v", branch, err)
		return
	}
	e.status = status
	cache[key] = e
}

// rollupEntry is the subset of a statusCheckRollup element that carries an
// outcome. A CheckRun (GitHub Actions and most apps) reports `conclusion` once
// finished and `status` while running; a legacy StatusContext — some deploy and
// coverage bots still post these — reports only `state`.
type rollupEntry struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
	State      string `json:"state"`
}

type prView struct {
	Number            int           `json:"number"`
	State             string        `json:"state"`
	StatusCheckRollup []rollupEntry `json:"statusCheckRollup"`
}

// fetch runs gh in repoPath and classifies the result.
func fetch(repoPath, branch string) (Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, ghPath, "pr", "view", branch, "--json", "number,state,statusCheckRollup")
	// Run inside the repo so gh resolves the remote itself; this also means a
	// fork or a non-github remote simply yields an error we swallow.
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		stderr := stderrOf(err)
		// gh exits non-zero for a branch with no pull request. That is a verdict,
		// not a failure — and the common case for a session whose work is not
		// pushed yet, so it must not be logged as an error every ttl.
		if strings.Contains(stderr, "no pull requests found") {
			return Status{State: StateNoPR, FetchedAt: time.Now()}, nil
		}
		// An auth problem is not per-branch and will not fix itself; stop asking.
		if strings.Contains(stderr, "gh auth login") {
			disable("gh is not authenticated")
			return Status{}, errors.New("gh is not authenticated")
		}
		return Status{}, fmt.Errorf("gh pr view: %w: %s", err, strings.TrimSpace(stderr))
	}

	var view prView
	if err := json.Unmarshal(out, &view); err != nil {
		return Status{}, fmt.Errorf("parsing gh output: %w", err)
	}

	if terminal := terminalState(view.State); terminal != StateUnknown {
		return Status{State: terminal, PRNumber: view.Number, FetchedAt: time.Now()}, nil
	}

	status := classify(view.StatusCheckRollup)
	status.PRNumber = view.Number
	status.FetchedAt = time.Now()
	return status, nil
}

// terminalState maps a pull request's own state to a verdict, for the states where
// the PR is the answer and its checks are not.
//
// Every check on a merged PR is green, so letting classify decide would render a
// merged branch identically to an open one that is simply passing -- and a
// closed-unmerged PR would likewise show a green tick for abandoned work. Returns
// StateUnknown for an open PR, meaning "let the checks decide".
func terminalState(prState string) State {
	switch strings.ToUpper(prState) {
	case "MERGED":
		return StateMerged
	case "CLOSED":
		return StateClosed
	default:
		return StateUnknown
	}
}

// classify reduces a status check rollup to a single verdict.
//
// SKIPPED and CANCELLED are ignored rather than counted as failures. A cancelled
// run is nearly always one superseded by a newer push to the same branch, and
// skipped jobs are the normal, healthy result of path filters — counting either
// as red would leave most branches permanently showing a failure.
func classify(entries []rollupEntry) Status {
	var s Status
	for _, e := range entries {
		switch outcomeOf(e) {
		case "SUCCESS", "NEUTRAL":
			s.Passed++
		case "FAILURE", "TIMED_OUT", "STARTUP_FAILURE", "ACTION_REQUIRED", "ERROR":
			s.Failed++
		case "SKIPPED", "CANCELLED", "STALE", "":
			// Carries no verdict — see the docblock.
		default:
			// QUEUED, IN_PROGRESS, PENDING, WAITING, REQUESTED, EXPECTED.
			s.Pending++
		}
	}

	// Deliberately ordered worst-first: one red check matters even while others
	// are still running, which is the whole point of glancing at the badge.
	switch {
	case s.Failed > 0:
		s.State = StateFailure
	case s.Pending > 0:
		s.State = StatePending
	case s.Passed > 0:
		s.State = StateSuccess
	default:
		s.State = StateUnknown
	}
	return s
}

// outcomeOf picks whichever field carries this entry's outcome.
func outcomeOf(e rollupEntry) string {
	if e.Conclusion != "" {
		return strings.ToUpper(e.Conclusion)
	}
	if e.Status != "" {
		return strings.ToUpper(e.Status)
	}
	return strings.ToUpper(e.State)
}

// available reports whether gh can be consulted. The PATH lookup happens once.
func available() bool {
	lookOnce.Do(func() {
		path, err := exec.LookPath("gh")
		if err != nil {
			disable("gh CLI not found on PATH")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		ghPath = path
	})

	mu.Lock()
	defer mu.Unlock()
	return !disabled && ghPath != ""
}

// disable latches the feature off for the rest of the process, logging why once.
func disable(reason string) {
	mu.Lock()
	defer mu.Unlock()
	if disabled {
		return
	}
	disabled = true
	log.WarningLog.Printf("GitHub CI status badges disabled: %s", reason)
}

func stderrOf(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(exitErr.Stderr)
	}
	return err.Error()
}
