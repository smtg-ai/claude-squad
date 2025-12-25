# Async Notification System

A production-quality, high-performance async notification system for Go with support for multiple delivery channels, priority queuing, retry logic with exponential backoff, and at-least-once delivery guarantees.

## Features

### Core Functionality
- **Non-blocking notification delivery** - All notifications are processed asynchronously
- **Multiple notification channels** - In-app, system, and webhook delivery
- **Priority-based processing** - Critical, High, Normal, and Low priority levels
- **Batch operations** - Send multiple notifications efficiently
- **Notification templates** - Reusable message templates with variable substitution
- **Delivery guarantees** - At-least-once delivery with tracking

### Advanced Features
- **Retry with exponential backoff** - Automatic retry on failure with configurable backoff
- **Priority queue** - Higher priority notifications processed first
- **Concurrent workers** - Configurable number of worker goroutines
- **Graceful shutdown** - Ensures all queued notifications are processed
- **Real-time metrics** - Track sent, failed, and queued notifications
- **Callback support** - Register callbacks for notification events
- **Context-aware** - Support for cancellation and timeouts

## Architecture

### Components

#### 1. NotificationService
The main service that orchestrates notification delivery:
- Manages worker pool for async processing
- Routes notifications to appropriate channels
- Tracks delivery status and metrics
- Handles graceful shutdown

#### 2. NotificationQueue
Priority queue implementation:
- Thread-safe queue with mutex protection
- Priority-based ordering (Critical > High > Normal > Low)
- Blocking dequeue with condition variables
- Support for graceful closure

#### 3. DeliveryTracker
Ensures at-least-once delivery:
- Tracks delivery attempts per notification per channel
- Implements exponential backoff for retries
- Configurable max retries and backoff parameters
- Thread-safe record management

#### 4. TemplateEngine
Template rendering for notifications:
- Register reusable templates
- Variable substitution using Go's text/template
- Thread-safe template storage
- Error handling for malformed templates

#### 5. NotificationChannel Interface
Abstraction for delivery mechanisms:
```go
type NotificationChannel interface {
    Name() string
    Send(ctx context.Context, notification *Notification) error
    SupportsRecipient(recipient string) bool
}
```

### Channel Implementations

#### InAppChannel
Stores notifications for in-app display:
- Thread-safe notification storage
- Callback support for real-time updates
- Retrieve all notifications
- Simulated processing delay

#### SystemChannel
Delivers OS-level notifications:
- Platform-specific notification APIs
- Non-blocking execution
- Configurable command function (for testing)
- Title and message formatting

#### WebhookChannel
Sends notifications via HTTP webhooks:
- POST requests with JSON payload
- Configurable timeout (10 seconds default)
- Context support for cancellation
- HTTP status code validation

## Data Structures

### Notification
```go
type Notification struct {
    ID          string                 // Unique identifier
    Type        NotificationType       // info, warning, error, success
    Priority    Priority              // Low, Normal, High, Critical
    Payload     map[string]interface{} // Notification data
    Recipients  []string              // Target recipients
    Template    string                // Optional template name
    CreatedAt   time.Time             // Creation timestamp
    ScheduledAt time.Time             // Optional scheduled delivery
    Channels    []string              // Target channels
    Metadata    map[string]interface{} // Additional metadata
}
```

### Priority Levels
```go
const (
    PriorityLow      Priority = 0
    PriorityNormal   Priority = 1
    PriorityHigh     Priority = 2
    PriorityCritical Priority = 3
)
```

### Notification Types
```go
const (
    TypeInfo    NotificationType = "info"
    TypeWarning NotificationType = "warning"
    TypeError   NotificationType = "error"
    TypeSuccess NotificationType = "success"
)
```

### Delivery Status
```go
const (
    StatusPending   DeliveryStatus = "pending"
    StatusSent      DeliveryStatus = "sent"
    StatusFailed    DeliveryStatus = "failed"
    StatusRetrying  DeliveryStatus = "retrying"
    StatusCancelled DeliveryStatus = "cancelled"
)
```

## Usage

### Basic Usage

