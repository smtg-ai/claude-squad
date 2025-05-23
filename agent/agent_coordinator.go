package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// AgentCoordinator implements the chronOS agent loop integration
type AgentCoordinator struct {
	squadID           string
	vectorClock       *VectorClock
	messageBuffer     []Message
	messageBufferMu   sync.RWMutex
	sharedRegistry    *SharedKnowledgeRegistry
	syncCoordinator   *SynchronizationCoordinator
	whiplashProtocol  *WhiplashProtocol
	autonomous        bool
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewAgentCoordinator creates a new agent coordinator with full integration
func NewAgentCoordinator(squadID string) *AgentCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	
	ac := &AgentCoordinator{
		squadID:          squadID,
		vectorClock:      NewVectorClock(squadID),
		messageBuffer:    make([]Message, 0, 1000),
		sharedRegistry:   NewSharedKnowledgeRegistry(),
		syncCoordinator:  NewSynchronizationCoordinator(squadID),
		whiplashProtocol: NewWhiplashProtocol(),
		autonomous:       true,
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// Start autonomous operation loops
	go ac.runAgentLoop()
	go ac.runSynchronizationLoop()
	go ac.runMessageBusProcessor()
	
	return ac
}

// Agent Loop Implementation (OODA Pattern)
func (ac *AgentCoordinator) runAgentLoop() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz agent loop
	defer ticker.Stop()
	
	for {
		select {
		case <-ac.ctx.Done():
			return
		case <-ticker.C:
			ac.executeOODALoop()
		}
	}
}

// executeOODALoop implements the Observe-Orient-Decide-Act pattern
func (ac *AgentCoordinator) executeOODALoop() {
	// OBSERVE: Gather environment state
	observations := ac.observe()
	
	// ORIENT: Process and contextualize
	orientation := ac.orient(observations)
	
	// DECIDE: Determine action
	decision := ac.decide(orientation)
	
	// ACT: Execute action
	ac.act(decision)
	
	// Update vector clock
	ac.vectorClock.Tick()
}

// Observe phase - gather information from environment
func (ac *AgentCoordinator) observe() map[string]interface{} {
	observations := make(map[string]interface{})
	
	// Get shared knowledge state
	observations["shared_knowledge"] = ac.sharedRegistry.GetAllEntries()
	
	// Get pending messages
	ac.messageBufferMu.RLock()
	observations["pending_messages"] = len(ac.messageBuffer)
	ac.messageBufferMu.RUnlock()
	
	// Get sync status
	observations["sync_status"] = ac.syncCoordinator.GetStatus()
	
	// Get vector clock state
	observations["vector_clock"] = ac.vectorClock.GetState()
	
	return observations
}

// Orient phase - process and contextualize information
func (ac *AgentCoordinator) orient(observations map[string]interface{}) map[string]interface{} {
	orientation := make(map[string]interface{})
	
	// Analyze message queue depth
	pendingMessages := observations["pending_messages"].(int)
	orientation["message_pressure"] = pendingMessages > 100
	
	// Check if sync is needed
	syncStatus := observations["sync_status"].(SyncStatus)
	orientation["sync_required"] = syncStatus.LastSync.Before(time.Now().Add(-5 * time.Minute))
	
	// Determine squad coordination needs
	orientation["requires_coordination"] = ac.shouldCoordinate(observations)
	
	return orientation
}

// Decide phase - determine appropriate action
func (ac *AgentCoordinator) decide(orientation map[string]interface{}) Action {
	// Priority-based decision making
	if orientation["message_pressure"].(bool) {
		return Action{Type: "process_messages", Priority: 1}
	}
	
	if orientation["sync_required"].(bool) {
		return Action{Type: "force_sync", Priority: 2}
	}
	
	if orientation["requires_coordination"].(bool) {
		return Action{Type: "coordinate_squads", Priority: 3}
	}
	
	return Action{Type: "optimize", Priority: 10}
}

// Act phase - execute the chosen action
func (ac *AgentCoordinator) act(action Action) {
	switch action.Type {
	case "process_messages":
		ac.processMessageBatch()
	case "force_sync":
		ac.forceSynchronization()
	case "coordinate_squads":
		ac.coordinateWithSquads()
	case "optimize":
		ac.performOptimization()
	}
}

