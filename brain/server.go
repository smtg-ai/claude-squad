package brain

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"sync"
	"time"
)

const actionTimeout = 30 * time.Second

// Server listens on a Unix domain socket and dispatches requests to a Manager.
type Server struct {
	manager    *Manager
	eventBus   *EventBus
	listener   net.Listener
	socketPath string

	// actionCh relays Tier 3 requests (create_instance, inject_message, etc.)
	// from socket handlers to the TUI process. Buffered to avoid blocking the
	// accept loop if the TUI is briefly busy.
	actionCh chan ActionRequest

	wg     sync.WaitGroup
	closed chan struct{}
}

// NewServer creates a new brain server. Call Start() to begin listening.
func NewServer(socketPath string) *Server {
	s := &Server{
		manager:    NewManager(),
		eventBus:   NewEventBus(1000),
		socketPath: socketPath,
		actionCh:   make(chan ActionRequest, 16),
		closed:     make(chan struct{}),
	}
	s.manager.SetEventCallback(func(e Event) {
		s.eventBus.Emit(e)
	})
	return s
}

// Actions returns the channel for receiving action requests from agents.
// The TUI reads from this channel to process Tier 3 operations.
func (s *Server) Actions() <-chan ActionRequest {
	return s.actionCh
}

// Start begins listening on the Unix socket. It blocks in an accept loop
// until Stop() is called.
func (s *Server) Start() error {
	// Remove stale socket file from a previous run.
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	// Restrict socket permissions to owner only.
	os.Chmod(s.socketPath, 0600)

	s.listener = ln

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop()
	}()

	// Prune stale event subscribers periodically.
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.eventBus.PruneStale(5 * time.Minute)
			case <-s.closed:
				return
			}
		}
	}()

	return nil
}

// Stop closes the listener, waits for in-flight connections to finish,
// and removes the socket file.
func (s *Server) Stop() error {
	close(s.closed)
	err := s.listener.Close()
	s.wg.Wait()
	os.Remove(s.socketPath)
	return err
}

// SocketPath returns the path the server is listening on.
func (s *Server) SocketPath() string {
	return s.socketPath
}

// Manager returns the underlying state manager for direct in-process access.
func (s *Server) Manager() *Manager {
	return s.manager
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
			}
			// Transient error — keep accepting.
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	// Allow up to 35s for long-poll requests (25s max poll + overhead).
	conn.SetDeadline(time.Now().Add(35 * time.Second))

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max message
	if !scanner.Scan() {
		return
	}

	var req Request
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		writeResponse(conn, Response{Error: "invalid request: " + err.Error()})
		return
	}

	resp := s.dispatch(req)
	writeResponse(conn, resp)
}

func (s *Server) dispatch(req Request) Response {
	switch req.Method {
	case MethodPing:
		return Response{OK: true}

	case MethodGetBrain:
		state := s.manager.GetBrain(req.RepoPath, req.InstanceID)
		data, err := json.Marshal(state)
		if err != nil {
			return Response{Error: "marshal error: " + err.Error()}
		}
		return Response{OK: true, Data: data}

	case MethodUpdateStatus:
		feature, _ := req.Params["feature"].(string)
		files := toStringSlice(req.Params["files"])
		role, _ := req.Params["role"].(string)
		var result *UpdateStatusResult
		if role != "" {
			result = s.manager.UpdateStatusWithRole(req.RepoPath, req.InstanceID, feature, files, role)
		} else {
			result = s.manager.UpdateStatus(req.RepoPath, req.InstanceID, feature, files)
		}
		data, err := json.Marshal(result)
		if err != nil {
			return Response{Error: "marshal error: " + err.Error()}
		}
		return Response{OK: true, Data: data}

	case MethodSendMessage:
		to, _ := req.Params["to"].(string)
		content, _ := req.Params["content"].(string)
		s.manager.SendMessage(req.RepoPath, req.InstanceID, to, content)
		return Response{OK: true}

	case MethodRemoveAgent:
		s.manager.RemoveAgent(req.RepoPath, req.InstanceID)
		return Response{OK: true}

	// Tier 3: actions relayed to TUI via action channel.
	case MethodCreateInstance:
		// Forward the requesting agent's ID so the TUI can inherit topic.
		params := req.Params
		if params == nil {
			params = make(map[string]any)
		}
		params["source_instance"] = req.InstanceID
		return s.sendAction(ActionCreateInstance, params)

	case MethodInjectMessage:
		// Also store the message in brain state for visibility in get_brain.
		to, _ := req.Params["to"].(string)
		content, _ := req.Params["content"].(string)
		if content != "" {
			s.manager.SendMessage(req.RepoPath, req.InstanceID, to, content)
		}
		params := req.Params
		if params == nil {
			params = make(map[string]any)
		}
		params["from"] = req.InstanceID
		return s.sendAction(ActionInjectMessage, params)

	case MethodPauseInstance:
		return s.sendAction(ActionPauseInstance, req.Params)

	case MethodResumeInstance:
		return s.sendAction(ActionResumeInstance, req.Params)

	case MethodKillInstance:
		return s.sendAction(ActionKillInstance, req.Params)

	case MethodDefineWorkflow:
		return s.dispatchDefineWorkflow(req)

	case MethodCompleteTask:
		return s.dispatchCompleteTask(req)

	case MethodGetWorkflow:
		return s.dispatchGetWorkflow(req)

	case MethodSubscribe:
		return s.handleSubscribe(req)

	case MethodPollEvents:
		return s.handlePollEvents(req)

	case MethodUnsubscribe:
		return s.handleUnsubscribe(req)

	default:
		return Response{Error: "unknown method: " + req.Method}
	}
}