```go
// Create service with 5 worker goroutines
service := NewNotificationService(NotificationServiceConfig{
    Workers: 5,
})
defer service.Shutdown(10 * time.Second)

// Register a channel
inAppChannel := NewInAppChannel()
service.RegisterChannel(inAppChannel)

// Send a notification
notification := &Notification{
    Type:       TypeInfo,
    Priority:   PriorityNormal,
    Payload:    map[string]interface{}{"message": "Hello, World!"},
    Recipients: []string{"user123"},
    Channels:   []string{"in-app"},
}

err := service.Notify(notification)
```

### Using Templates

```go
// Register a template
service.RegisterTemplate("welcome",
    "Welcome {{.username}}! You have {{.tasks}} tasks pending.")

// Send notification with template
notification := &Notification{
    Type:     TypeSuccess,
    Priority: PriorityHigh,
    Template: "welcome",
    Payload: map[string]interface{}{
        "username": "Alice",
        "tasks":    5,
    },
    Channels: []string{"in-app"},
}

service.Notify(notification)
```

### Multi-Channel Delivery

```go
// Register multiple channels
service.RegisterChannel(NewInAppChannel())
service.RegisterChannel(NewSystemChannel())
service.RegisterChannel(NewWebhookChannel("http://example.com/webhook"))

// Send to all channels
notification := &Notification{
    Type:     TypeError,
    Priority: PriorityCritical,
    Payload:  map[string]interface{}{"message": "Critical error!"},
    Channels: []string{"in-app", "system", "webhook"},
}

service.Notify(notification)
```

### Batch Operations

```go
// Create batch
var batch []*Notification
for i := 0; i < 100; i++ {
    batch = append(batch, &Notification{
        Type:     TypeInfo,
        Priority: PriorityNormal,
        Payload:  map[string]interface{}{"message": fmt.Sprintf("Message %d", i)},
        Channels: []string{"in-app"},
    })
}

// Send batch
err := service.BatchNotify(batch)
```

### Broadcasting to Multiple Recipients

```go
recipients := []string{"user1", "user2", "user3"}

notification := &Notification{
    Type:     TypeWarning,
    Priority: PriorityHigh,
    Payload:  map[string]interface{}{"message": "System maintenance tonight"},
    Channels: []string{"in-app"},
}

err := service.NotifyAll(recipients, notification)
```

### Callback Support

```go
inAppChannel := NewInAppChannel()
inAppChannel.OnNotify(func(n Notification) {
    fmt.Printf("Received: %s - %v\n", n.Type, n.Payload["message"])
})

service.RegisterChannel(inAppChannel)
```

### Metrics and Monitoring

```go
// Get metrics
sent, failed := service.GetMetrics()
queueSize := service.GetQueueSize()

fmt.Printf("Sent: %d, Failed: %d, Queued: %d\n", sent, failed, queueSize)

// Check delivery status
record := service.tracker.GetRecord(notificationID, "webhook")
if record != nil {
    fmt.Printf("Status: %s, Attempts: %d\n", record.Status, record.Attempts)
}
```

## Configuration

### NotificationServiceConfig

```go
type NotificationServiceConfig struct {
    Workers int // Number of concurrent worker goroutines (default: 5)
}
```

### DeliveryTracker Configuration

Default values (can be customized by accessing tracker directly):
- **MaxRetries**: 5 attempts
- **BaseBackoff**: 1 second
- **MaxBackoff**: 5 minutes
- **BackoffFactor**: 2.0 (exponential)

Retry schedule example:
1. Attempt 1: Immediate
2. Attempt 2: After 1 second
3. Attempt 3: After 2 seconds
4. Attempt 4: After 4 seconds
5. Attempt 5: After 8 seconds

### Channel Timeouts

- **WebhookChannel**: 10 seconds per request
- **NotificationService context**: 30 seconds per send operation

## Concurrency Model

### Worker Pool
- Configurable number of worker goroutines
- Each worker processes notifications from the priority queue
- Non-blocking notification submission
- Graceful shutdown waits for all workers

### Thread Safety
- All operations are thread-safe
- Mutex protection for shared state
- Condition variables for queue synchronization
- Read-write locks for optimal performance