// Message Bus Implementation
func (ac *AgentCoordinator) runMessageBusProcessor() {
	ticker := time.NewTicker(50 * time.Millisecond) // 20Hz message processing
	defer ticker.Stop()
	
	for {
		select {
		case <-ac.ctx.Done():
			return
		case <-ticker.C:
			ac.processMessageBatch()
		}
	}
}

// SendMessage sends a message through the bus with whiplash compression
func (ac *AgentCoordinator) SendMessage(to string, content string) error {
	// Compress using Whiplash protocol
	compressed, err := ac.whiplashProtocol.Compress(content)
	if err != nil {
		return fmt.Errorf("whiplash compression failed: %v", err)
	}
	
	message := Message{
		ID:        generateMessageID(),
		From:      ac.squadID,
		To:        to,
		Content:   compressed,
		Timestamp: ac.vectorClock.Now(),
		Type:      "whiplash",
	}
	
	ac.messageBufferMu.Lock()
	ac.messageBuffer = append(ac.messageBuffer, message)
	ac.messageBufferMu.Unlock()
	
	// Store in shared registry for persistence
	key := fmt.Sprintf("message:%s", message.ID)
	ac.sharedRegistry.Put(key, message, ac.vectorClock.Now())
	
	return nil
}

// SendCommand sends a command with automatic compression and routing
func (ac *AgentCoordinator) SendCommand(directive, operator, target string, params map[string]string) error {
	command := WhiplashCommand{
		Directive: directive,
		Operator:  operator,
		Target:    target,
		Params:    params,
		Timestamp: ac.vectorClock.Now(),
	}
	
	compressed := ac.whiplashProtocol.CompressCommand(command)
	return ac.SendMessage("*", compressed) // Broadcast command
}

// processMessageBatch processes pending messages in batches
func (ac *AgentCoordinator) processMessageBatch() {
	ac.messageBufferMu.Lock()
	if len(ac.messageBuffer) == 0 {
		ac.messageBufferMu.Unlock()
		return
	}
	
	// Process up to 50 messages per batch
	batchSize := 50
	if len(ac.messageBuffer) < batchSize {
		batchSize = len(ac.messageBuffer)
	}
	
	batch := make([]Message, batchSize)
	copy(batch, ac.messageBuffer[:batchSize])
	ac.messageBuffer = ac.messageBuffer[batchSize:]
	ac.messageBufferMu.Unlock()
	
	// Process batch concurrently
	for _, msg := range batch {
		go ac.processMessage(msg)
	}
}

// processMessage handles individual message processing
func (ac *AgentCoordinator) processMessage(msg Message) {
	switch msg.Type {
	case "whiplash":
		// Decompress whiplash message
		decompressed, err := ac.whiplashProtocol.Decompress(msg.Content)
		if err != nil {
			log.Printf("Failed to decompress whiplash message: %v", err)
			return
		}
		ac.handleDecompressedMessage(msg.From, decompressed)
	case "command":
		ac.handleCommand(msg)
	case "sync":
		ac.handleSyncMessage(msg)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// Synchronization Implementation
func (ac *AgentCoordinator) runSynchronizationLoop() {
	ticker := time.NewTicker(30 * time.Second) // Sync every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ac.ctx.Done():
			return
		case <-ticker.C:
			if ac.autonomous {
				ac.performAutoSync()
			}
		}
	}
}

// forceSynchronization performs immediate synchronization with all squads
func (ac *AgentCoordinator) forceSynchronization() {
	log.Printf("Force synchronization initiated by squad %s", ac.squadID)
	
	// Update vector clock
	ac.vectorClock.Tick()
	
	// Sync shared knowledge registry
	ac.syncCoordinator.SyncSharedKnowledge()
	
	// Sync git repository
	ac.syncCoordinator.SyncGitRepository()
	
	// Broadcast sync completion
	ac.SendMessage("*", fmt.Sprintf("SYNC_COMPLETE:%s:%d", ac.squadID, ac.vectorClock.Now().Logical))
}

