package concurrency

import (
	"fmt"
	"log"
	"time"
)

// ExampleBasicUsage demonstrates basic notification sending
func ExampleNotificationBasicUsage() {
	// Create notification service with 5 workers
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 5,
	})
	defer service.Shutdown(10 * time.Second)

	// Register an in-app notification channel
	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Send a simple notification
	notification := &Notification{
		Type:       TypeInfo,
		Priority:   PriorityNormal,
		Payload:    map[string]interface{}{"message": "Hello, World!"},
		Recipients: []string{"user123"},
		Channels:   []string{"in-app"},
	}

	err := service.Notify(notification)
	if err != nil {
		fmt.Printf("Error sending notification: %v\n", err)
		return
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Check metrics
	sent, failed := service.GetMetrics()
	fmt.Printf("Sent: %d, Failed: %d\n", sent, failed)
}

// ExampleWithTemplate demonstrates using templates for notifications
func ExampleWithTemplate() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 3,
	})
	defer service.Shutdown(10 * time.Second)

	// Register templates
	service.RegisterTemplate("welcome", "Welcome {{.username}}! You have {{.tasks}} tasks pending.")
	service.RegisterTemplate("alert", "ALERT: {{.message}} - Priority: {{.priority}}")

	// Register channels
	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Send notification using template
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
	time.Sleep(100 * time.Millisecond)

	// View received notifications
	notifications := inAppChannel.GetNotifications()
	for _, n := range notifications {
		fmt.Printf("Notification: %v\n", n.Payload["message"])
	}
}

// ExampleMultiChannel demonstrates sending to multiple channels
func ExampleMultiChannel() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 5,
	})
	defer service.Shutdown(10 * time.Second)

	// Register multiple channels
	inAppChannel := NewInAppChannel()
	systemChannel := NewSystemChannel()
	webhookChannel, err := NewWebhookChannel("http://example.com/webhook")
	if err != nil {
		log.Fatalf("Failed to create webhook channel: %v", err)
	}

	service.RegisterChannel(inAppChannel)
	service.RegisterChannel(systemChannel)
	service.RegisterChannel(webhookChannel)

	// Send to all channels
	notification := &Notification{
		Type:     TypeError,
		Priority: PriorityCritical,
		Payload: map[string]interface{}{
			"message": "Critical system error detected!",
			"error_code": 500,
		},
		Recipients: []string{"admin@example.com"},
		Channels:   []string{"in-app", "system", "webhook"},
	}

	service.Notify(notification)
	time.Sleep(200 * time.Millisecond)

	fmt.Println("Notification sent to all channels")
}

// ExamplePriorityHandling demonstrates priority-based processing
func ExamplePriorityHandling() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 1, // Single worker to see priority ordering
	})
	defer service.Shutdown(10 * time.Second)

	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Send notifications with different priorities
	notifications := []*Notification{
		{
			ID:       "low-1",
			Type:     TypeInfo,
			Priority: PriorityLow,
			Payload:  map[string]interface{}{"message": "Low priority message"},
			Channels: []string{"in-app"},
		},
		{
			ID:       "critical-1",
			Type:     TypeError,
			Priority: PriorityCritical,
			Payload:  map[string]interface{}{"message": "Critical alert!"},
			Channels: []string{"in-app"},
		},
		{
			ID:       "normal-1",
			Type:     TypeInfo,
			Priority: PriorityNormal,
			Payload:  map[string]interface{}{"message": "Normal message"},
			Channels: []string{"in-app"},
		},
		{
			ID:       "high-1",
			Type:     TypeWarning,
			Priority: PriorityHigh,
			Payload:  map[string]interface{}{"message": "High priority warning"},
			Channels: []string{"in-app"},
		},
	}

	// Send all notifications
	for _, n := range notifications {
		service.Notify(n)
	}

	time.Sleep(500 * time.Millisecond)

	// Critical should be processed first, then high, then normal, then low
	fmt.Println("Notifications processed in priority order")
}

// ExampleBatchNotifications demonstrates batch sending
func ExampleBatchNotifications() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 10,
	})
	defer service.Shutdown(10 * time.Second)

	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Create batch of notifications
	var batch []*Notification
	for i := 0; i < 50; i++ {
		batch = append(batch, &Notification{
			Type:     TypeInfo,
			Priority: PriorityNormal,
			Payload: map[string]interface{}{
				"message": fmt.Sprintf("Batch notification %d", i+1),
			},
			Channels: []string{"in-app"},
		})
	}

	// Send batch
	err := service.BatchNotify(batch)
	if err != nil {
		fmt.Printf("Error in batch notify: %v\n", err)
		return
	}

	time.Sleep(1 * time.Second)

	sent, _ := service.GetMetrics()
	fmt.Printf("Sent %d notifications in batch\n", sent)
}

