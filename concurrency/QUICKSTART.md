# Notification System Quick Start

Get up and running with the async notification system in 5 minutes.

## Installation

```bash
go get github.com/google/uuid
```

## 30-Second Example

```go
package main

import (
    "fmt"
    "time"
    "your-project/concurrency"
)

func main() {
    // 1. Create service
    service := concurrency.NewNotificationService(concurrency.NotificationServiceConfig{
        Workers: 5,
    })
    defer service.Shutdown(10 * time.Second)

    // 2. Register channel
    channel := concurrency.NewInAppChannel()
    service.RegisterChannel(channel)

    // 3. Send notification
    service.Notify(&concurrency.Notification{
        Type:     concurrency.TypeInfo,
        Priority: concurrency.PriorityNormal,
        Payload:  map[string]interface{}{"message": "Hello, World!"},
        Channels: []string{"in-app"},
    })

    // 4. Wait and check
    time.Sleep(100 * time.Millisecond)
    sent, failed := service.GetMetrics()
    fmt.Printf("Sent: %d, Failed: %d\n", sent, failed)
}
```

## Common Use Cases

### 1. User Notifications

```go
// Setup
service := NewNotificationService(NotificationServiceConfig{Workers: 5})
defer service.Shutdown(10 * time.Second)

inApp := NewInAppChannel()
service.RegisterChannel(inApp)

// Send
service.Notify(&Notification{
    Type:       TypeSuccess,
    Priority:   PriorityHigh,
    Payload:    map[string]interface{}{"message": "Your order has shipped!"},
    Recipients: []string{"user@example.com"},
    Channels:   []string{"in-app"},
})
```

### 2. System Alerts

```go
// Setup
service := NewNotificationService(NotificationServiceConfig{Workers: 10})
defer service.Shutdown(15 * time.Second)

system := NewSystemChannel()
webhook := NewWebhookChannel("https://hooks.slack.com/services/YOUR/WEBHOOK")
service.RegisterChannel(system)
service.RegisterChannel(webhook)

// Send critical alert
service.Notify(&Notification{
    Type:     TypeError,
    Priority: PriorityCritical,
    Payload: map[string]interface{}{
        "message": "Database connection lost!",
        "server":  "prod-db-01",
    },
    Channels: []string{"system", "webhook"},
})
```

### 3. Batch Processing

```go
// Create batch
notifications := make([]*Notification, 100)
for i := 0; i < 100; i++ {
    notifications[i] = &Notification{
        Type:     TypeInfo,
        Priority: PriorityNormal,
        Payload:  map[string]interface{}{"message": fmt.Sprintf("Task %d completed", i)},
        Channels: []string{"in-app"},
    }
}

// Send all at once
service.BatchNotify(notifications)
```

### 4. Templates

```go
// Register template
service.RegisterTemplate("order_update",
    "Order #{{.order_id}} status: {{.status}}")

// Use template
service.Notify(&Notification{
    Type:     TypeSuccess,
    Priority: PriorityNormal,
    Template: "order_update",
    Payload: map[string]interface{}{
        "order_id": "12345",
        "status":   "Delivered",
    },
    Channels: []string{"in-app"},
})
```

### 5. Real-time Updates

```go
// Setup with callback
channel := NewInAppChannel()
channel.OnNotify(func(n Notification) {
    fmt.Printf("[%s] %v\n", n.Type, n.Payload["message"])
})

service.RegisterChannel(channel)

// All notifications will trigger the callback
service.Notify(&Notification{
    Type:     TypeInfo,
    Priority: PriorityNormal,
    Payload:  map[string]interface{}{"message": "Real-time update"},
    Channels: []string{"in-app"},
})
```

## Priority Levels

```go
// Use appropriate priority for your use case
PriorityLow      // Background tasks, logs
PriorityNormal   // Standard notifications
PriorityHigh     // Important updates
PriorityCritical // System failures, security alerts
```

## Testing Your Integration

```go
func TestNotifications(t *testing.T) {
    service := NewNotificationService(NotificationServiceConfig{Workers: 2})
    defer service.Shutdown(5 * time.Second)

    channel := NewInAppChannel()
    service.RegisterChannel(channel)

    // Send test notification
    service.Notify(&Notification{
        Type:     TypeInfo,
        Priority: PriorityNormal,
        Payload:  map[string]interface{}{"message": "test"},
        Channels: []string{"in-app"},
    })

    // Verify
    time.Sleep(100 * time.Millisecond)
    notifications := channel.GetNotifications()

    if len(notifications) != 1 {
        t.Errorf("Expected 1 notification, got %d", len(notifications))
    }
}
```

## Production Checklist

- [ ] Configure appropriate number of workers
- [ ] Set up monitoring for metrics (sent/failed)
- [ ] Implement webhook health checks
- [ ] Add error logging for failed deliveries
- [ ] Test graceful shutdown behavior
- [ ] Set up alerts for high failure rates
- [ ] Validate notification templates
- [ ] Test retry logic with network failures

## Performance Tips

1. **Worker Count**: Start with `Workers: 5`, increase for high throughput
2. **Batch Operations**: Use `BatchNotify()` for sending 10+ notifications
3. **Channel Selection**: In-app is fastest, webhooks are slowest
4. **Templates**: Pre-register all templates at startup
5. **Shutdown**: Allow 2-3 seconds per 100 queued notifications

## Troubleshooting

### Notifications not being delivered

```go
// Check queue size
queueSize := service.GetQueueSize()
if queueSize > 1000 {
    // Queue is backing up, increase workers
}

// Check metrics
sent, failed := service.GetMetrics()
if failed > sent * 0.1 { // More than 10% failure
    // Investigate channel issues
}
```

### Slow processing

```go
// Increase workers
service := NewNotificationService(NotificationServiceConfig{
    Workers: 10, // or more
})

// Use batch operations
service.BatchNotify(notifications) // Faster than individual Notify() calls
```

### Webhook failures

```go
// Check delivery record
record := service.tracker.GetRecord(notificationID, "webhook")
if record != nil {
    fmt.Printf("Status: %s, Attempts: %d, Error: %v\n",
        record.Status, record.Attempts, record.Error)
}
```

## Next Steps

1. Read the full documentation: [NOTIFICATIONS_README.md](NOTIFICATIONS_README.md)
2. Check out examples: [notifications_example.go](notifications_example.go)
3. Run the tests: `go test -v`
4. Implement your first notification channel
5. Set up monitoring and alerting

## Support

For issues or questions:
- Read the full README
- Check the example code
- Review the test cases
- Examine the source code (well-documented)

## Key Concepts

- **Async by default**: All notifications are non-blocking
- **Priority matters**: Critical notifications jump the queue
- **Retries are automatic**: Failed deliveries retry with exponential backoff
- **Graceful shutdown**: Always call `Shutdown()` to drain the queue
- **Thread-safe**: All operations can be called from multiple goroutines
- **At-least-once**: Delivery tracker ensures notifications aren't lost

Happy notifying!
