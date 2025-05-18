package sync

import (
	"claude-squad/config"
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentMessage represents a message that can be shared between agents
type AgentMessage struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentState represents the state of an agent that can be shared
type AgentState struct {
	ID             string            `json:"id"`
	Title          string            `json:"title"`
	LastActive     time.Time         `json:"last_active"`
	Branch         string            `json:"branch"`
	UpdatedFiles   []string          `json:"updated_files"`
	Tags           []string          `json:"tags"`
	CustomMetadata map[string]string `json:"custom_metadata"`
}

// AgentSync handles synchronization between different agent instances
type AgentSync struct {
	config          *config.Config
	instanceID      string
	instanceTitle   string
	messageChannel  chan AgentMessage
	stateChannel    chan AgentState
	sharedDir       string
	messagesFile    string
	stateFile       string
	notifyCallback  func(AgentMessage)
	stateCallback   func(AgentState)
	active          bool
	mutex           sync.RWMutex
	cleanupInterval time.Duration
	retentionPeriod time.Duration
}

// NewAgentSync creates a new agent synchronization manager
func NewAgentSync(cfg *config.Config, instanceID, instanceTitle string) (*AgentSync, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	// Create shared directory for inter-agent communication
	sharedDir := filepath.Join(configDir, "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create shared directory: %w", err)
	}

	messagesFile := filepath.Join(sharedDir, "messages.json")
	stateFile := filepath.Join(sharedDir, "states.json")

	// Create a new agent sync with channels for messages and state updates
	return &AgentSync{
		config:          cfg,
		instanceID:      instanceID,
		instanceTitle:   instanceTitle,
		messageChannel:  make(chan AgentMessage, 100),
		stateChannel:    make(chan AgentState, 100),
		sharedDir:       sharedDir,
		messagesFile:    messagesFile,
		stateFile:       stateFile,
		active:          false,
		cleanupInterval: 1 * time.Hour,
		retentionPeriod: 24 * time.Hour,
	}, nil
}

// Start begins the synchronization process
func (a *AgentSync) Start() {
	a.mutex.Lock()
	if a.active {
		a.mutex.Unlock()
		return
	}
	a.active = true
	a.mutex.Unlock()

	// Start goroutines for message processing and state synchronization
	go a.processMessages()
	go a.processStates()
	go a.periodicCleanup()

	log.InfoLog.Printf("Agent sync started for instance %s (%s)", a.instanceID, a.instanceTitle)
}

// Stop halts the synchronization process
func (a *AgentSync) Stop() {
	a.mutex.Lock()
	a.active = false
	a.mutex.Unlock()
	log.InfoLog.Printf("Agent sync stopped for instance %s", a.instanceID)
}

// IsActive returns whether the synchronization is active
func (a *AgentSync) IsActive() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.active
}

// SendMessage broadcasts a message to all other agents
func (a *AgentSync) SendMessage(msgType, content string) error {
	message := AgentMessage{
		ID:        generateID(),
		Sender:    a.instanceID,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Add to internal channel for processing
	a.messageChannel <- message

	return nil
}

// UpdateState updates and shares the current agent state
func (a *AgentSync) UpdateState(state AgentState) error {
	// Update state ID and title
	state.ID = a.instanceID
	state.LastActive = time.Now()

	// Add to internal channel for processing
	a.stateChannel <- state

	return nil
}

// SetNotificationCallback sets a callback for new messages
func (a *AgentSync) SetNotificationCallback(callback func(AgentMessage)) {
	a.notifyCallback = callback
}

// SetStateCallback sets a callback for state changes
func (a *AgentSync) SetStateCallback(callback func(AgentState)) {
	a.stateCallback = callback
}

// GetAllMessages retrieves all messages within the retention period
func (a *AgentSync) GetAllMessages() ([]AgentMessage, error) {
	messages, err := a.loadMessages()
	if err != nil {
		return nil, err
	}

	// Filter out older messages
	cutoff := time.Now().Add(-a.retentionPeriod)
	var filtered []AgentMessage
	for _, msg := range messages {
		if msg.Timestamp.After(cutoff) {
			filtered = append(filtered, msg)
		}
	}

	return filtered, nil
}

// GetAllStates retrieves all agent states
func (a *AgentSync) GetAllStates() ([]AgentState, error) {
	return a.loadStates()
}

// GetActiveAgents returns IDs of recently active agents
func (a *AgentSync) GetActiveAgents() ([]string, error) {
	states, err := a.loadStates()
	if err != nil {
		return nil, err
	}

	// Consider agents active if updated in the last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	var active []string
	for _, state := range states {
		if state.LastActive.After(cutoff) {
			active = append(active, state.ID)
		}
	}

	return active, nil
}

// processMessages handles message synchronization
func (a *AgentSync) processMessages() {
	ticker := time.NewTicker(time.Duration(a.config.AgentSyncInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case msg := <-a.messageChannel:
			a.mutex.RLock()
			active := a.active
			a.mutex.RUnlock()

			if !active {
				return
			}

			// Save the message to the shared file
			if err := a.saveMessage(msg); err != nil {
				log.ErrorLog.Printf("Failed to save message: %v", err)
			}

		case <-ticker.C:
			a.mutex.RLock()
			active := a.active
			a.mutex.RUnlock()

			if !active {
				return
			}

			// Check for new messages and notify
			if err := a.checkNewMessages(); err != nil {
				log.ErrorLog.Printf("Failed to check new messages: %v", err)
			}
		}
	}
}

// processStates handles state synchronization
func (a *AgentSync) processStates() {
	ticker := time.NewTicker(time.Duration(a.config.AgentSyncInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case state := <-a.stateChannel:
			a.mutex.RLock()
			active := a.active
			a.mutex.RUnlock()

			if !active {
				return
			}

			// Save the state to the shared file
			if err := a.saveState(state); err != nil {
				log.ErrorLog.Printf("Failed to save state: %v", err)
			}

		case <-ticker.C:
			a.mutex.RLock()
			active := a.active
			a.mutex.RUnlock()

			if !active {
				return
			}

			// Check for state changes and notify
			if err := a.checkStateChanges(); err != nil {
				log.ErrorLog.Printf("Failed to check state changes: %v", err)
			}
		}
	}
}

// periodicCleanup performs periodic cleanup of old messages
func (a *AgentSync) periodicCleanup() {
	ticker := time.NewTicker(a.cleanupInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		
		a.mutex.RLock()
		active := a.active
		a.mutex.RUnlock()

		if !active {
			return
		}

		// Clean up old messages
		if err := a.cleanupOldMessages(); err != nil {
			log.ErrorLog.Printf("Failed to clean up old messages: %v", err)
		}
	}
}

// saveMessage adds a message to the shared messages file
func (a *AgentSync) saveMessage(msg AgentMessage) error {
	messages, err := a.loadMessages()
	if err != nil {
		return err
	}

	// Add the new message
	messages = append(messages, msg)

	// Save back to file
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	return os.WriteFile(a.messagesFile, data, 0644)
}

// saveState updates the agent state in the shared states file
func (a *AgentSync) saveState(state AgentState) error {
	states, err := a.loadStates()
	if err != nil {
		return err
	}

	// Update or add the state
	found := false
	for i, s := range states {
		if s.ID == state.ID {
			states[i] = state
			found = true
			break
		}
	}

	if !found {
		states = append(states, state)
	}

	// Save back to file
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal states: %w", err)
	}

	return os.WriteFile(a.stateFile, data, 0644)
}

