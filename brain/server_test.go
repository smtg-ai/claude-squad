package brain

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func startTestServer(t *testing.T) *Server {
	t.Helper()
	// Use /tmp directly instead of t.TempDir() to keep the socket path under
	// macOS's 104-byte limit for Unix sockets. Long test names can push
	// t.TempDir() paths past this limit.
	tmpDir, err := os.MkdirTemp("/tmp", "brain-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	sock := filepath.Join(tmpDir, "t.sock")
	s := NewServer(sock)
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	t.Cleanup(func() { s.Stop() })
	return s
}

func roundTrip(t *testing.T, socketPath string, req Request) Response {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	data, _ := json.Marshal(req)
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		t.Fatal("no response from server")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func TestServerPing(t *testing.T) {
	s := startTestServer(t)
	resp := roundTrip(t, s.SocketPath(), Request{Method: MethodPing})
	if !resp.OK {
		t.Errorf("ping: OK = false, error = %q", resp.Error)
	}
}

func TestServerUpdateAndGetBrain(t *testing.T) {
	s := startTestServer(t)

	// Update status for agent-1
	resp := roundTrip(t, s.SocketPath(), Request{
		Method:     MethodUpdateStatus,
		InstanceID: "agent-1",
		RepoPath:   "/repo",
		Params: map[string]any{
			"feature": "auth",
			"files":   []any{"auth.go", "middleware.go"},
		},
	})
	if !resp.OK {
		t.Fatalf("update_status: %s", resp.Error)
	}

	// Get brain for agent-1
	resp = roundTrip(t, s.SocketPath(), Request{
		Method:     MethodGetBrain,
		InstanceID: "agent-1",
		RepoPath:   "/repo",
	})
	if !resp.OK {
		t.Fatalf("get_brain: %s", resp.Error)
	}

	var state BrainState
	if err := json.Unmarshal(resp.Data, &state); err != nil {
		t.Fatalf("unmarshal brain state: %v", err)
	}
	if len(state.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(state.Agents))
	}
	if state.Agents["agent-1"].Feature != "auth" {
		t.Errorf("Feature = %q, want %q", state.Agents["agent-1"].Feature, "auth")
	}
}

func TestServerSendMessage(t *testing.T) {
	s := startTestServer(t)

	resp := roundTrip(t, s.SocketPath(), Request{
		Method:     MethodSendMessage,
		InstanceID: "agent-1",
		RepoPath:   "/repo",
		Params: map[string]any{
			"to":      "agent-2",
			"content": "don't touch auth.go",
		},
	})
	if !resp.OK {
		t.Fatalf("send_message: %s", resp.Error)
	}

	// Verify agent-2 can see the message
	resp = roundTrip(t, s.SocketPath(), Request{
		Method:     MethodGetBrain,
		InstanceID: "agent-2",
		RepoPath:   "/repo",
	})
	if !resp.OK {
		t.Fatalf("get_brain: %s", resp.Error)
	}

	var state BrainState
	json.Unmarshal(resp.Data, &state)
	if len(state.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "don't touch auth.go" {
		t.Errorf("Content = %q, want %q", state.Messages[0].Content, "don't touch auth.go")
	}
}

func TestServerRemoveAgent(t *testing.T) {
	s := startTestServer(t)

	// Add two agents
	roundTrip(t, s.SocketPath(), Request{
		Method: MethodUpdateStatus, InstanceID: "agent-1", RepoPath: "/repo",
		Params: map[string]any{"feature": "a"},
	})
	roundTrip(t, s.SocketPath(), Request{
		Method: MethodUpdateStatus, InstanceID: "agent-2", RepoPath: "/repo",
		Params: map[string]any{"feature": "b"},
	})

	// Remove agent-1
	resp := roundTrip(t, s.SocketPath(), Request{
		Method: MethodRemoveAgent, InstanceID: "agent-1", RepoPath: "/repo",
	})
	if !resp.OK {
		t.Fatalf("remove_agent: %s", resp.Error)
	}

	// Verify only agent-2 remains
	resp = roundTrip(t, s.SocketPath(), Request{
		Method: MethodGetBrain, InstanceID: "agent-2", RepoPath: "/repo",
	})
	var state BrainState
	json.Unmarshal(resp.Data, &state)
	if len(state.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(state.Agents))
	}
	if _, ok := state.Agents["agent-2"]; !ok {
		t.Error("expected agent-2 to remain")
	}
}

func TestServerUnknownMethod(t *testing.T) {
	s := startTestServer(t)
	resp := roundTrip(t, s.SocketPath(), Request{Method: "nonexistent"})
	if resp.OK {
		t.Error("expected OK=false for unknown method")
	}
	if resp.Error == "" {
		t.Error("expected error message for unknown method")
	}
}

func TestServerConflictDetection(t *testing.T) {
	s := startTestServer(t)

	// agent-1 claims auth.go
	roundTrip(t, s.SocketPath(), Request{
		Method: MethodUpdateStatus, InstanceID: "agent-1", RepoPath: "/repo",
		Params: map[string]any{"feature": "auth", "files": []any{"auth.go"}},
	})

	// agent-2 also claims auth.go
	resp := roundTrip(t, s.SocketPath(), Request{
		Method: MethodUpdateStatus, InstanceID: "agent-2", RepoPath: "/repo",
		Params: map[string]any{"feature": "auth fix", "files": []any{"auth.go"}},
	})
	if !resp.OK {
		t.Fatalf("update_status: %s", resp.Error)
	}

	var result UpdateStatusResult
	json.Unmarshal(resp.Data, &result)
	if len(result.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(result.Conflicts))
	}
}