### Goroutine Usage
- Main service: 1 goroutine for retry loop
- Workers: N goroutines (configurable)
- Per-notification: 1 goroutine per channel delivery
- Total: 1 + N + (active notifications × channels)

## Performance Characteristics

### Throughput
- **Queue operations**: O(n) insert (priority-based), O(1) dequeue
- **Channel registration**: O(1) lookup
- **Template rendering**: O(1) lookup, O(n) rendering

### Memory Usage
- **Per notification**: ~200-500 bytes (depending on payload)
- **Queue overhead**: Minimal (slice-based)
- **Worker overhead**: ~8KB per goroutine stack

### Latency
- **Enqueue**: < 1ms (priority insertion)
- **Dequeue**: Immediate or blocking
- **In-app delivery**: ~10ms
- **System delivery**: ~50-100ms
- **Webhook delivery**: Network dependent (timeout: 10s)

## Error Handling

### Retry Logic
Failed deliveries are automatically retried with exponential backoff:
1. Notification fails to send
2. DeliveryTracker marks as failed
3. If attempts < maxRetries, schedule retry
4. Calculate backoff: baseBackoff × (backoffFactor ^ attempts)
5. Retry after backoff period
6. If maxRetries exceeded, mark as permanently failed

### Graceful Degradation
- Individual channel failures don't affect other channels
- Failed webhooks don't block in-app notifications
- Service continues processing even if some notifications fail

### Context Cancellation
- All send operations respect context cancellation
- Shutdown cancels pending operations gracefully
- Timeout prevents indefinite blocking

## Testing

### Run Tests
```bash
go test -v -timeout 60s notifications_test.go notifications.go
```

### Test Coverage
- Unit tests for all components
- Integration tests for service
- Concurrency tests with 100+ notifications
- Priority ordering verification
- Retry logic validation
- Graceful shutdown testing

### Benchmarks
```bash
go test -bench=. -benchmem notifications_test.go notifications.go
```

## Best Practices

### 1. Worker Configuration
- Start with 5 workers for most applications
- Increase for high-throughput scenarios (1000+ notifications/sec)
- Decrease for resource-constrained environments

### 2. Priority Usage
- **Critical**: System failures, security alerts
- **High**: User-facing errors, important updates
- **Normal**: General notifications, status updates
- **Low**: Background tasks, informational messages

### 3. Template Design
- Keep templates simple and focused
- Validate templates at startup
- Use meaningful variable names
- Handle missing variables gracefully

### 4. Channel Selection
- Use in-app for user-facing notifications
- Use system for desktop/mobile alerts
- Use webhook for integrations and external systems
- Combine channels for critical notifications

### 5. Error Handling
- Monitor failed delivery metrics
- Set up alerts for high failure rates
- Implement webhook endpoint health checks
- Log delivery failures for debugging

### 6. Shutdown
- Always call Shutdown() with appropriate timeout
- Allow enough time for queue to drain
- Monitor queue size before shutdown
- Handle shutdown errors appropriately

## Production Considerations

### Scalability
- Horizontal: Run multiple service instances with shared queue (requires external queue)
- Vertical: Increase worker count based on CPU cores
- Queue: Consider external queue (Redis, RabbitMQ) for distributed systems

### Monitoring
Track these metrics:
- Notifications sent/failed per second
- Queue size and depth
- Delivery latency per channel
- Retry rates and backoff times
- Worker utilization

### Reliability
- Persist notifications for disaster recovery
- Implement dead letter queue for permanent failures
- Set up alerting for delivery failures
- Regular health checks for webhook endpoints

### Security
- Validate webhook URLs
- Implement authentication for webhook channels
- Sanitize notification payloads
- Rate limit notification creation

## Examples

See `notifications_example.go` for comprehensive examples including:
- Basic usage
- Template usage
- Multi-channel delivery
- Priority handling
- Batch operations
- Broadcasting
- Callbacks
- Real-time monitoring
- Error handling
- Graceful shutdown
- Custom channels
- Complex workflows

## License

Part of claude-squad project.
