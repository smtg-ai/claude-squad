package brain

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const dialTimeout = 2 * time.Second

// Client connects to a brain server over a Unix domain socket.
type Client struct {
	socketPath string
}

// NewClient creates a new socket client.
func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// Ping checks connectivity to the brain server.
func (c *Client) Ping() error {
	_, err := c.send(Request{Method: MethodPing})
	return err
}

// GetBrain retrieves the coordination state for a repo, filtered for the requesting agent.
func (c *Client) GetBrain(repoPath, instanceID string) (*BrainState, error) {
	resp, err := c.send(Request{
		Method:     MethodGetBrain,
		InstanceID: instanceID,
		RepoPath:   repoPath,
	})
	if err != nil {
		return nil, err
	}

	var state BrainState
	if err := json.Unmarshal(resp.Data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal brain state: %w", err)
	}
	return &state, nil
}

// UpdateStatus declares the agent's current feature and files.
func (c *Client) UpdateStatus(repoPath, instanceID, feature string, files []string) (*UpdateStatusResult, error) {
	return c.UpdateStatusWithRole(repoPath, instanceID, feature, files, "")
}

// UpdateStatusWithRole declares the agent's current feature, files, and role.
func (c *Client) UpdateStatusWithRole(repoPath, instanceID, feature string, files []string, role string) (*UpdateStatusResult, error) {
	params := map[string]any{
		"feature": feature,
		"files":   files,
	}
	if role != "" {
		params["role"] = role
	}
	resp, err := c.send(Request{
		Method:     MethodUpdateStatus,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params:     params,
	})
	if err != nil {
		return nil, err
	}

	var result UpdateStatusResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal update result: %w", err)
	}
	return &result, nil
}

// SendMessage sends a message from one agent to another (or broadcast if to is empty).
func (c *Client) SendMessage(repoPath, from, to, content string) error {
	_, err := c.send(Request{
		Method:     MethodSendMessage,
		InstanceID: from,
		RepoPath:   repoPath,
		Params: map[string]any{
			"to":      to,
			"content": content,
		},
	})
	return err
}

// RemoveAgent removes an agent from the brain state.
func (c *Client) RemoveAgent(repoPath, instanceID string) error {
	_, err := c.send(Request{
		Method:     MethodRemoveAgent,
		InstanceID: instanceID,
		RepoPath:   repoPath,
	})
	return err
}

// CreateInstance requests the TUI to spawn a new agent instance.
func (c *Client) CreateInstance(repoPath, instanceID string, params CreateInstanceParams) (*CreateInstanceResult, error) {
	p := map[string]any{
		"title":   params.Title,
		"program": params.Program,
		"prompt":  params.Prompt,
		"role":    params.Role,
		"topic":   params.Topic,
	}
	if params.SkipPermissions != nil {
		p["skip_permissions"] = *params.SkipPermissions
	}
	resp, err := c.send(Request{
		Method:     MethodCreateInstance,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params:     p,
	})
	if err != nil {
		return nil, err
	}

	var result CreateInstanceResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal create result: %w", err)
	}
	return &result, nil
}

// InjectMessage requests the TUI to inject text directly into an agent's terminal.
func (c *Client) InjectMessage(repoPath, instanceID string, params InjectMessageParams) error {
	_, err := c.send(Request{
		Method:     MethodInjectMessage,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params: map[string]any{
			"to":      params.To,
			"content": params.Content,
			"format":  params.Format,
		},
	})
	return err
}

// PauseInstance requests the TUI to pause an agent instance.
func (c *Client) PauseInstance(repoPath, instanceID, target string) error {
	_, err := c.send(Request{
		Method:     MethodPauseInstance,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params:     map[string]any{"target": target},
	})
	return err
}

// ResumeInstance requests the TUI to resume a paused agent instance.
func (c *Client) ResumeInstance(repoPath, instanceID, target string) error {
	_, err := c.send(Request{
		Method:     MethodResumeInstance,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params:     map[string]any{"target": target},
	})
	return err
}