// ExampleNotifyAll demonstrates broadcasting to multiple recipients
func ExampleNotifyAll() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 5,
	})
	defer service.Shutdown(10 * time.Second)

	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Broadcast to all users
	recipients := []string{
		"user1@example.com",
		"user2@example.com",
		"user3@example.com",
		"admin@example.com",
	}

	notification := &Notification{
		Type:     TypeWarning,
		Priority: PriorityHigh,
		Payload: map[string]interface{}{
			"message": "System maintenance scheduled for tonight at 10 PM",
		},
		Channels: []string{"in-app"},
	}

	err := service.NotifyAll(recipients, notification)
	if err != nil {
		fmt.Printf("Error broadcasting: %v\n", err)
		return
	}

	time.Sleep(200 * time.Millisecond)
	fmt.Printf("Notification broadcast to %d recipients\n", len(recipients))
}

// ExampleWithCallback demonstrates receiving notifications with callbacks
func ExampleWithCallback() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 3,
	})
	defer service.Shutdown(10 * time.Second)

	// Setup channel with callback
	inAppChannel := NewInAppChannel()
	inAppChannel.OnNotify(func(n Notification) {
		fmt.Printf("Received notification: %s - %v\n", n.Type, n.Payload["message"])
	})

	service.RegisterChannel(inAppChannel)

	// Send notifications
	for i := 0; i < 5; i++ {
		service.Notify(&Notification{
			Type:     TypeInfo,
			Priority: PriorityNormal,
			Payload: map[string]interface{}{
				"message": fmt.Sprintf("Message %d", i+1),
			},
			Channels: []string{"in-app"},
		})
	}

	time.Sleep(300 * time.Millisecond)
	fmt.Println("All callbacks processed")
}

// ExampleRealTimeMonitoring demonstrates real-time monitoring scenario
func ExampleRealTimeMonitoring() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 10,
	})
	defer service.Shutdown(15 * time.Second)

	// Register all channels
	inAppChannel := NewInAppChannel()
	systemChannel := NewSystemChannel()

	service.RegisterChannel(inAppChannel)
	service.RegisterChannel(systemChannel)

	// Register alert template
	service.RegisterTemplate("metric_alert",
		"ALERT: {{.metric}} has reached {{.value}}{{.unit}} (threshold: {{.threshold}}{{.unit}})")

	// Simulate monitoring events
	metrics := []struct {
		name      string
		value     int
		threshold int
		priority  Priority
	}{
		{"CPU Usage", 95, 80, PriorityCritical},
		{"Memory Usage", 75, 70, PriorityHigh},
		{"Disk Usage", 60, 90, PriorityNormal},
		{"Network Latency", 250, 200, PriorityHigh},
	}

	for _, m := range metrics {
		notification := &Notification{
			Type:     TypeWarning,
			Priority: m.priority,
			Template: "metric_alert",
			Payload: map[string]interface{}{
				"metric":    m.name,
				"value":     m.value,
				"unit":      "%",
				"threshold": m.threshold,
			},
			Recipients: []string{"ops-team@example.com"},
			Channels:   []string{"in-app", "system"},
		}

		service.Notify(notification)
	}

	time.Sleep(1 * time.Second)

	sent, failed := service.GetMetrics()
	queueSize := service.GetQueueSize()

	fmt.Printf("Monitoring stats - Sent: %d, Failed: %d, Queue: %d\n", sent, failed, queueSize)
}

// ExampleErrorHandling demonstrates error handling and retry logic
func ExampleErrorHandling() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 3,
	})
	defer service.Shutdown(10 * time.Second)

	// Register a failing webhook (will retry)
	failingWebhook, err := NewWebhookChannel("http://invalid-url-that-will-fail.local")
	if err != nil {
		log.Fatalf("Failed to create webhook channel: %v", err)
	}
	service.RegisterChannel(failingWebhook)

	// Also register working channel
	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	notification := &Notification{
		Type:     TypeInfo,
		Priority: PriorityNormal,
		Payload:  map[string]interface{}{"message": "This will fail on webhook but succeed on in-app"},
		Channels: []string{"webhook", "in-app"},
	}

	service.Notify(notification)

	// Wait for initial attempt and retries
	time.Sleep(2 * time.Second)

	sent, failed := service.GetMetrics()
	fmt.Printf("Sent: %d, Failed: %d (webhook failed, in-app succeeded)\n", sent, failed)

	// Check delivery records
	record := service.tracker.GetRecord(notification.ID, "webhook")
	if record != nil {
		fmt.Printf("Webhook delivery status: %s, attempts: %d\n", record.Status, record.Attempts)
	}
}

