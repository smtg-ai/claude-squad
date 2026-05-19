package server

import (
	"claude-squad/session"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// managed wraps a session.Instance with the extra metadata the server
// tracks per session — a UUID the HTTP clients use, an originating
// trace id, and an initial prompt.
type managed struct {
	ID            string
	Instance      *session.Instance
	TraceID       string
	ParentSpanID  string
	InitialPrompt string
	CreatedAt     time.Time
}

// store is a concurrency-safe in-memory registry of active sessions.
// Intentionally not persisted to disk — the caller (Paperclip) is the
// authoritative source of which sessions should exist. On cs-server
// restart the caller's harness sees SSE disconnects and recreates any
// sessions it still needs.
type store struct {
	mu    sync.RWMutex
	items map[string]*managed
}

func newStore() *store {
	return &store{items: make(map[string]*managed)}
}

func (s *store) add(inst *session.Instance, prompt, traceID, spanID string) *managed {
	id := uuid.NewString()
	s.mu.Lock()
	defer s.mu.Unlock()
	m := &managed{
		ID:            id,
		Instance:      inst,
		TraceID:       traceID,
		ParentSpanID:  spanID,
		InitialPrompt: prompt,
		CreatedAt:     time.Now(),
	}
	s.items[id] = m
	return m
}

func (s *store) get(id string) (*managed, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.items[id]
	return m, ok
}

func (s *store) list() []*managed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*managed, 0, len(s.items))
	for _, m := range s.items {
		out = append(out, m)
	}
	return out
}

func (s *store) remove(id string) (*managed, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.items[id]
	if ok {
		delete(s.items, id)
	}
	return m, ok
}

// toDTO renders a managed session to its JSON form.
func toDTO(m *managed) *InstanceDTO {
	inst := m.Instance
	dto := &InstanceDTO{
		ID:            m.ID,
		Title:         inst.Title,
		Status:        statusString(inst.Status),
		Program:       inst.Program,
		Branch:        inst.Branch,
		AutoYes:       inst.AutoYes,
		CreatedAt:     inst.CreatedAt,
		UpdatedAt:     inst.UpdatedAt,
		Paused:        inst.Paused(),
		TmuxAlive:     inst.TmuxAlive(),
		TraceID:       m.TraceID,
		ParentSpanID:  m.ParentSpanID,
		InitialPrompt: m.InitialPrompt,
	}
	if wt, err := inst.GetGitWorktree(); err == nil && wt != nil {
		dto.RepoPath = wt.GetRepoPath()
		dto.WorktreePath = wt.GetWorktreePath()
		dto.Branch = wt.GetBranchName()
	}
	if stats := inst.GetDiffStats(); stats != nil {
		dto.DiffAdded = stats.Added
		dto.DiffRemoved = stats.Removed
	}
	return dto
}

func statusString(s session.Status) string {
	switch s {
	case session.Running:
		return "running"
	case session.Ready:
		return "ready"
	case session.Loading:
		return "loading"
	case session.Paused:
		return "paused"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}