// sendAction relays a request to the TUI via the action channel and blocks until
// the TUI responds or the timeout expires.
func (s *Server) sendAction(actionType ActionType, params map[string]any) Response {
	respCh := make(chan ActionResponse, 1)
	action := ActionRequest{
		Type:       actionType,
		Params:     params,
		ResponseCh: respCh,
	}

	select {
	case s.actionCh <- action:
	case <-s.closed:
		return Response{Error: "server shutting down"}
	case <-time.After(actionTimeout):
		return Response{Error: "action channel full, TUI not responding"}
	}

	select {
	case resp := <-respCh:
		if !resp.OK {
			return Response{Error: resp.Error}
		}
		if resp.Data != nil {
			data, err := json.Marshal(resp.Data)
			if err != nil {
				return Response{Error: "marshal action response: " + err.Error()}
			}
			return Response{OK: true, Data: data}
		}
		return Response{OK: true}
	case <-s.closed:
		return Response{Error: "server shutting down"}
	case <-time.After(actionTimeout):
		return Response{Error: "TUI did not respond in time"}
	}
}

// dispatchDefineWorkflow handles the define_workflow method.
func (s *Server) dispatchDefineWorkflow(req Request) Response {
	tasksRaw, ok := req.Params["tasks"].([]any)
	if !ok {
		return Response{Error: "missing or invalid 'tasks' parameter"}
	}

	var tasks []*WorkflowTask
	for _, raw := range tasksRaw {
		taskMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		task := &WorkflowTask{
			ID:        toString(taskMap["id"]),
			Title:     toString(taskMap["title"]),
			Status:    TaskPending,
			DependsOn: toStringSlice(taskMap["depends_on"]),
			Prompt:    toString(taskMap["prompt"]),
			Role:      toString(taskMap["role"]),
		}
		tasks = append(tasks, task)
	}

	if len(tasks) == 0 {
		return Response{Error: "no valid tasks provided"}
	}

	result := s.manager.DefineWorkflow(req.RepoPath, tasks)

	// Auto-trigger tasks with no dependencies.
	triggered := s.manager.EvaluateWorkflow(req.RepoPath)
	result.Triggered = triggered

	// Spawn instances for triggered tasks.
	for _, taskID := range triggered {
		task := s.manager.GetWorkflowTask(req.RepoPath, taskID)
		if task == nil {
			continue
		}
		s.sendAction(ActionCreateInstance, map[string]any{
			"title":   task.ID,
			"prompt":  task.Prompt,
			"role":    task.Role,
			"_from_workflow": true,
		})
	}

	data, err := json.Marshal(result)
	if err != nil {
		return Response{Error: "marshal error: " + err.Error()}
	}
	return Response{OK: true, Data: data}
}

