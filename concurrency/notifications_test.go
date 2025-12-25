package concurrency

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNotificationQueue(t *testing.T) {
	queue := NewNotificationQueue()

	// Test enqueue with different priorities
	lowPriority := &Notification{
		ID:       "1",
		Priority: PriorityLow,
		Payload:  map[string]interface{}{"message": "low"},
	}

	highPriority := &Notification{
		ID:       "2",
		Priority: PriorityHigh,
		Payload:  map[string]interface{}{"message": "high"},
	}

	normalPriority := &Notification{
		ID:       "3",
		Priority: PriorityNormal,
		Payload:  map[string]interface{}{"message": "normal"},
	}

	// Enqueue in random order
	queue.Enqueue(lowPriority)
	queue.Enqueue(highPriority)
	queue.Enqueue(normalPriority)

	// Dequeue should return in priority order
	n1, _ := queue.Dequeue()
	if n1.ID != "2" {
		t.Errorf("Expected high priority first, got %s", n1.ID)
	}

	n2, _ := queue.Dequeue()
	if n2.ID != "3" {
		t.Errorf("Expected normal priority second, got %s", n2.ID)
	}

	n3, _ := queue.Dequeue()
	if n3.ID != "1" {
		t.Errorf("Expected low priority last, got %s", n3.ID)
	}

	queue.Close()
}

func TestInAppChannel(t *testing.T) {
	channel := NewInAppChannel()

	notification := &Notification{
		ID:         "test-1",
		Type:       TypeInfo,
		Priority:   PriorityNormal,
		Payload:    map[string]interface{}{"message": "test message"},
		Recipients: []string{"user1"},
	}

	ctx := context.Background()
	err := channel.Send(ctx, notification)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	notifications := channel.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].ID != "test-1" {
		t.Errorf("Expected notification ID 'test-1', got '%s'", notifications[0].ID)
	}
}

func TestInAppChannelCallback(t *testing.T) {
	channel := NewInAppChannel()

	var receivedNotification *Notification
	var mu sync.Mutex

	channel.OnNotify(func(n Notification) {
		mu.Lock()
		receivedNotification = &n
		mu.Unlock()
	})

	notification := &Notification{
		ID:       "callback-test",
		Type:     TypeSuccess,
		Priority: PriorityHigh,
		Payload:  map[string]interface{}{"message": "callback test"},
	}

	ctx := context.Background()
	err := channel.Send(ctx, notification)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Give callback time to execute
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedNotification == nil {
		t.Fatal("Callback was not invoked")
	}

	if receivedNotification.ID != "callback-test" {
		t.Errorf("Expected notification ID 'callback-test', got '%s'", receivedNotification.ID)
	}
}

func TestTemplateEngine(t *testing.T) {
	engine := NewTemplateEngine()

	// Register a template
	err := engine.RegisterTemplate("welcome", "Welcome {{.name}}! You have {{.count}} new messages.")
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}

	// Render the template
	data := map[string]interface{}{
		"name":  "John",
		"count": 5,
	}

	result, err := engine.Render("welcome", data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	expected := "Welcome John! You have 5 new messages."
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestDeliveryTracker(t *testing.T) {
	tracker := NewDeliveryTracker()

	notificationID := "notification-1"
	channel := "in-app"

	// Track delivery
	record := tracker.TrackDelivery(notificationID, channel)
	if record.Status != StatusPending {
		t.Errorf("Expected status Pending, got %v", record.Status)
	}

	// Mark as sent
	tracker.MarkSent(notificationID, channel)
	record = tracker.GetRecord(notificationID, channel)
	if record.Status != StatusSent {
		t.Errorf("Expected status Sent, got %v", record.Status)
	}
}

func TestDeliveryTrackerRetry(t *testing.T) {
	tracker := NewDeliveryTracker()
	tracker.maxRetries = 3
	tracker.baseBackoff = 100 * time.Millisecond

	notificationID := "notification-retry"
	channel := "webhook"

	// Track delivery
	tracker.TrackDelivery(notificationID, channel)

	// Mark as failed
	shouldRetry := tracker.MarkFailed(notificationID, channel, fmt.Errorf("connection error"))
	if !shouldRetry {
		t.Error("Expected retry to be allowed")
	}

	record := tracker.GetRecord(notificationID, channel)
	if record.Status != StatusRetrying {
		t.Errorf("Expected status Retrying, got %v", record.Status)
	}

	if record.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", record.Attempts)
	}

	// Should not retry immediately
	if tracker.ShouldRetry(notificationID, channel) {
		t.Error("Should not retry immediately")
	}

	// Wait for backoff period
	time.Sleep(150 * time.Millisecond)

	// Should retry now
	if !tracker.ShouldRetry(notificationID, channel) {
		t.Error("Should allow retry after backoff")
	}
}

