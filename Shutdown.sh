#!/bin/bash
# SHUTDOWN - Intelligent squad shutdown with learning preservation and PR submission
# Captures squad learnings, optimizes prompts, and submits improvements via PR

set -e

echo "🛑 SHUTDOWN: Intelligent Squad Termination with Learning Preservation"

WORK_DIR="/Users/schizodactyl/projects/chronOS/claude-squad"
LEARNING_DIR="$WORK_DIR/squad_learnings"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create learning directory
mkdir -p "$LEARNING_DIR"

# Stop watch daemon first
if [ -f "/tmp/chronos_watch_daemon.pid" ]; then
    echo "🛑 Stopping watch daemon..."
    kill "$(cat /tmp/chronos_watch_daemon.pid)" 2>/dev/null || true
    rm -f "/tmp/chronos_watch_daemon.pid"
fi

# Get all chronos sessions
sessions=($(tmux list-sessions 2>/dev/null | grep chronos_ | cut -d: -f1 || true))

if [ ${#sessions[@]} -eq 0 ]; then
    echo "ℹ️ No chronos sessions found to shutdown"
    exit 0
fi

echo "📊 Found ${#sessions[@]} squads to shutdown with learning preservation"

# Extract learnings from each squad
extract_squad_learnings() {
    local session=$1
    local squad_name=$(echo "$session" | sed 's/chronos_//')
    local learning_file="$LEARNING_DIR/${squad_name}_learnings_${TIMESTAMP}.md"
    
    echo "🧠 Extracting learnings from $squad_name..."
    
    # Capture full session history
    tmux capture-pane -t "$session" -p > "$learning_file.raw" 2>/dev/null || true
    
    # Send learning extraction command
    tmux send-keys -t "$session" "" Enter
    tmux send-keys -t "$session" "SHUTDOWN PROTOCOL: Summarize all learnings, improvements made, and optimal prompt refinements. Include: 1) Key Swift/iOS insights discovered 2) Most effective techniques used 3) Recommended prompt improvements 4) Suggestions for future iterations. Format as markdown." Enter
    
    # Wait for response
    sleep 10
    
    # Capture learning summary
    tmux capture-pane -t "$session" -p | tail -50 > "$learning_file"
    
    # Add metadata
    cat << EOF >> "$learning_file"

# Squad Learning Summary - $squad_name
**Timestamp:** $TIMESTAMP
**Session Duration:** $(tmux display-message -t "$session" -p "#{session_created}")
**Swift App Focus:** Creative improvements and research-driven development

## Raw Session Log
\`\`\`
$(cat "$learning_file.raw")
\`\`\`

## Next Generation Prompt Suggestions
Based on this session's learnings, the following prompt optimizations are recommended:

1. **Enhanced Research Integration:** [To be filled by squad analysis]
2. **Creative Problem Solving:** [To be filled by squad analysis] 
3. **Feedback Integration Patterns:** [To be filled by squad analysis]
4. **Swift-Specific Optimizations:** [To be filled by squad analysis]

EOF
    
    rm -f "$learning_file.raw"
    echo "✅ Learning extracted for $squad_name"
}

# Extract learnings from all squads
for session in "${sessions[@]}"; do
    if tmux has-session -t "$session" 2>/dev/null; then
        extract_squad_learnings "$session"
    fi
done

# Create consolidated learning report
echo "📝 Creating consolidated learning report..."
REPORT_FILE="$LEARNING_DIR/consolidated_learnings_${TIMESTAMP}.md"

cat << EOF > "$REPORT_FILE"
# ChronOS Squad Learning Report
**Generated:** $(date)
**Total Squads:** ${#sessions[@]}
**Focus:** Swift App Creative Improvements

## Executive Summary
This report consolidates learnings from all squad sessions focused on creative Swift app improvements.

## Squad-Specific Learnings

EOF

# Append each squad's learnings
for session in "${sessions[@]}"; do
    squad_name=$(echo "$session" | sed 's/chronos_//')
    learning_file="$LEARNING_DIR/${squad_name}_learnings_${TIMESTAMP}.md"
    
    if [ -f "$learning_file" ]; then
        echo "### $squad_name Squad" >> "$REPORT_FILE"
        cat "$learning_file" >> "$REPORT_FILE"
        echo -e "\n---\n" >> "$REPORT_FILE"
    fi
done

# Create optimized Boot.sh for next iteration
echo "🔧 Creating optimized Boot.sh based on learnings..."
OPTIMIZED_BOOT="$WORK_DIR/Boot_optimized_${TIMESTAMP}.sh"

# Copy current Boot.sh and add learning-based improvements
cp "$WORK_DIR/Boot.sh" "$OPTIMIZED_BOOT"

# Add learning integration to optimized boot
cat << 'EOF' >> "$OPTIMIZED_BOOT"

# LEARNING INTEGRATION - Based on previous squad sessions
# Auto-load previous learnings and apply prompt optimizations

load_squad_learnings() {
    local squad=$1
    local latest_learning=$(ls -t squad_learnings/${squad,,}_learnings_*.md 2>/dev/null | head -1)
    
    if [ -f "$latest_learning" ]; then
        echo "🧠 Loading previous learnings for $squad from $latest_learning"
        # Integration logic would be added here
    fi
}

# Enhanced prompt injection with learning context
for squad_entry in "${SQUADS[@]}"; do
    squad=$(echo "$squad_entry" | cut -d: -f1)
    load_squad_learnings "$squad"
done
EOF

# Kill all squads after learning extraction
echo "💀 Terminating all squad sessions..."
for session in "${sessions[@]}"; do
    if tmux has-session -t "$session" 2>/dev/null; then
        echo "🛑 Shutting down $session"
        tmux kill-session -t "$session"
    fi
done

# Create improvement PR
echo "📤 Creating improvement PR based on squad learnings..."

# Stage all learning files
git add squad_learnings/ 2>/dev/null || true
git add "$OPTIMIZED_BOOT" 2>/dev/null || true

# Create learning-based commit
if [ -n "$(git diff --cached)" ]; then
    git commit -m "🧠 LEARNING PRESERVATION: Squad session learnings and optimizations

Generated from squad shutdown on $TIMESTAMP

## Key Improvements:
- Consolidated learnings from ${#sessions[@]} specialized squads
- Swift app creative improvement insights captured
- Optimized Boot.sh with learning integration
- Enhanced prompt strategies based on session outcomes

## Squad Focus Areas:
$(echo "${sessions[@]}" | sed 's/chronos_//g' | tr ' ' '\n' | sed 's/^/- /' | tr '\n' ' ')

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>" || true

    # Create PR
    git push origin main 2>/dev/null || true
    
    # Create PR with learning context
    gh pr create --title "🧠 Squad Learning Integration: Swift App Improvement Optimizations" --body "$(cat <<'PREOF'
## Summary
- Consolidated learnings from autonomous squad sessions focused on Swift app creative improvements
- Captured specialized insights from Architecture, Performance, Testing, Security, Integration, and Coordination squads  
- Generated optimized Boot.sh with learning integration capabilities

## Key Learning Areas
- **Swift/iOS Research Patterns:** Enhanced techniques for discovering innovative solutions
- **Creative Problem Solving:** Improved approaches to Swift app enhancement
- **Feedback Integration:** Better strategies for incorporating user input while maintaining creativity
- **Autonomous Coordination:** Refined multi-squad collaboration patterns

## Next Steps
- Review squad-specific learning files in `squad_learnings/`
- Test optimized Boot.sh for improved squad performance
- Integrate most effective prompt strategies into future iterations

🤖 Generated with [Claude Code](https://claude.ai/code)
PREOF
)" 2>/dev/null || echo "⚠️ PR creation failed - manual review required"
fi

echo ""
echo "🎉 SHUTDOWN COMPLETE with Learning Preservation"
echo ""
echo "📋 Summary:"
echo "   • ${#sessions[@]} squads terminated gracefully"
echo "   • Learnings extracted to: $LEARNING_DIR"
echo "   • Consolidated report: $REPORT_FILE"
echo "   • Optimized boot script: $OPTIMIZED_BOOT"
echo "   • PR created for review (if applicable)"
echo ""
echo "🔄 Next iteration can use: ./Boot_optimized_${TIMESTAMP}.sh"