// dispatchCompleteTask handles the complete_task method.
func (s *Server) dispatchCompleteTask(req Request) Response {
	taskID, _ := req.Params["task_id"].(string)
	status, _ := req.Params["status"].(string)
	errMsg, _ := req.Params["error"].(string)

	if taskID == "" {
		return Response{Error: "missing required parameter: task_id"}
	}

	var taskStatus TaskStatus
	switch status {
	case "done", "":
		taskStatus = TaskDone
	case "failed":
		taskStatus = TaskFailed
	default:
		return Response{Error: "invalid status: " + status + " (must be 'done' or 'failed')"}
	}

	if err := s.manager.CompleteTask(req.RepoPath, taskID, taskStatus, errMsg); err != nil {
		return Response{Error: err.Error()}
	}

	// Evaluate DAG for newly unblocked tasks.
	triggered := s.manager.EvaluateWorkflow(req.RepoPath)

	// Spawn instances for triggered tasks.
	for _, tid := range triggered {
		task := s.manager.GetWorkflowTask(req.RepoPath, tid)
		if task == nil {
			continue
		}
		s.sendAction(ActionCreateInstance, map[string]any{
			"title":   task.ID,
			"prompt":  task.Prompt,
			"role":    task.Role,
			"_from_workflow": true,
		})
	}

	result := WorkflowResult{Triggered: triggered}
	data, err := json.Marshal(result)
	if err != nil {
		return Response{Error: "marshal error: " + err.Error()}
	}
	return Response{OK: true, Data: data}
}

// dispatchGetWorkflow handles the get_workflow method.
func (s *Server) dispatchGetWorkflow(req Request) Response {
	workflow := s.manager.GetWorkflow(req.RepoPath)
	if workflow == nil {
		return Response{OK: true, Data: json.RawMessage(`{"tasks":[]}`)}
	}
	data, err := json.Marshal(workflow)
	if err != nil {
		return Response{Error: "marshal error: " + err.Error()}
	}
	return Response{OK: true, Data: data}
}

// PushEvent emits an event into the event bus (used by TUI for instance lifecycle events).
func (s *Server) PushEvent(event Event) {
	s.eventBus.Emit(event)
}

func (s *Server) handleSubscribe(req Request) Response {
	var filter EventFilter
	if types := toStringSlice(req.Params["types"]); len(types) > 0 {
		for _, t := range types {
			filter.Types = append(filter.Types, EventType(t))
		}
	}
	filter.Instances = toStringSlice(req.Params["instances"])
	if pt, ok := req.Params["parent_title"].(string); ok {
		filter.ParentTitle = pt
	}

	subID := s.eventBus.Subscribe(filter)

	result := SubscribeResult{SubscriberID: subID}
	data, err := json.Marshal(result)
	if err != nil {
		return Response{Error: "marshal error: " + err.Error()}
	}
	return Response{OK: true, Data: data}
}

func (s *Server) handlePollEvents(req Request) Response {
	subID, _ := req.Params["subscriber_id"].(string)
	if subID == "" {
		return Response{Error: "missing required parameter: subscriber_id"}
	}

	timeoutSec := 15
	if v, ok := req.Params["timeout"].(float64); ok {
		timeoutSec = clamp(int(v), 1, 25)
	}

	events, err := s.eventBus.Poll(subID, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return Response{Error: err.Error()}
	}

	result := PollEventsResult{
		SubscriberID: subID,
		Events:       events,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return Response{Error: "marshal error: " + err.Error()}
	}
	return Response{OK: true, Data: data}
}

func (s *Server) handleUnsubscribe(req Request) Response {
	subID, _ := req.Params["subscriber_id"].(string)
	if subID == "" {
		return Response{Error: "missing required parameter: subscriber_id"}
	}
	s.eventBus.Unsubscribe(subID)
	return Response{OK: true}
}

// clamp constrains v to the range [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// toString safely extracts a string from an any value.
func toString(v any) string {
	s, _ := v.(string)
	return s
}

// toStringSlice extracts a []string from a JSON-decoded []any value.
// Returns nil if the value is not a []any or is nil.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, elem := range arr {
		if str, ok := elem.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	conn.Write(data)
}