func TestDeliveryTrackerMaxRetries(t *testing.T) {
	tracker := NewDeliveryTracker()
	tracker.maxRetries = 2

	notificationID := "notification-max-retry"
	channel := "webhook"

	tracker.TrackDelivery(notificationID, channel)

	// First failure
	shouldRetry := tracker.MarkFailed(notificationID, channel, fmt.Errorf("error 1"))
	if !shouldRetry {
		t.Error("Expected retry after first failure")
	}

	// Second failure (reaches max retries)
	shouldRetry = tracker.MarkFailed(notificationID, channel, fmt.Errorf("error 2"))
	if shouldRetry {
		t.Error("Should not retry after reaching max retries")
	}

	record := tracker.GetRecord(notificationID, channel)
	if record.Status != StatusFailed {
		t.Errorf("Expected status Failed, got %v", record.Status)
	}
}

func TestNotificationService(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 2,
	})
	defer service.Shutdown(5 * time.Second)

	// Register channel
	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	// Send notification
	notification := &Notification{
		Type:       TypeInfo,
		Priority:   PriorityNormal,
		Payload:    map[string]interface{}{"message": "test notification"},
		Recipients: []string{"user1"},
		Channels:   []string{"in-app"},
	}

	err := service.Notify(notification)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Check metrics
	sent, failed := service.GetMetrics()
	if sent != 1 {
		t.Errorf("Expected 1 sent notification, got %d", sent)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed notifications, got %d", failed)
	}

	// Verify notification was received
	notifications := channel.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification in channel, got %d", len(notifications))
	}
}

func TestNotificationServiceWithTemplate(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 2,
	})
	defer service.Shutdown(5 * time.Second)

	// Register template
	err := service.RegisterTemplate("order_confirm", "Order #{{.order_id}} has been confirmed!")
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}

	// Register channel
	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	// Send notification with template
	notification := &Notification{
		Type:     TypeSuccess,
		Priority: PriorityHigh,
		Payload: map[string]interface{}{
			"order_id": "12345",
		},
		Template: "order_confirm",
		Channels: []string{"in-app"},
	}

	err = service.Notify(notification)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify rendered message
	notifications := channel.GetNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	message, ok := notifications[0].Payload["message"].(string)
	if !ok {
		t.Fatal("Message not found in payload")
	}

	expected := "Order #12345 has been confirmed!"
	if message != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, message)
	}
}

func TestNotificationServiceBatchNotify(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 3,
	})
	defer service.Shutdown(5 * time.Second)

	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	// Create batch of notifications
	notifications := []*Notification{
		{
			Type:     TypeInfo,
			Priority: PriorityLow,
			Payload:  map[string]interface{}{"message": "message 1"},
			Channels: []string{"in-app"},
		},
		{
			Type:     TypeWarning,
			Priority: PriorityNormal,
			Payload:  map[string]interface{}{"message": "message 2"},
			Channels: []string{"in-app"},
		},
		{
			Type:     TypeError,
			Priority: PriorityHigh,
			Payload:  map[string]interface{}{"message": "message 3"},
			Channels: []string{"in-app"},
		},
	}

	err := service.BatchNotify(notifications)
	if err != nil {
		t.Fatalf("Failed to batch notify: %v", err)
	}

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	sent, _ := service.GetMetrics()
	if sent != 3 {
		t.Errorf("Expected 3 sent notifications, got %d", sent)
	}
}

func TestNotificationServiceNotifyAll(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 2,
	})
	defer service.Shutdown(5 * time.Second)

	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	recipients := []string{"user1", "user2", "user3"}
	notification := &Notification{
		Type:     TypeInfo,
		Priority: PriorityNormal,
		Payload:  map[string]interface{}{"message": "broadcast message"},
		Channels: []string{"in-app"},
	}

	err := service.NotifyAll(recipients, notification)
	if err != nil {
		t.Fatalf("Failed to notify all: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify notification has all recipients
	notifications := channel.GetNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if len(notifications[0].Recipients) != 3 {
		t.Errorf("Expected 3 recipients, got %d", len(notifications[0].Recipients))
	}
}

func TestNotificationServiceConcurrency(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 5,
	})
	defer service.Shutdown(10 * time.Second)

	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	// Send many notifications concurrently
	numNotifications := 100
	var wg sync.WaitGroup

	for i := 0; i < numNotifications; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			notification := &Notification{
				Type:     TypeInfo,
				Priority: Priority(id % 4), // Mix priorities
				Payload:  map[string]interface{}{"message": fmt.Sprintf("message %d", id)},
				Channels: []string{"in-app"},
			}

			service.Notify(notification)
		}(i)
	}

	wg.Wait()

	// Wait for all notifications to be processed with retries
	maxWait := 3 * time.Second
	checkInterval := 100 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		sent, _ := service.GetMetrics()
		if sent == int64(numNotifications) {
			return // Test passed
		}
		time.Sleep(checkInterval)
	}

	sent, _ := service.GetMetrics()
	if sent != int64(numNotifications) {
		t.Errorf("Expected %d sent notifications, got %d", numNotifications, sent)
	}
}

