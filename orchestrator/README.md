# Oxigraph-Powered Concurrent Agent Orchestrator

Advanced concurrent agent orchestration system for Claude Squad using Oxigraph knowledge graphs for semantic task management.

## 🚀 Features

- **Maximum 10 Concurrent Agents**: Intelligent pool management with configurable concurrency
- **Semantic Task Tracking**: Oxigraph RDF knowledge graph for rich task relationships
- **Dependency Resolution**: Automatic task ordering based on dependencies
- **Intelligent Load Balancing**: Optimized task distribution across available agents
- **Real-time Analytics**: Comprehensive monitoring and performance metrics
- **Task Chaining**: Complex dependency graphs with automatic resolution
- **Graceful Degradation**: Robust error handling and recovery
- **RESTful API**: Complete HTTP API for integration
- **TUI Dashboard**: Beautiful terminal dashboard for monitoring

## 📋 Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Claude Squad Application              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │   Agent 1   │  │   Agent 2   │  │   Agent N   │    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘    │
│         │                │                │            │
│         └────────────────┴────────────────┘            │
│                          │                             │
│                ┌─────────▼─────────┐                   │
│                │   Agent Pool      │                   │
│                │  (Go Runtime)     │                   │
│                └─────────┬─────────┘                   │
└──────────────────────────┼─────────────────────────────┘
                           │ HTTP/REST
                ┌──────────▼──────────┐
                │  Oxigraph Service   │
                │  (Python/Flask)     │
                │                     │
                │  ┌───────────────┐  │
                │  │  Oxigraph     │  │
                │  │  RDF Store    │  │
                │  └───────────────┘  │
                └─────────────────────┘
```

## 🏃 Quick Start

### Prerequisites

- Python 3.11+
- Go 1.23+
- Docker (optional)

### Option 1: Docker (Recommended)

```bash
cd orchestrator
docker-compose up -d
```

The service will be available at `http://localhost:5000`

### Option 2: Manual Setup

1. **Install Python dependencies:**

```bash
cd orchestrator
pip install -r requirements.txt
```

2. **Start the Oxigraph service:**

```bash
python oxigraph_service.py
```

3. **Run the Go example:**

```bash
cd ..
go run orchestrator/example.go
```

## 📚 Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "claude-squad/orchestrator"
)

func main() {
    ctx := context.Background()

    // Create agent pool
    pool, err := orchestrator.NewAgentPool(
        "http://localhost:5000",
        &YourAgentExecutor{},
    )
    if err != nil {
        panic(err)
    }

    // Start processing
    pool.Start(ctx)
    defer pool.Stop()

    // Submit tasks
    taskID, err := pool.SubmitTask(&orchestrator.Task{
        Description: "Analyze codebase",
        Priority:    10,
    })

    // Wait for completion
    pool.WaitForCompletion(ctx)
}
```

### Complex Dependency Graph

```go
// Create dependency chain: T1 -> T2 -> T3
t1, _ := pool.SubmitTask(&orchestrator.Task{
    Description: "Read files",
    Priority:    10,
})

t2, _ := pool.SubmitTask(&orchestrator.Task{
    Description:  "Process data",
    Priority:     9,
    Dependencies: []string{t1},
})

t3, _ := pool.SubmitTask(&orchestrator.Task{
    Description:  "Generate report",
    Priority:     8,
    Dependencies: []string{t2},
})
```

### Parallel Task Execution

```go
// Submit 10 independent tasks - they'll execute concurrently
taskIDs := []string{}
for i := 0; i < 10; i++ {
    taskID, _ := pool.SubmitTask(&orchestrator.Task{
        Description: fmt.Sprintf("Parallel task %d", i),
        Priority:    5,
    })
    taskIDs = append(taskIDs, taskID)
}

// Final task depends on all parallel tasks
pool.SubmitTask(&orchestrator.Task{
    Description:  "Aggregate results",
    Priority:     10,
    Dependencies: taskIDs,
})
```

### Monitoring Dashboard

```go
import "claude-squad/orchestrator"

// Run the TUI dashboard
orchestrator.RunDashboard(ctx, "http://localhost:5000")
```

## 🔌 API Reference

### REST Endpoints

#### Create Task
```http
POST /tasks
Content-Type: application/json

{
  "description": "Task description",
  "priority": 10,
  "dependencies": ["task-id-1", "task-id-2"]
}
```

#### Update Task Status
```http
PUT /tasks/{task_id}/status
Content-Type: application/json

{
  "status": "completed",
  "result": "Task result"
}
```

#### Get Ready Tasks
```http
GET /tasks/ready?limit=10
```

#### Get Analytics
```http
GET /analytics
```

Response:
```json
{
  "status_counts": {
    "pending": 5,
    "running": 3,
    "completed": 10,
    "failed": 1
  },
  "total_tasks": 19,
  "running_count": 3,
  "max_concurrent": 10,
  "available_slots": 7
}
```

#### Get Task Dependency Chain
```http
GET /tasks/{task_id}/chain
```

#### Optimize Task Distribution
```http
GET /optimize
```

### Go Client API

```go
type Client interface {
    // Health checks service status
    Health() error

    // CreateTask creates a new task
    CreateTask(task *Task) (string, error)

    // UpdateTaskStatus updates task status
    UpdateTaskStatus(taskID, status string, result *string) error

    // GetReadyTasks gets executable tasks
    GetReadyTasks(limit int) ([]TaskInfo, error)

    // GetRunningTasks gets currently running tasks
    GetRunningTasks() ([]string, error)

    // GetAnalytics gets execution metrics
    GetAnalytics() (*Analytics, error)

    // GetTaskChain gets dependency chain
    GetTaskChain(taskID string) ([]DependencyChain, error)

    // OptimizeDistribution gets optimized task selection
    OptimizeDistribution() ([]string, error)
}
```

## 🧪 Testing

Run the example scenarios:

```bash
# Basic example
go run orchestrator/example.go