// KillInstance requests the TUI to terminate an agent instance.
func (c *Client) KillInstance(repoPath, instanceID, target string) error {
	_, err := c.send(Request{
		Method:     MethodKillInstance,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params:     map[string]any{"target": target},
	})
	return err
}

// DefineWorkflow creates a workflow DAG for a repo.
func (c *Client) DefineWorkflow(repoPath, instanceID string, tasks []*WorkflowTask) (*WorkflowResult, error) {
	resp, err := c.send(Request{
		Method:     MethodDefineWorkflow,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params:     map[string]any{"tasks": tasks},
	})
	if err != nil {
		return nil, err
	}

	var result WorkflowResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal workflow result: %w", err)
	}
	return &result, nil
}

// CompleteTask marks a workflow task as done or failed.
func (c *Client) CompleteTask(repoPath, instanceID, taskID, status, errMsg string) (*WorkflowResult, error) {
	resp, err := c.send(Request{
		Method:     MethodCompleteTask,
		InstanceID: instanceID,
		RepoPath:   repoPath,
		Params: map[string]any{
			"task_id": taskID,
			"status":  status,
			"error":   errMsg,
		},
	})
	if err != nil {
		return nil, err
	}

	var result WorkflowResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal workflow result: %w", err)
	}
	return &result, nil
}

// GetWorkflow retrieves the current workflow DAG for a repo.
func (c *Client) GetWorkflow(repoPath, instanceID string) (*Workflow, error) {
	resp, err := c.send(Request{
		Method:     MethodGetWorkflow,
		InstanceID: instanceID,
		RepoPath:   repoPath,
	})
	if err != nil {
		return nil, err
	}

	var workflow Workflow
	if err := json.Unmarshal(resp.Data, &workflow); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}
	return &workflow, nil
}

// Subscribe creates an event subscription with the given filter.
func (c *Client) Subscribe(repoPath string, filter EventFilter) (string, error) {
	params := make(map[string]any)
	if len(filter.Types) > 0 {
		params["types"] = toAnySlice(filter.Types)
	}
	if len(filter.Instances) > 0 {
		params["instances"] = toAnySlice(filter.Instances)
	}
	if filter.ParentTitle != "" {
		params["parent_title"] = filter.ParentTitle
	}

	resp, err := c.send(Request{
		Method:   MethodSubscribe,
		RepoPath: repoPath,
		Params:   params,
	})
	if err != nil {
		return "", err
	}

	var result SubscribeResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", fmt.Errorf("unmarshal subscribe result: %w", err)
	}
	return result.SubscriberID, nil
}

// toAnySlice converts a typed slice to []any for JSON-based IPC params.
func toAnySlice[T ~string](src []T) []any {
	out := make([]any, len(src))
	for i, v := range src {
		out[i] = string(v)
	}
	return out
}

// PollEvents long-polls for events on the given subscription.
func (c *Client) PollEvents(subscriberID string, timeoutSec int) ([]Event, error) {
	resp, err := c.sendWithTimeout(Request{
		Method: MethodPollEvents,
		Params: map[string]any{
			"subscriber_id": subscriberID,
			"timeout":       timeoutSec,
		},
	}, time.Duration(timeoutSec+5)*time.Second)
	if err != nil {
		return nil, err
	}

	var result PollEventsResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal poll result: %w", err)
	}
	return result.Events, nil
}

// Unsubscribe removes an event subscription.
func (c *Client) Unsubscribe(subscriberID string) error {
	_, err := c.send(Request{
		Method: MethodUnsubscribe,
		Params: map[string]any{
			"subscriber_id": subscriberID,
		},
	})
	return err
}

// send dials the socket, sends a request, reads the response, and closes the connection.
func (c *Client) send(req Request) (*Response, error) {
	return c.sendWithTimeout(req, dialTimeout)
}

// sendWithTimeout is like send but with a custom dial and read deadline.
func (c *Client) sendWithTimeout(req Request, timeout time.Duration) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("connect to brain server: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return nil, fmt.Errorf("no response from brain server")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("brain server error: %s", resp.Error)
	}
	return &resp, nil
}