func TestNotificationServiceShutdown(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 2,
	})

	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	// Send a notification
	notification := &Notification{
		Type:     TypeInfo,
		Priority: PriorityNormal,
		Payload:  map[string]interface{}{"message": "shutdown test"},
		Channels: []string{"in-app"},
	}

	service.Notify(notification)

	// Shutdown with timeout
	err := service.Shutdown(5 * time.Second)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify we can't send after shutdown
	err = service.Notify(notification)
	if err == nil {
		t.Error("Expected error when sending after shutdown")
	}
}

func TestSystemChannel(t *testing.T) {
	channel := NewSystemChannel()

	// Mock the command function
	var calledTitle, calledMessage string
	channel.commandFunc = func(title, message string) error {
		calledTitle = title
		calledMessage = message
		return nil
	}

	notification := &Notification{
		Type:    TypeWarning,
		Payload: map[string]interface{}{"message": "system test"},
	}

	ctx := context.Background()
	err := channel.Send(ctx, notification)
	if err != nil {
		t.Fatalf("Failed to send system notification: %v", err)
	}

	if calledTitle != "warning Notification" {
		t.Errorf("Expected title 'warning Notification', got '%s'", calledTitle)
	}

	if calledMessage != "system test" {
		t.Errorf("Expected message 'system test', got '%s'", calledMessage)
	}
}

func TestWebhookChannel(t *testing.T) {
	// This is a basic test - in production you'd use a test server
	channel := NewWebhookChannel("http://example.com/webhook")

	if channel.Name() != "webhook" {
		t.Errorf("Expected channel name 'webhook', got '%s'", channel.Name())
	}

	if !channel.SupportsRecipient("anyone") {
		t.Error("WebhookChannel should support all recipients")
	}
}

func TestPriorityOrdering(t *testing.T) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 1, // Single worker to ensure sequential processing
	})
	defer service.Shutdown(5 * time.Second)

	channel := NewInAppChannel()
	var processOrder []string
	var mu sync.Mutex

	channel.OnNotify(func(n Notification) {
		mu.Lock()
		processOrder = append(processOrder, n.ID)
		mu.Unlock()
	})

	service.RegisterChannel(channel)

	// Send notifications in reverse priority order
	notifications := []*Notification{
		{ID: "low", Priority: PriorityLow, Payload: map[string]interface{}{"message": "low"}, Channels: []string{"in-app"}},
		{ID: "normal", Priority: PriorityNormal, Payload: map[string]interface{}{"message": "normal"}, Channels: []string{"in-app"}},
		{ID: "high", Priority: PriorityHigh, Payload: map[string]interface{}{"message": "high"}, Channels: []string{"in-app"}},
		{ID: "critical", Priority: PriorityCritical, Payload: map[string]interface{}{"message": "critical"}, Channels: []string{"in-app"}},
	}

	for _, n := range notifications {
		service.Notify(n)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Verify critical was processed first
	if len(processOrder) != 4 {
		t.Fatalf("Expected 4 notifications processed, got %d", len(processOrder))
	}

	if processOrder[0] != "critical" {
		t.Errorf("Expected 'critical' first, got '%s'", processOrder[0])
	}

	if processOrder[1] != "high" {
		t.Errorf("Expected 'high' second, got '%s'", processOrder[1])
	}
}

// Benchmark tests
func BenchmarkNotificationQueue(b *testing.B) {
	queue := NewNotificationQueue()
	defer queue.Close()

	notification := &Notification{
		Priority: PriorityNormal,
		Payload:  map[string]interface{}{"message": "benchmark"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Enqueue(notification)
		queue.Dequeue()
	}
}

func BenchmarkNotificationService(b *testing.B) {
	service := NewNotificationService(NotificationServiceConfig{
		Workers: 5,
	})
	defer service.Shutdown(10 * time.Second)

	channel := NewInAppChannel()
	service.RegisterChannel(channel)

	notification := &Notification{
		Type:     TypeInfo,
		Priority: PriorityNormal,
		Payload:  map[string]interface{}{"message": "benchmark"},
		Channels: []string{"in-app"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.Notify(notification)
	}

	// Wait for processing
	time.Sleep(1 * time.Second)
}