// ExampleGracefulShutdown demonstrates graceful shutdown
func ExampleNotificationGracefulShutdown() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 5,
	})

	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Send many notifications
	for i := 0; i < 100; i++ {
		service.Notify(&Notification{
			Type:     TypeInfo,
			Priority: PriorityNormal,
			Payload:  map[string]interface{}{"message": fmt.Sprintf("Message %d", i)},
			Channels: []string{"in-app"},
		})
	}

	fmt.Printf("Queue size before shutdown: %d\n", service.GetQueueSize())

	// Graceful shutdown - waits for queue to empty
	err := service.Shutdown(10 * time.Second)
	if err != nil {
		fmt.Printf("Shutdown error: %v\n", err)
	} else {
		fmt.Println("All notifications processed before shutdown")
	}

	sent, failed := service.GetMetrics()
	fmt.Printf("Final stats - Sent: %d, Failed: %d\n", sent, failed)
}

// ExampleCustomChannel demonstrates creating a custom notification channel
func ExampleCustomChannel() {
	// Custom channel that logs to a file (simplified example)
	type FileLogChannel struct {
		name string
	}

	// Implement NotificationChannel interface methods
	fileChannel := &FileLogChannel{name: "file-log"}

	// In a real implementation, you would add these methods:
	// func (f *FileLogChannel) Name() string { return f.name }
	// func (f *FileLogChannel) Send(ctx context.Context, n *Notification) error {
	//     // Write to file
	//     return nil
	// }
	// func (f *FileLogChannel) SupportsRecipient(recipient string) bool { return true }

	service := NewNotificationService(NotificationServiceConfig{
		Workers: 3,
	})
	defer service.Shutdown(10 * time.Second)

	// Once implemented, you would register like this:
	// service.RegisterChannel(fileChannel)

	fmt.Printf("Custom channel '%s' ready\n", fileChannel.name)
}

// ExampleComplexWorkflow demonstrates a complex notification workflow
func ExampleNotificationComplexWorkflow() {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 8,
	})
	defer service.Shutdown(15 * time.Second)

	// Setup channels
	inAppChannel := NewInAppChannel()
	service.RegisterChannel(inAppChannel)

	// Register multiple templates
	templates := map[string]string{
		"order_created":   "Order #{{.order_id}} created successfully! Total: ${{.total}}",
		"order_shipped":   "Order #{{.order_id}} has shipped! Tracking: {{.tracking}}",
		"order_delivered": "Order #{{.order_id}} delivered to {{.address}}",
		"payment_failed":  "Payment failed for order #{{.order_id}}: {{.reason}}",
	}

	for name, tmpl := range templates {
		service.RegisterTemplate(name, tmpl)
	}

	// Simulate order processing workflow
	orderID := "ORD-12345"
	userID := "user@example.com"

	// 1. Order created
	service.Notify(&Notification{
		Type:       TypeSuccess,
		Priority:   PriorityNormal,
		Template:   "order_created",
		Payload:    map[string]interface{}{"order_id": orderID, "total": "99.99"},
		Recipients: []string{userID},
		Channels:   []string{"in-app"},
	})

	time.Sleep(100 * time.Millisecond)

	// 2. Payment processing (simulated failure)
	service.Notify(&Notification{
		Type:       TypeError,
		Priority:   PriorityHigh,
		Template:   "payment_failed",
		Payload:    map[string]interface{}{"order_id": orderID, "reason": "Insufficient funds"},
		Recipients: []string{userID},
		Channels:   []string{"in-app"},
	})

	time.Sleep(100 * time.Millisecond)

	// 3. After payment retry - Order shipped
	service.Notify(&Notification{
		Type:       TypeSuccess,
		Priority:   PriorityHigh,
		Template:   "order_shipped",
		Payload:    map[string]interface{}{"order_id": orderID, "tracking": "1Z999AA10123456784"},
		Recipients: []string{userID},
		Channels:   []string{"in-app"},
	})

	time.Sleep(100 * time.Millisecond)

	// 4. Order delivered
	service.Notify(&Notification{
		Type:       TypeSuccess,
		Priority:   PriorityNormal,
		Template:   "order_delivered",
		Payload:    map[string]interface{}{"order_id": orderID, "address": "123 Main St"},
		Recipients: []string{userID},
		Channels:   []string{"in-app"},
	})

	time.Sleep(300 * time.Millisecond)

	sent, failed := service.GetMetrics()
	fmt.Printf("Order workflow complete - Sent: %d, Failed: %d\n", sent, failed)

	// Display all notifications received
	notifications := inAppChannel.GetNotifications()
	fmt.Printf("\nUser received %d notifications:\n", len(notifications))
	for i, n := range notifications {
		fmt.Printf("%d. [%s] %v\n", i+1, n.Type, n.Payload["message"])
	}
}
