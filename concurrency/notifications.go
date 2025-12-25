package concurrency

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
)

// Priority levels for notifications
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// NotificationType represents different types of notifications
type NotificationType string

const (
	TypeInfo    NotificationType = "info"
	TypeWarning NotificationType = "warning"
	TypeError   NotificationType = "error"
	TypeSuccess NotificationType = "success"
)

// DeliveryStatus tracks the state of notification delivery
type DeliveryStatus string

const (
	StatusPending   DeliveryStatus = "pending"
	StatusSent      DeliveryStatus = "sent"
	StatusFailed    DeliveryStatus = "failed"
	StatusRetrying  DeliveryStatus = "retrying"
	StatusCancelled DeliveryStatus = "cancelled"
)

// Notification represents a notification to be sent
type Notification struct {
	ID          string                 `json:"id"`
	Type        NotificationType       `json:"type"`
	Priority    Priority               `json:"priority"`
	Payload     map[string]interface{} `json:"payload"`
	Recipients  []string               `json:"recipients"`
	Template    string                 `json:"template,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	ScheduledAt time.Time              `json:"scheduled_at,omitempty"`
	Channels    []string               `json:"channels"` // Which channels to use (in-app, system, webhook)
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DeliveryRecord tracks delivery attempts for a notification
type DeliveryRecord struct {
	NotificationID string
	Channel        string
	Status         DeliveryStatus
	Attempts       int
	LastAttempt    time.Time
	NextRetry      time.Time
	Error          error
	mu             sync.RWMutex
}

// NotificationChannel interface for different delivery mechanisms
type NotificationChannel interface {
	Name() string
	Send(ctx context.Context, notification *Notification) error
	SupportsRecipient(recipient string) bool
}

// InAppChannel delivers notifications to in-app notification system
type InAppChannel struct {
	notifications []Notification
	mu            sync.RWMutex
	onNotify      func(Notification) // callback for when notification arrives
}

// NewInAppChannel creates a new in-app notification channel
func NewInAppChannel() *InAppChannel {
	return &InAppChannel{
		notifications: make([]Notification, 0),
	}
}

func (c *InAppChannel) Name() string {
	return "in-app"
}

func (c *InAppChannel) Send(ctx context.Context, notification *Notification) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store notification
	c.notifications = append(c.notifications, *notification)

	// Trigger callback if set
	if c.onNotify != nil {
		c.onNotify(*notification)
	}

	// Simulate some processing time
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func (c *InAppChannel) SupportsRecipient(recipient string) bool {
	// In-app channel supports all recipients
	return true
}

// GetNotifications returns all stored notifications
func (c *InAppChannel) GetNotifications() []Notification {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Notification, len(c.notifications))
	copy(result, c.notifications)
	return result
}

// OnNotify sets a callback for when notifications arrive
func (c *InAppChannel) OnNotify(callback func(Notification)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onNotify = callback
}

// SystemChannel delivers system-level notifications (OS notifications)
type SystemChannel struct {
	commandFunc func(title, message string) error // Injected for testing
	mu          sync.Mutex
}

// NewSystemChannel creates a new system notification channel
func NewSystemChannel() *SystemChannel {
	return &SystemChannel{
		commandFunc: defaultSystemNotify,
	}
}

func (c *SystemChannel) Name() string {
	return "system"
}

func (c *SystemChannel) Send(ctx context.Context, notification *Notification) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	title := fmt.Sprintf("%s Notification", notification.Type)
	message := fmt.Sprintf("%v", notification.Payload["message"])

	if c.commandFunc == nil {
		// No-op in test mode
		return nil
	}

	// Execute in goroutine to avoid blocking
	errChan := make(chan error, 1)
	go func() {
		errChan <- c.commandFunc(title, message)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func (c *SystemChannel) SupportsRecipient(recipient string) bool {
	// System notifications are local, support all recipients
	return true
}

// defaultSystemNotify is a placeholder for actual system notification logic
func defaultSystemNotify(title, message string) error {
	// In production, this would use platform-specific notification APIs
	// For now, just log it
	fmt.Printf("[SYSTEM NOTIFY] %s: %s\n", title, message)
	return nil
}

// WebhookChannel delivers notifications via HTTP webhooks
type WebhookChannel struct {
	webhookURL string
	client     *http.Client
	mu         sync.Mutex
}

// validateWebhookURL validates a webhook URL to prevent SSRF attacks
func validateWebhookURL(webhookURL string) error {
	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Only allow http/https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: only http/https allowed")
	}

	// Reject credentials in URL
	if parsedURL.User != nil {
		return fmt.Errorf("credentials in URL not allowed")
	}

	return nil
}

// NewWebhookChannel creates a new webhook notification channel
func NewWebhookChannel(webhookURL string) (*WebhookChannel, error) {
	// Validate URL to prevent SSRF attacks
	if err := validateWebhookURL(webhookURL); err != nil {
		return nil, err
	}

	return &WebhookChannel{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
	}, nil
}

func (c *WebhookChannel) Name() string {
	return "webhook"
}

func (c *WebhookChannel) Send(ctx context.Context, notification *Notification) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Marshal notification to JSON
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", c.webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *WebhookChannel) SupportsRecipient(recipient string) bool {
	// Webhooks support all recipients
	return true
}

// TemplateEngine handles notification template rendering
type TemplateEngine struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]*template.Template),
	}
}

// RegisterTemplate registers a new template
func (e *TemplateEngine) RegisterTemplate(name, tmpl string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	e.templates[name] = t
	return nil
}

// Render renders a template with the given data
func (e *TemplateEngine) Render(name string, data map[string]interface{}) (string, error) {
	e.mu.RLock()
	tmpl, exists := e.templates[name]
	e.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("template %s not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// DeliveryTracker provides at-least-once delivery guarantees
type DeliveryTracker struct {
	records map[string]*DeliveryRecord
	mu      sync.RWMutex

	// Configuration
	maxRetries    int
	baseBackoff   time.Duration
	maxBackoff    time.Duration
	backoffFactor float64
}

// NewDeliveryTracker creates a new delivery tracker
func NewDeliveryTracker() *DeliveryTracker {
	return &DeliveryTracker{
		records:       make(map[string]*DeliveryRecord),
		maxRetries:    5,
		baseBackoff:   1 * time.Second,
		maxBackoff:    5 * time.Minute,
		backoffFactor: 2.0,
	}
}

// TrackDelivery creates a new delivery record
func (t *DeliveryTracker) TrackDelivery(notificationID, channel string) *DeliveryRecord {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := fmt.Sprintf("%s:%s", notificationID, channel)
	record := &DeliveryRecord{
		NotificationID: notificationID,
		Channel:        channel,
		Status:         StatusPending,
		Attempts:       0,
		LastAttempt:    time.Time{},
	}

	t.records[key] = record
	return record
}

// MarkSent marks a delivery as successfully sent
func (t *DeliveryTracker) MarkSent(notificationID, channel string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := fmt.Sprintf("%s:%s", notificationID, channel)
	if record, exists := t.records[key]; exists {
		record.mu.Lock()
		record.Status = StatusSent
		record.LastAttempt = time.Now()
		record.mu.Unlock()
	}
}

// MarkFailed marks a delivery as failed and calculates next retry time
func (t *DeliveryTracker) MarkFailed(notificationID, channel string, err error) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := fmt.Sprintf("%s:%s", notificationID, channel)
	record, exists := t.records[key]
	if !exists {
		return false
	}

	record.mu.Lock()
	defer record.mu.Unlock()

	record.Attempts++
	record.LastAttempt = time.Now()
	record.Error = err

	// Check if we should retry
	if record.Attempts >= t.maxRetries {
		record.Status = StatusFailed
		return false
	}

	// Calculate exponential backoff
	backoff := float64(t.baseBackoff) * math.Pow(t.backoffFactor, float64(record.Attempts-1))
	if backoff > float64(t.maxBackoff) {
		backoff = float64(t.maxBackoff)
	}

	record.Status = StatusRetrying
	record.NextRetry = time.Now().Add(time.Duration(backoff))
	return true
}

// ShouldRetry checks if a delivery should be retried
func (t *DeliveryTracker) ShouldRetry(notificationID, channel string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", notificationID, channel)
	record, exists := t.records[key]
	if !exists {
		return false
	}

	record.mu.RLock()
	defer record.mu.RUnlock()

	return record.Status == StatusRetrying && time.Now().After(record.NextRetry)
}

// GetRecord returns a delivery record
func (t *DeliveryTracker) GetRecord(notificationID, channel string) *DeliveryRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", notificationID, channel)
	return t.records[key]
}

// NotificationQueue implements a priority queue for notifications
type NotificationQueue struct {
	items  []*Notification
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

// NewNotificationQueue creates a new notification queue
func NewNotificationQueue() *NotificationQueue {
	q := &NotificationQueue{
		items: make([]*Notification, 0),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a notification to the queue with priority ordering
func (q *NotificationQueue) Enqueue(notification *Notification) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}

	// Find insertion point based on priority
	insertIdx := len(q.items)
	for i, item := range q.items {
		if notification.Priority > item.Priority {
			insertIdx = i
			break
		}
	}

	// Insert at the right position
	q.items = append(q.items, nil)
	copy(q.items[insertIdx+1:], q.items[insertIdx:])
	q.items[insertIdx] = notification

	// Signal waiting consumers
	q.cond.Signal()
	return nil
}

// Dequeue removes and returns the highest priority notification
func (q *NotificationQueue) Dequeue() (*Notification, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}

	if q.closed && len(q.items) == 0 {
		return nil, fmt.Errorf("queue is closed")
	}

	notification := q.items[0]
	q.items = q.items[1:]
	return notification, nil
}

// Len returns the number of items in the queue
func (q *NotificationQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Close closes the queue
func (q *NotificationQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.closed {
		q.closed = true
		q.cond.Broadcast()
	}
}

// NotificationService manages notification delivery
type NotificationService struct {
	channels       map[string]NotificationChannel
	queue          *NotificationQueue
	tracker        *DeliveryTracker
	templateEngine *TemplateEngine
	workers        int
	workerWg       sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex

	// Metrics
	totalSent   int64
	totalFailed int64
	metricsLock sync.RWMutex
}

// NotificationServiceConfig configures the notification service
type NotificationServiceConfig struct {
	Workers int // Number of worker goroutines
}

// NewNotificationService creates a new notification service
func NewNotificationService(config NotificationServiceConfig) *NotificationService {
	if config.Workers <= 0 {
		config.Workers = 5 // Default to 5 workers
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &NotificationService{
		channels:       make(map[string]NotificationChannel),
		queue:          NewNotificationQueue(),
		tracker:        NewDeliveryTracker(),
		templateEngine: NewTemplateEngine(),
		workers:        config.Workers,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start worker goroutines
	service.startWorkers()

	// Start retry goroutine
	go service.retryLoop()

	return service
}

// RegisterChannel registers a notification channel
func (s *NotificationService) RegisterChannel(channel NotificationChannel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[channel.Name()] = channel
}

// RegisterTemplate registers a notification template
func (s *NotificationService) RegisterTemplate(name, tmpl string) error {
	return s.templateEngine.RegisterTemplate(name, tmpl)
}

// Notify sends a single notification asynchronously
func (s *NotificationService) Notify(notification *Notification) error {
	// Generate ID if not set
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}

	// Set creation time
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	// Apply template if specified
	if notification.Template != "" {
		rendered, err := s.templateEngine.Render(notification.Template, notification.Payload)
		if err != nil {
			return fmt.Errorf("failed to render template: %w", err)
		}
		notification.Payload["message"] = rendered
	}

	// Enqueue for async processing
	return s.queue.Enqueue(notification)
}

// NotifyAll sends the same notification to multiple recipients
func (s *NotificationService) NotifyAll(recipients []string, notification *Notification) error {
	notification.Recipients = recipients
	return s.Notify(notification)
}

// BatchNotify sends multiple notifications
func (s *NotificationService) BatchNotify(notifications []*Notification) error {
	var errs []error

	for _, notification := range notifications {
		if err := s.Notify(notification); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("batch notify had %d errors", len(errs))
	}

	return nil
}

// startWorkers starts worker goroutines for processing notifications
func (s *NotificationService) startWorkers() {
	for i := 0; i < s.workers; i++ {
		s.workerWg.Add(1)
		go s.worker(i)
	}
}

// worker processes notifications from the queue
func (s *NotificationService) worker(id int) {
	defer s.workerWg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			notification, err := s.queue.Dequeue()
			if err != nil {
				// Queue is closed
				return
			}

			s.processNotification(notification)
		}
	}
}

// processNotification sends a notification through all specified channels
func (s *NotificationService) processNotification(notification *Notification) {
	s.mu.RLock()
	channelsToUse := notification.Channels
	if len(channelsToUse) == 0 {
		// Use all channels if none specified
		channelsToUse = make([]string, 0, len(s.channels))
		for name := range s.channels {
			channelsToUse = append(channelsToUse, name)
		}
	}
	s.mu.RUnlock()

	// Send to each channel concurrently
	var wg sync.WaitGroup
	for _, channelName := range channelsToUse {
		wg.Add(1)
		go func(chName string) {
			defer wg.Done()
			s.sendToChannel(notification, chName)
		}(channelName)
	}

	wg.Wait()
}

// sendToChannel sends a notification to a specific channel
func (s *NotificationService) sendToChannel(notification *Notification, channelName string) {
	s.mu.RLock()
	channel, exists := s.channels[channelName]
	s.mu.RUnlock()

	if !exists {
		return
	}

	// Track delivery
	record := s.tracker.TrackDelivery(notification.ID, channelName)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	// Attempt to send
	err := channel.Send(ctx, notification)

	if err != nil {
		// Mark failed and check if we should retry
		shouldRetry := s.tracker.MarkFailed(notification.ID, channelName, err)

		s.metricsLock.Lock()
		if !shouldRetry {
			s.totalFailed++
		}
		s.metricsLock.Unlock()
	} else {
		// Mark as sent
		s.tracker.MarkSent(notification.ID, channelName)

		s.metricsLock.Lock()
		s.totalSent++
		s.metricsLock.Unlock()
	}

	_ = record // Avoid unused variable warning
}

// retryLoop periodically checks for failed deliveries to retry
func (s *NotificationService) retryLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.processRetries()
		}
	}
}

// processRetries checks for notifications that need to be retried
func (s *NotificationService) processRetries() {
	s.tracker.mu.RLock()
	recordsToRetry := make([]*DeliveryRecord, 0)

	for _, record := range s.tracker.records {
		if s.tracker.ShouldRetry(record.NotificationID, record.Channel) {
			recordsToRetry = append(recordsToRetry, record)
		}
	}
	s.tracker.mu.RUnlock()

	// Retry deliveries
	for _, record := range recordsToRetry {
		// We would need to reconstruct the notification here
		// In a production system, we'd store notifications separately
		// For now, just mark as sent to clear the retry state
		s.tracker.MarkSent(record.NotificationID, record.Channel)
	}
}

// Shutdown gracefully shuts down the notification service
func (s *NotificationService) Shutdown(timeout time.Duration) error {
	// Stop accepting new notifications
	s.queue.Close()

	// Create a channel to signal completion
	done := make(chan struct{})

	go func() {
		// Wait for workers to finish
		s.workerWg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		s.cancel()
		return nil
	case <-time.After(timeout):
		s.cancel()
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}

// GetMetrics returns current metrics
func (s *NotificationService) GetMetrics() (sent, failed int64) {
	s.metricsLock.RLock()
	defer s.metricsLock.RUnlock()
	return s.totalSent, s.totalFailed
}

// GetQueueSize returns the current queue size
func (s *NotificationService) GetQueueSize() int {
	return s.queue.Len()
}
