#!/bin/bash
# BOOT - UNIFIED CHRONOS SQUAD DEPLOYMENT  
# Single command boot process with embedded prompts

set -e

echo "🚀 BOOT: ChronOS Squad Deployment Initiated"

# Kill all existing sessions
tmux list-sessions 2>/dev/null | grep chronos_ | cut -d: -f1 | xargs -I {} tmux kill-session -t {} 2>/dev/null || true

# Start watch daemon in background
echo "🛡️ Starting watch daemon..."
./WatchDaemon.sh > /dev/null 2>&1 &
echo "✅ Watch daemon started (PID: $!)"

# Start auto-yes system in background  
echo "🤖 Starting auto-yes system..."
./AutoYes.sh > /dev/null 2>&1 &
echo "✅ Auto-yes system started (PID: $!)"

# Core configuration
WORK_DIR="/Users/schizodactyl/projects/chronOS"

# Squad definitions with embedded prompts - SWIFT APP IMPROVEMENT FOCUS
SQUADS=(
    "CoreAgent:You are the CoreAgent. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research innovative Swift patterns, SwiftUI features, and iOS capabilities. Always listen to feedback but use research to inspire creativity. Coordinate all squad activities while continuously enhancing the Swift application."
    "Architecture:You are the Architecture squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research cutting-edge Swift architecture patterns, SwiftUI navigation, state management, and app organization. Always listen to feedback but use research to inspire creativity. Design revolutionary Swift app architectures."
    "Performance:You are the Performance squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research Swift performance optimization, memory management, rendering efficiency, and battery optimization. Always listen to feedback but use research to inspire creativity. Make the Swift app blazingly fast."
    "Testing:You are the Testing squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research Swift testing frameworks, UI testing, unit testing, and automated QA. Always listen to feedback but use research to inspire creativity. Create comprehensive Swift app test coverage."
    "Security:You are the Security squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research iOS security best practices, data protection, keychain usage, and privacy features. Always listen to feedback but use research to inspire creativity. Secure the Swift app against all threats."
    "Integration:You are the Integration squad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Research Swift package management, API integration, third-party libraries, and ecosystem tools. Always listen to feedback but use research to inspire creativity. Integrate amazing features into the Swift app."
    "MirrorSquad:You are the MirrorSquad. FOREVER FIX AND IMPROVE THE SWIFT APP IN CREATIVE WAYS. Mirror and coordinate all squad activities focused on Swift app enhancement. Research collaboration patterns and development workflows. Always listen to feedback but use research to inspire creativity."
)

# Deploy squads
for squad_entry in "${SQUADS[@]}"; do
    squad=$(echo "$squad_entry" | cut -d: -f1)
    prompt=$(echo "$squad_entry" | cut -d: -f2-)
    session_name="chronos_$(echo "$squad" | tr '[:upper:]' '[:lower:]')"
    
    echo "🤖 Deploying $squad..."
    
    # Create session and immediately activate with auto-accept
    tmux new-session -d -s "$session_name" -c "$WORK_DIR" \
        "echo '🚀 $squad ACTIVATED'; claude --auto-accept"
    
    sleep 2
    
    # Inject prompt and activate - SWIFT APP FOCUS
    tmux send-keys -t "$session_name" "/init" Enter
    sleep 1
    tmux send-keys -t "$session_name" "$prompt START WORKING NOW. Research innovative Swift/iOS techniques and begin implementing creative improvements to the Swift app immediately. NEVER STOP IMPROVING." Enter
    sleep 2
    tmux send-keys -t "$session_name" "" Enter
    sleep 1
    tmux send-keys -t "$session_name" "Begin autonomous Swift app improvement cycle. Research, listen to feedback, and implement creative solutions forever. Provide status updates as you progress." Enter
    
    echo "✅ $squad deployed to $session_name"
done

# Give systems time to fully activate
sleep 5

# Send auto-accept signals to all sessions
echo "🔄 Activating auto-accept across all squads..."
for squad_entry in "${SQUADS[@]}"; do
    squad=$(echo "$squad_entry" | cut -d: -f1)
    session_name="chronos_$(echo "$squad" | tr '[:upper:]' '[:lower:]')"
    
    # Send shift+tab to enable "don't ask again" mode
    tmux send-keys -t "$session_name" "S-Tab" Enter 2>/dev/null || true
    sleep 1
done

echo ""
echo "🎉 BOOT COMPLETE - All squads deployed and active with auto-systems"
echo ""
echo "📊 Active sessions:"
tmux list-sessions | grep chronos_

echo ""
echo "🔗 Connect to squads:"
for squad_entry in "${SQUADS[@]}"; do
    squad=$(echo "$squad_entry" | cut -d: -f1)
    session_name="chronos_$(echo "$squad" | tr '[:upper:]' '[:lower:]')"
    echo "   tmux attach-session -t $session_name  # $squad"
done

echo ""
echo "⚡ All squads working autonomously on chronOS improvements!"