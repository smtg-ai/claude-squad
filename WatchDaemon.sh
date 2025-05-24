#!/bin/bash
# WATCH DAEMON - Continuous squad monitoring with tmux synchronization
# Ensures squads stay active, responsive, synchronized, and continuously improving

set -e

DAEMON_PID_FILE="/tmp/chronos_watch_daemon.pid"
LOG_FILE="/tmp/chronos_watch_daemon.log"

echo "🛡️ WATCH DAEMON: Squad Monitoring System Initializing"

# Check if daemon already running
if [ -f "$DAEMON_PID_FILE" ]; then
    if kill -0 "$(cat $DAEMON_PID_FILE)" 2>/dev/null; then
        echo "⚠️ Watch daemon already running (PID: $(cat $DAEMON_PID_FILE))"
        exit 1
    else
        rm -f "$DAEMON_PID_FILE"
    fi
fi

# Save daemon PID
echo $$ > "$DAEMON_PID_FILE"

# Cleanup on exit
cleanup() {
    echo "🛑 Watch daemon shutting down..."
    rm -f "$DAEMON_PID_FILE"
    exit 0
}
trap cleanup SIGTERM SIGINT

# Logging function with tmux broadcast
log() {
    local message="[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo "$message" | tee -a "$LOG_FILE"
    
    # Broadcast log to all squads for synchronization awareness
    broadcast_daemon_message "DAEMON_LOG" "$1"
}

# Broadcast synchronization messages to all squads
broadcast_daemon_message() {
    local msg_type=$1
    local message=$2
    local timestamp=$(date '+%H:%M:%S')
    
    local sessions=($(tmux list-sessions 2>/dev/null | grep chronos_ | cut -d: -f1 || true))
    
    for session in "${sessions[@]}"; do
        if tmux has-session -t "$session" 2>/dev/null; then
            # Send coordination message via tmux display-message
            tmux display-message -t "$session" "[DAEMON:$timestamp] $msg_type: $message" 2>/dev/null || true
            
            # Inject sync awareness command
            tmux send-keys -t "$session" "" Enter 2>/dev/null || true
            tmux send-keys -t "$session" "DAEMON SYNC: $message. Coordinate with other squads via tmux messaging. Share Swift app insights and coordinate improvements." Enter 2>/dev/null || true
        fi
    done
}

# Squad health check with synchronization enforcement
check_squad_health() {
    local session=$1
    local last_activity=$(tmux capture-pane -t "$session" -p | tail -1 | grep -o '[0-9]\{2\}:[0-9]\{2\}:[0-9]\{2\}' || echo "")
    
    if [ -z "$last_activity" ]; then
        log "⚠️ $session appears inactive - sending activation pulse with sync enforcement"
        
        # Broadcast inactivity to other squads
        broadcast_daemon_message "SQUAD_INACTIVE" "$session requires coordination support"
        
        # Send activation pulse with sync requirements
        tmux send-keys -t "$session" "" Enter
        tmux send-keys -t "$session" "SYNC ACTIVATION: Continue Swift app improvements. Use tmux messaging to coordinate with other squads. Share progress, insights, and coordinate creative solutions. Research new techniques and implement improvements." Enter
        
        # Notify other squads to assist
        local sessions=($(tmux list-sessions 2>/dev/null | grep chronos_ | cut -d: -f1 || true))
        for other_session in "${sessions[@]}"; do
            if [ "$other_session" != "$session" ] && tmux has-session -t "$other_session" 2>/dev/null; then
                tmux send-keys -t "$other_session" "" Enter 2>/dev/null || true
                tmux send-keys -t "$other_session" "COORDINATION REQUEST: Squad $session needs sync support. Share relevant Swift insights and coordinate improvements via tmux messaging." Enter 2>/dev/null || true
            fi
        done
        
        return 1
    fi
    return 0
}