# Advanced example with complex dependencies
# (Modify example.go to call AdvancedExample())
```

Monitor with dashboard:
```bash
go run -tags dashboard orchestrator/dashboard.go
```

## 🎯 Advanced Features

### Intelligent Task Distribution

The orchestrator uses several algorithms to optimize task execution:

1. **Priority-based scheduling**: Higher priority tasks execute first
2. **Dependency resolution**: Tasks only run when dependencies complete
3. **Diversity optimization**: Maximizes parallelism by selecting diverse tasks
4. **Load balancing**: Distributes work evenly across available slots

### Knowledge Graph Queries

The Oxigraph RDF store enables powerful semantic queries:

```sparql
# Find all tasks blocked by failed dependencies
PREFIX cs: <http://claude-squad.ai/ontology#>

SELECT ?task ?description
WHERE {
    ?task cs:hasStatus "pending" ;
          cs:hasDescription ?description ;
          cs:dependsOn ?dep .
    ?dep cs:hasStatus "failed" .
}
```

### Metadata and Tagging

Tasks can include arbitrary metadata:

```go
pool.SubmitTask(&orchestrator.Task{
    Description: "Analyze performance",
    Metadata: map[string]string{
        "type": "analysis",
        "component": "backend",
        "severity": "high",
    },
})
```

## 🔧 Configuration

### Environment Variables

- `FLASK_ENV`: Flask environment (default: `production`)
- `ORCHESTRATOR_HOST`: Service host (default: `0.0.0.0`)
- `ORCHESTRATOR_PORT`: Service port (default: `5000`)
- `MAX_CONCURRENT_AGENTS`: Maximum concurrent agents (default: `10`)

### Tuning Parameters

Adjust in `pool.go`:

```go
const (
    MaxConcurrentAgents = 10          // Max parallel agents
    pollingInterval     = 2 * time.Second  // Task polling frequency
)
```

## 📊 Monitoring

### Metrics Available

- Total tasks (by status)
- Running task count
- Available execution slots
- Utilization percentage
- Task completion rate
- Average execution time
- Dependency chain depth

### Dashboard Features

- Real-time task status
- Utilization bar chart
- Color-coded status indicators
- Auto-refresh (2-second interval)
- Error notification

## 🚦 Production Considerations

### Scaling

- **Horizontal scaling**: Run multiple orchestrator instances with load balancer
- **Persistent storage**: Use Oxigraph with disk backend for durability
- **Rate limiting**: Add rate limiting to API endpoints
- **Monitoring**: Integrate with Prometheus/Grafana

### Security

- Add authentication/authorization
- Enable HTTPS/TLS
- Validate input thoroughly
- Implement request signing

### High Availability

- Deploy orchestrator with redundancy
- Use persistent volume for RDF store
- Implement circuit breakers
- Add request retries with exponential backoff

## 🤝 Integration with Claude Squad

This orchestrator integrates seamlessly with Claude Squad:

```go
// In your Claude Squad session management
import "claude-squad/orchestrator"

type ClaudeAgentExecutor struct {
    sessionManager *session.Manager
}

func (e *ClaudeAgentExecutor) Execute(ctx context.Context, task *orchestrator.Task) (*string, error) {
    // Create a new Claude Code session for this task
    instance, err := e.sessionManager.CreateInstance(task.Description)
    if err != nil {
        return nil, err
    }

    // Wait for completion
    result := instance.WaitForCompletion(ctx)

    return &result, nil
}
```

## 📄 License

Inherits license from parent Claude Squad project (AGPL-3.0)

## 🙏 Credits

- **Oxigraph**: Fast RDF graph database
- **Flask**: Lightweight Python web framework
- **Bubble Tea**: Elegant Go TUI framework
- **Claude Squad**: Multi-agent terminal application

## 🐛 Troubleshooting

### Service won't start

```bash
# Check if port 5000 is available
lsof -i :5000

# Check Python dependencies
pip install -r requirements.txt --upgrade
```

### Tasks stuck in pending

```bash
# Check for dependency cycles
curl http://localhost:5000/tasks/{task_id}/chain

# Verify orchestrator is running
curl http://localhost:5000/health
```

### High memory usage

- Reduce `MaxConcurrentAgents`
- Implement task result cleanup
- Use streaming for large results

## 📈 Roadmap

- [ ] Persistent task storage
- [ ] Task retry policies
- [ ] Webhook notifications
- [ ] Distributed execution
- [ ] Web UI dashboard
- [ ] Task templates
- [ ] Scheduling/cron support
- [ ] Multi-tenant support
