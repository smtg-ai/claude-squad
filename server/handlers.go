package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	otelpkg "claude-squad/otel"
	"claude-squad/session"
	"claude-squad/session/git"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// register wires up all routes on the given mux.
func (s *Server) register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/health", s.handleHealth)
	mux.HandleFunc("/v1/events", s.handleEvents)
	mux.HandleFunc("/v1/instances", s.handleInstances)
	// Instance-scoped paths handled by a single dispatch function; Go's
	// stdlib mux doesn't do path parameters so we parse manually.
	mux.HandleFunc("/v1/instances/", s.handleInstanceScoped)
}

// -------- /v1/health --------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		OK:      true,
		Service: "claude-squad-server",
		Version: s.opts.Version,
	})
}

// -------- /v1/instances (list + create) --------

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleList(w, r)
	case http.MethodPost:
		s.handleCreate(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
	items := s.store.list()
	out := make([]*InstanceDTO, 0, len(items))
	for _, m := range items {
		out = append(out, toDTO(m))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title is required")
		return
	}

	basePath := req.WorkspaceBasePath
	if basePath == "" {
		cwd, err := filepath.Abs(".")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		basePath = cwd
	}
	if !git.IsGitRepo(basePath) {
		writeErr(w, http.StatusBadRequest,
			fmt.Sprintf("workspaceBasePath %q is not inside a git repo", basePath))
		return
	}

	program := req.Program
	if program == "" {
		program = "claude"
	}

	// Propagate inbound W3C trace context so the `cs.instance.*` spans
	// parent under whatever trace the caller started. When no
	// traceparent header is present this is a fresh root trace.
	ctx := propagation.TraceContext{}.Extract(r.Context(),
		propagation.HeaderCarrier(r.Header))
	ctx, span := s.tracer.Start(ctx, "cs.instance.create",
		trace.WithAttributes(
			attribute.String("cs.title", req.Title),
			attribute.String("cs.program", program),
			attribute.String("cs.branch", req.Branch),
			attribute.String("cs.workspace_base", basePath),
			attribute.Bool("cs.auto_yes", req.AutoYes),
			attribute.Bool("cs.has_prompt", req.Prompt != ""),
		))
	defer span.End()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   req.Title,
		Path:    basePath,
		Program: program,
		AutoYes: req.AutoYes,
		Branch:  req.Branch,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		writeErr(w, http.StatusBadRequest, "failed to init instance: "+err.Error())
		return
	}

	spanCtx := span.SpanContext()
	traceID, spanID := spanCtx.TraceID().String(), spanCtx.SpanID().String()

	// If OTEL is configured, inject the current span's W3C traceparent
	// + the full OTEL_* + CLAUDE_CODE_ENABLE_TELEMETRY bundle into the
	// tmux subprocess env. The agent emits its own spans as descendants
	// of cs.instance.create. Silent no-op when keys are unset.
	if s.opts.OtelCfg.PublicKey != "" {
		inst.SetSpawnEnv(otelpkg.SubprocessEnv(ctx, s.opts.OtelCfg,
			fmt.Sprintf("cs-agent-%s", shortID(spanID))))
	}

	// Start creates the worktree + tmux session.
	if err := inst.Start(true); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		writeErr(w, http.StatusInternalServerError, "failed to start instance: "+err.Error())
		return
	}

	// If the caller supplied an initial prompt, deliver it once tmux is
	// settled. Sends Enter after a short delay — matches the TUI's
	// existing behavior.
	if req.Prompt != "" {
		go func(prompt string, inst *session.Instance) {
			time.Sleep(800 * time.Millisecond)
			if err := inst.SendPrompt(prompt); err != nil {
				s.bus.publish("instance.prompt_send_failed", "",
					map[string]string{"error": err.Error()})
			}
		}(req.Prompt, inst)
	}

	m := s.store.add(inst, req.Prompt, traceID, spanID)
	s.bus.publish(EventInstanceCreated, m.ID, map[string]string{
		"title":   m.Instance.Title,
		"program": m.Instance.Program,
		"branch":  m.Instance.Branch,
		"traceId": traceID,
	})
	s.bus.publish(EventInstanceStarted, m.ID, map[string]string{
		"worktreePath": inst.GetWorktreePath(),
	})
	span.SetAttributes(
		attribute.String("cs.instance_id", m.ID),
		attribute.String("cs.worktree_path", inst.GetWorktreePath()),
	)

	writeJSON(w, http.StatusCreated, toDTO(m))
}