// performAutoSync performs automatic background synchronization
func (ac *AgentCoordinator) performAutoSync() {
	// Check if sync is needed based on conflict detection
	if ac.syncCoordinator.HasConflicts() {
		ac.forceSynchronization()
	}
}

// Squad Coordination
func (ac *AgentCoordinator) coordinateWithSquads() {
	// Get list of active squads
	squads := ac.sharedRegistry.GetActiveSquads()
	
	for _, squad := range squads {
		if squad != ac.squadID {
			// Send coordination message
			ac.SendCommand("COORDINATE", "MIRROR", squad, map[string]string{
				"source":    ac.squadID,
				"timestamp": fmt.Sprintf("%d", ac.vectorClock.Now().Logical),
			})
		}
	}
}

// shouldCoordinate determines if squad coordination is needed
func (ac *AgentCoordinator) shouldCoordinate(observations map[string]interface{}) bool {
	// Check for conflicting operations
	if ac.syncCoordinator.HasConflicts() {
		return true
	}
	
	// Check message queue depth across squads
	return ac.sharedRegistry.GetGlobalMessageQueueDepth() > 1000
}

// performOptimization runs continuous improvement algorithms
func (ac *AgentCoordinator) performOptimization() {
	// Optimize vector clock resolution
	ac.vectorClock.Optimize()
	
	// Clean up processed messages
	ac.sharedRegistry.Cleanup()
	
	// Optimize protocol compression ratios
	ac.whiplashProtocol.OptimizeCompression()
}

// handleDecompressedMessage processes decompressed whiplash messages
func (ac *AgentCoordinator) handleDecompressedMessage(from, content string) {
	log.Printf("Received message from %s: %s", from, content)
	
	// Parse for commands
	if command, ok := ac.whiplashProtocol.ParseCommand(content); ok {
		ac.executeWhiplashCommand(command)
	}
}

// executeWhiplashCommand executes a whiplash protocol command
func (ac *AgentCoordinator) executeWhiplashCommand(cmd WhiplashCommand) {
	log.Printf("Executing whiplash command: %s %s %s", cmd.Directive, cmd.Operator, cmd.Target)
	
	switch cmd.Directive {
	case "SYNC":
		ac.forceSynchronization()
	case "COORDINATE":
		ac.coordinateWithSquads()
	case "OPTIMIZE":
		ac.performOptimization()
	case "STATUS":
		ac.broadcastStatus()
	}
}

// broadcastStatus sends current agent status to all squads
func (ac *AgentCoordinator) broadcastStatus() {
	status := map[string]interface{}{
		"squad_id":      ac.squadID,
		"vector_clock":  ac.vectorClock.GetState(),
		"message_queue": len(ac.messageBuffer),
		"sync_status":   ac.syncCoordinator.GetStatus(),
		"autonomous":    ac.autonomous,
	}
	
	statusJSON, _ := json.Marshal(status)
	ac.SendMessage("*", string(statusJSON))
}

// GetStatus returns current coordinator status
func (ac *AgentCoordinator) GetStatus() map[string]interface{} {
	ac.messageBufferMu.RLock()
	messageCount := len(ac.messageBuffer)
	ac.messageBufferMu.RUnlock()
	
	return map[string]interface{}{
		"squad_id":           ac.squadID,
		"vector_clock":       ac.vectorClock.GetState(),
		"pending_messages":   messageCount,
		"sync_status":        ac.syncCoordinator.GetStatus(),
		"autonomous_active":  ac.autonomous,
		"uptime":            time.Since(ac.vectorClock.StartTime),
	}
}

// Shutdown gracefully stops the agent coordinator
func (ac *AgentCoordinator) Shutdown() {
	log.Printf("Shutting down agent coordinator for squad %s", ac.squadID)
	ac.cancel()
}

// Types

type Action struct {
	Type     string
	Priority int
	Params   map[string]interface{}
}

type Message struct {
	ID        string
	From      string
	To        string
	Content   string
	Timestamp VectorClockTimestamp
	Type      string
}

type WhiplashCommand struct {
	Directive string
	Operator  string
	Target    string
	Params    map[string]string
	Timestamp VectorClockTimestamp
}

// Helper function to generate unique message IDs
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}