// loadMessages loads all messages from the shared file
func (a *AgentSync) loadMessages() ([]AgentMessage, error) {
	// If file doesn't exist, return empty slice
	if _, err := os.Stat(a.messagesFile); os.IsNotExist(err) {
		return []AgentMessage{}, nil
	}

	data, err := os.ReadFile(a.messagesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages file: %w", err)
	}

	var messages []AgentMessage
	if len(data) == 0 {
		return []AgentMessage{}, nil
	}

	if err := json.Unmarshal(data, &messages); err != nil {
		log.WarningLog.Printf("Failed to parse messages file, starting with empty messages: %v", err)
		return []AgentMessage{}, nil
	}

	return messages, nil
}

// loadStates loads all agent states from the shared file
func (a *AgentSync) loadStates() ([]AgentState, error) {
	// If file doesn't exist, return empty slice
	if _, err := os.Stat(a.stateFile); os.IsNotExist(err) {
		return []AgentState{}, nil
	}

	data, err := os.ReadFile(a.stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read states file: %w", err)
	}

	var states []AgentState
	if len(data) == 0 {
		return []AgentState{}, nil
	}

	if err := json.Unmarshal(data, &states); err != nil {
		log.WarningLog.Printf("Failed to parse states file, starting with empty states: %v", err)
		return []AgentState{}, nil
	}

	return states, nil
}

// checkNewMessages checks for new messages and notifies callbacks
func (a *AgentSync) checkNewMessages() error {
	if a.notifyCallback == nil {
		return nil
	}

	messages, err := a.loadMessages()
	if err != nil {
		return err
	}

	// Filter for messages from other agents in the last interval
	cutoff := time.Now().Add(-time.Duration(a.config.AgentSyncInterval) * time.Millisecond * 2)
	for _, msg := range messages {
		if msg.Sender != a.instanceID && msg.Timestamp.After(cutoff) {
			a.notifyCallback(msg)
		}
	}

	return nil
}

// checkStateChanges checks for state changes and notifies callbacks
func (a *AgentSync) checkStateChanges() error {
	if a.stateCallback == nil {
		return nil
	}

	states, err := a.loadStates()
	if err != nil {
		return err
	}

	// Notify about other agents' states
	for _, state := range states {
		if state.ID != a.instanceID {
			a.stateCallback(state)
		}
	}

	return nil
}

// cleanupOldMessages removes messages older than the retention period
func (a *AgentSync) cleanupOldMessages() error {
	messages, err := a.loadMessages()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-a.retentionPeriod)
	var filtered []AgentMessage
	for _, msg := range messages {
		if msg.Timestamp.After(cutoff) {
			filtered = append(filtered, msg)
		}
	}

	// If we removed any messages, save the filtered list
	if len(filtered) < len(messages) {
		data, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal filtered messages: %w", err)
		}

		return os.WriteFile(a.messagesFile, data, 0644)
	}

	return nil
}

// generateID creates a unique ID for messages
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}