// shortID returns the first 8 chars of a hex span id for readable
// service.name suffixes on agent subprocesses.
func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// quickSpan wraps a small synchronous handler body in an OTEL span
// parented to whatever trace context the request carried. Used for
// pause/resume/kill/diff where span attribution is cheap.
func (s *Server) quickSpan(r *http.Request, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx := propagation.TraceContext{}.Extract(r.Context(),
		propagation.HeaderCarrier(r.Header))
	return s.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// -------- /v1/instances/:id/... (per-instance operations) --------

func (s *Server) handleInstanceScoped(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/instances/")
	if rest == "" {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]
	m, ok := s.store.get(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "instance not found")
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, toDTO(m))
		case http.MethodDelete:
			s.handleKill(w, r, m)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	switch parts[1] {
	case "pane":
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handlePane(w, r, m)
	case "input":
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleInput(w, r, m)
	case "pause":
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handlePause(w, r, m)
	case "resume":
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleResume(w, r, m)
	case "diff":
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleDiff(w, r, m)
	default:
		writeErr(w, http.StatusNotFound, "unknown subresource")
	}
}

func (s *Server) handlePane(w http.ResponseWriter, r *http.Request, m *managed) {
	withAnsi := r.URL.Query().Get("ansi") != "false" // default true
	var content string
	var err error
	if r.URL.Query().Get("full") == "true" {
		content, err = m.Instance.PreviewFullHistory()
	} else {
		content, err = m.Instance.Preview()
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !withAnsi {
		content = stripAnsi(content)
	}
	writeJSON(w, http.StatusOK, PaneResponse{
		ID:       m.ID,
		Content:  content,
		WithAnsi: withAnsi,
	})
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request, m *managed) {
	var req InputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	var err error
	switch {
	case req.Prompt != "":
		err = m.Instance.SendPrompt(req.Prompt)
	case req.Keys != "":
		err = m.Instance.SendKeys(req.Keys)
	case req.TapEnter:
		m.Instance.TapEnter()
	default:
		writeErr(w, http.StatusBadRequest,
			"supply one of: prompt, keys, tapEnter=true")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request, m *managed) {
	_, span := s.quickSpan(r, "cs.instance.pause",
		attribute.String("cs.instance_id", m.ID))
	defer span.End()
	if err := m.Instance.Pause(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.bus.publish(EventInstancePaused, m.ID, nil)
	writeJSON(w, http.StatusOK, toDTO(m))
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request, m *managed) {
	_, span := s.quickSpan(r, "cs.instance.resume",
		attribute.String("cs.instance_id", m.ID))
	defer span.End()
	if err := m.Instance.Resume(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.bus.publish(EventInstanceResumed, m.ID, nil)
	writeJSON(w, http.StatusOK, toDTO(m))
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request, m *managed) {
	_, span := s.quickSpan(r, "cs.instance.kill",
		attribute.String("cs.instance_id", m.ID))
	defer span.End()
	if err := m.Instance.Kill(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.store.remove(m.ID)
	s.bus.publish(EventInstanceKilled, m.ID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDiff(w http.ResponseWriter, _ *http.Request, m *managed) {
	stats := m.Instance.ComputeDiff()
	if stats == nil {
		writeErr(w, http.StatusInternalServerError, "diff unavailable")
		return
	}
	m.Instance.SetDiffStats(stats)
	s.bus.publish(EventInstanceDiffUpdated, m.ID, map[string]string{
		"added":   strconv.Itoa(stats.Added),
		"removed": strconv.Itoa(stats.Removed),
	})
	writeJSON(w, http.StatusOK, DiffResponse{
		ID:      m.ID,
		Added:   stats.Added,
		Removed: stats.Removed,
		Content: stats.Content,
	})
}

// -------- /v1/events (SSE) --------

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	since := int64(0)
	if q := r.URL.Query().Get("since"); q != "" {
		if n, err := strconv.ParseInt(q, 10, 64); err == nil {
			since = n
		}
	}

	ch, unsub := s.bus.subscribe(since, 64)
	defer unsub()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n",
				e.Seq, e.Type, payload); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// -------- helpers --------

// extractTraceHeaders peels W3C traceparent out of the inbound request so
// the DTO can report back what trace the caller started us under. When
// the OTEL layer (commit 2) is active, it also uses these for span
// propagation — but the bare HTTP server works without it.
func extractTraceHeaders(r *http.Request) (traceID, spanID string) {
	tp := r.Header.Get("Traceparent")
	if tp == "" {
		return "", ""
	}
	parts := strings.Split(tp, "-")
	if len(parts) >= 3 {
		return parts[1], parts[2]
	}
	return "", ""
}

// stripAnsi removes ANSI escape sequences from a string. Matches most
// common CSI and OSC sequences; not perfect but good enough for a
// plain-text pane view requested via ?ansi=false.
func stripAnsi(s string) string {
	// Minimal hand-rolled to avoid adding a regexp dependency in the
	// hot path; collapses ESC[ ... final_byte and ESC] ... BEL.
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != 0x1B {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(s) {
			break
		}
		next := s[i+1]
		if next == '[' {
			i += 2
			for i < len(s) {
				b2 := s[i]
				if (b2 >= 0x40 && b2 <= 0x7E) || b2 == '~' {
					break
				}
				i++
			}
			continue
		}
		if next == ']' {
			i += 2
			for i < len(s) && s[i] != 0x07 {
				i++
			}
			continue
		}
		// unknown ESC sequence; skip the next char
		i++
	}
	return b.String()
}
