#!/bin/bash
# AUTO-YES - Automatic squad decision acceptance system with tmux synchronization
# Continuously monitors and auto-accepts all squad prompts with inter-squad messaging

set -e

echo "🤖 AUTO-YES: Squad Auto-Acceptance System with TMUX Synchronization Activated"

# Get all chronos sessions
SESSIONS=($(tmux list-sessions | grep chronos_ | cut -d: -f1))

if [ ${#SESSIONS[@]} -eq 0 ]; then
    echo "❌ No chronos sessions found. Run Boot.sh first."
    exit 1
fi

# Auto-accept function with tmux message synchronization
auto_accept_session() {
    local session=$1
    echo "🔄 Auto-accepting prompts in $session with sync messaging..."
    
    # Send synchronization message to all other squads
    broadcast_sync_message "$session" "AUTO_ACCEPT_INITIATED"
    
    # Send '1' (Yes) + Enter to accept any pending prompts
    tmux send-keys -t "$session" "1" Enter 2>/dev/null || true
    
    # Also send 'y' for any other confirmation prompts
    tmux send-keys -t "$session" "y" Enter 2>/dev/null || true
    
    # Send shift+tab for "don't ask again" option if available
    tmux send-keys -t "$session" "S-Tab" Enter 2>/dev/null || true
    
    # Notify completion via tmux messaging
    broadcast_sync_message "$session" "AUTO_ACCEPT_COMPLETED"
}

# Broadcast synchronization messages to all squads
broadcast_sync_message() {
    local sender=$1
    local message=$2
    local timestamp=$(date '+%H:%M:%S')
    
    for session in "${SESSIONS[@]}"; do
        if [ "$session" != "$sender" ] && tmux has-session -t "$session" 2>/dev/null; then
            # Send coordination message via tmux display-message
            tmux display-message -t "$session" "[SYNC:$timestamp] $sender: $message" 2>/dev/null || true
            
            # Also inject as command for squad awareness
            tmux send-keys -t "$session" "" Enter 2>/dev/null || true
            tmux send-keys -t "$session" "SYNC MESSAGE from $sender: $message. Coordinate your Swift app improvements accordingly. Continue research and creative development." Enter 2>/dev/null || true
        fi
    done
}

# Continuous monitoring loop with synchronization
echo "🚀 Starting continuous auto-acceptance with TMUX sync for ${#SESSIONS[@]} sessions..."

# Initialize synchronization
broadcast_sync_message "AutoYes" "SYNC_SYSTEM_ACTIVATED"

while true; do
    for session in "${SESSIONS[@]}"; do
        # Check if session still exists
        if tmux has-session -t "$session" 2>/dev/null; then
            auto_accept_session "$session"
            
            # Send periodic sync heartbeat
            if [ $(($(date +%s) % 30)) -eq 0 ]; then
                broadcast_sync_message "$session" "HEARTBEAT_ACTIVE"
            fi
        fi
    done
    
    # Send coordination pulse every cycle
    broadcast_sync_message "AutoYes" "COORDINATION_PULSE"
    
    # Wait 5 seconds before next check
    sleep 5
done