package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Task represents an agent task in the knowledge graph
type Task struct {
	ID           string            `json:"id"`
	AgentID      string            `json:"agent_id"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	Priority     int               `json:"priority"`
	Dependencies []string          `json:"dependencies"`
	CreatedAt    string            `json:"created_at"`
	StartedAt    *string           `json:"started_at,omitempty"`
	CompletedAt  *string           `json:"completed_at,omitempty"`
	Result       *string           `json:"result,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TaskInfo represents minimal task information
type TaskInfo struct {
	ID          string `json:"id"`
	URI         string `json:"uri"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
}

// Analytics represents task execution analytics
type Analytics struct {
	StatusCounts   map[string]int `json:"status_counts"`
	TotalTasks     int            `json:"total_tasks"`
	RunningCount   int            `json:"running_count"`
	MaxConcurrent  int            `json:"max_concurrent"`
	AvailableSlots int            `json:"available_slots"`
}

// DependencyChain represents a task's dependency chain
type DependencyChain struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// Client is the HTTP client for the Oxigraph orchestrator service
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new orchestrator client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Health checks if the orchestrator service is healthy
func (c *Client) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// CreateTask creates a new task in the knowledge graph
func (c *Client) CreateTask(task *Task) (string, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("failed to marshal task: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/tasks",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create task returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		TaskID  string `json:"task_id"`
		TaskURI string `json:"task_uri"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.TaskID, nil
}

// UpdateTaskStatus updates the status of a task
func (c *Client) UpdateTaskStatus(taskID, status string, result *string) error {
	data := map[string]interface{}{
		"status": status,
	}
	if result != nil {
		data["result"] = *result
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/tasks/%s/status", c.baseURL, taskID),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update status returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetReadyTasks retrieves tasks ready for execution
func (c *Client) GetReadyTasks(limit int) ([]TaskInfo, error) {
	url := fmt.Sprintf("%s/tasks/ready?limit=%d", c.baseURL, limit)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get ready tasks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get ready tasks returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Tasks []TaskInfo `json:"tasks"`
		Count int        `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Tasks, nil
}

// GetRunningTasks retrieves currently running tasks
func (c *Client) GetRunningTasks() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/tasks/running")
	if err != nil {
		return nil, fmt.Errorf("failed to get running tasks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get running tasks returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Tasks []string `json:"tasks"`
		Count int      `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Tasks, nil
}

// GetAnalytics retrieves task execution analytics
func (c *Client) GetAnalytics() (*Analytics, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/analytics")
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get analytics returned status %d: %s", resp.StatusCode, string(body))
	}

	var analytics Analytics
	if err := json.NewDecoder(resp.Body).Decode(&analytics); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &analytics, nil
}

// GetTaskChain retrieves the dependency chain for a task
func (c *Client) GetTaskChain(taskID string) ([]DependencyChain, error) {
	url := fmt.Sprintf("%s/tasks/%s/chain", c.baseURL, taskID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get task chain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get task chain returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Chain []DependencyChain `json:"chain"`
		Count int               `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Chain, nil
}

// OptimizeDistribution gets optimized task distribution recommendations
func (c *Client) OptimizeDistribution() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/optimize")
	if err != nil {
		return nil, fmt.Errorf("failed to optimize distribution: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("optimize distribution returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Tasks []string `json:"tasks"`
		Count int      `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Tasks, nil
}