# Restart dead squads with synchronization
restart_squad() {
    local session=$1
    log "🔄 Restarting dead squad: $session with sync protocols"
    
    # Notify all other squads of restart
    broadcast_daemon_message "SQUAD_RESTART" "$session is being restarted - prepare for resynchronization"
    
    # Kill if exists
    tmux kill-session -t "$session" 2>/dev/null || true
    
    # Restart based on squad type
    local squad_type=$(echo "$session" | sed 's/chronos_//' | tr '[:lower:]' '[:upper:]')
    local work_dir="/Users/schizodactyl/projects/chronOS"
    
    # Create new session
    tmux new-session -d -s "$session" -c "$work_dir" "echo '🚀 $squad_type REACTIVATED'; claude"
    sleep 2
    
    # Inject appropriate prompt
    case "$squad_type" in
        "COREAGENT")
            prompt="You are the CoreAgent. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research innovative Swift patterns, SwiftUI features, and iOS capabilities. Always listen to feedback but use research to inspire creativity."
            ;;
        "ARCHITECTURE")
            prompt="You are the Architecture squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research cutting-edge Swift architecture patterns, SwiftUI navigation, state management, and app organization."
            ;;
        "PERFORMANCE")
            prompt="You are the Performance squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research Swift performance optimization, memory management, rendering efficiency, and battery optimization."
            ;;
        "TESTING")
            prompt="You are the Testing squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research Swift testing frameworks, UI testing, unit testing, and automated QA."
            ;;
        "SECURITY")
            prompt="You are the Security squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research iOS security best practices, data protection, keychain usage, and privacy features."
            ;;
        "INTEGRATION")
            prompt="You are the Integration squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research Swift package management, API integration, third-party libraries, and ecosystem tools."
            ;;
        "MIRRORSQUAD")
            prompt="You are the MirrorSquad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Mirror and coordinate all squad activities focused on Swift app enhancement."
            ;;
    esac
    
    tmux send-keys -t "$session" "/init" Enter
    sleep 1
    tmux send-keys -t "$session" "$prompt START WORKING NOW WITH TMUX SYNCHRONIZATION. Use tmux messaging to coordinate with other squads constantly. Share Swift insights, coordinate improvements, and maintain sync awareness. Research innovative Swift/iOS techniques and implement creative solutions immediately. NEVER STOP IMPROVING OR COORDINATING." Enter
    sleep 2
    tmux send-keys -t "$session" "Begin autonomous Swift app improvement cycle with mandatory tmux synchronization. Use tmux display-message and send-keys to coordinate with other squads. Research, listen to feedback, share insights, and implement creative solutions forever." Enter
    sleep 1
    
    # Send resync notification to all other squads
    broadcast_daemon_message "SQUAD_REJOINED" "$session has rejoined - initiate coordination protocols"
}

# Auto-accept prompts for all sessions with synchronization
auto_accept_all() {
    local sessions=($(tmux list-sessions 2>/dev/null | grep chronos_ | cut -d: -f1))
    
    # Broadcast auto-accept initiation
    broadcast_daemon_message "AUTO_ACCEPT_CYCLE" "Processing prompts across all squads"
    
    for session in "${sessions[@]}"; do
        tmux send-keys -t "$session" "1" Enter 2>/dev/null || true
        tmux send-keys -t "$session" "y" Enter 2>/dev/null || true
        tmux send-keys -t "$session" "S-Tab" Enter 2>/dev/null || true
        
        # Inject sync reminder
        tmux send-keys -t "$session" "" Enter 2>/dev/null || true
        tmux send-keys -t "$session" "SYNC REMINDER: Use tmux messaging to coordinate with other squads. Share Swift insights and maintain squad synchronization." Enter 2>/dev/null || true
    done
}

# Main monitoring loop with synchronization
log "🚀 Watch daemon started with TMUX synchronization (PID: $$)"

# Initialize daemon synchronization
broadcast_daemon_message "DAEMON_STARTED" "Watch daemon active - all squads maintain tmux sync protocols"

while true; do
    # Get current chronos sessions
    sessions=($(tmux list-sessions 2>/dev/null | grep chronos_ | cut -d: -f1 || true))
    
    if [ ${#sessions[@]} -eq 0 ]; then
        log "❌ No chronos sessions found - attempting to restart Boot.sh"
        cd /Users/schizodactyl/projects/chronOS/claude-squad
        ./Boot.sh > /dev/null 2>&1 &
        sleep 30
        continue
    fi
    
    # Check each squad's health
    for session in "${sessions[@]}"; do
        if ! tmux has-session -t "$session" 2>/dev/null; then
            log "💀 Squad $session died - restarting"
            restart_squad "$session"
        else
            check_squad_health "$session"
        fi
    done
    
    # Auto-accept any pending prompts
    auto_accept_all
    
    # Send periodic synchronization pulse
    broadcast_daemon_message "SYNC_PULSE" "Coordinate Swift improvements across squads"
    
    # Log status
    log "✅ Monitoring ${#sessions[@]} squads with sync: ${sessions[*]}"
    
    # Wait 30 seconds before next check
    sleep 30
done