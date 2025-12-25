#!/bin/bash

# Oxigraph Orchestrator Startup Script
# This script starts the orchestrator service and validates it's running correctly

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_PORT=5000
SERVICE_HOST="localhost"
SERVICE_URL="http://${SERVICE_HOST}:${SERVICE_PORT}"
LOG_FILE="/tmp/oxigraph-orchestrator.log"
PID_FILE="/tmp/oxigraph-orchestrator.pid"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${BLUE}[SUCCESS]${NC} $1"
}

# Check if service is already running
check_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            return 0
        fi
    fi
    return 1
}

# Stop existing service
stop_service() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            log_info "Stopping existing service (PID: $PID)..."
            kill "$PID"
            sleep 2

            # Force kill if still running
            if ps -p "$PID" > /dev/null 2>&1; then
                log_warn "Service didn't stop gracefully, forcing..."
                kill -9 "$PID"
            fi
        fi
        rm -f "$PID_FILE"
    fi
}

# Install dependencies
install_deps() {
    log_info "Checking Python dependencies..."

    if ! command -v python3 &> /dev/null; then
        log_error "Python 3 is not installed. Please install Python 3.11 or higher."
        exit 1
    fi

    PYTHON_VERSION=$(python3 -c 'import sys; print(".".join(map(str, sys.version_info[:2])))')
    log_info "Python version: $PYTHON_VERSION"

    if ! python3 -c "import flask" &> /dev/null; then
        log_info "Installing Python dependencies..."
        pip3 install -r "${SCRIPT_DIR}/requirements.txt" || {
            log_error "Failed to install dependencies"
            exit 1
        }
    else
        log_success "Dependencies already installed"
    fi
}

# Start the service
start_service() {
    log_info "Starting Oxigraph Orchestrator Service..."

    cd "$SCRIPT_DIR"

    # Start service in background
    nohup python3 oxigraph_service.py > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"

    log_info "Service starting (PID: $(cat $PID_FILE))..."
    log_info "Logs: $LOG_FILE"
}

# Wait for service to be ready
wait_for_service() {
    log_info "Waiting for service to be ready..."

    MAX_ATTEMPTS=30
    ATTEMPT=0

    while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
        if curl -s "${SERVICE_URL}/health" > /dev/null 2>&1; then
            log_success "Service is ready!"
            return 0
        fi

        ATTEMPT=$((ATTEMPT + 1))
        echo -n "."
        sleep 1
    done

    echo ""
    log_error "Service failed to start within ${MAX_ATTEMPTS} seconds"
    log_error "Check logs at: $LOG_FILE"
    tail -n 20 "$LOG_FILE"
    return 1
}

# Validate service
validate_service() {
    log_info "Validating service..."

    # Check health endpoint
    HEALTH=$(curl -s "${SERVICE_URL}/health")
    if echo "$HEALTH" | grep -q "healthy"; then
        log_success "Health check passed"
    else
        log_error "Health check failed"
        return 1
    fi

    # Check analytics endpoint
    if curl -s "${SERVICE_URL}/analytics" > /dev/null 2>&1; then
        log_success "Analytics endpoint working"
    else
        log_error "Analytics endpoint failed"
        return 1
    fi

    return 0
}

# Show service info
show_info() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Oxigraph Orchestrator Service${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  Status:      ${GREEN}Running${NC}"
    echo -e "  URL:         ${YELLOW}${SERVICE_URL}${NC}"
    echo -e "  Health:      ${YELLOW}${SERVICE_URL}/health${NC}"
    echo -e "  Analytics:   ${YELLOW}${SERVICE_URL}/analytics${NC}"
    echo -e "  PID:         $(cat $PID_FILE)"
    echo -e "  Logs:        ${LOG_FILE}"
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "API Endpoints:"
    echo "  POST   /tasks                 - Create task"
    echo "  PUT    /tasks/{id}/status     - Update task status"
    echo "  GET    /tasks/ready           - Get ready tasks"
    echo "  GET    /tasks/running         - Get running tasks"
    echo "  GET    /tasks/{id}/chain      - Get dependency chain"
    echo "  GET    /analytics             - Get analytics"
    echo "  GET    /optimize              - Get optimized distribution"
    echo ""
    echo "Management:"
    echo "  ${YELLOW}tail -f $LOG_FILE${NC}  - View logs"
    echo "  ${YELLOW}kill $(cat $PID_FILE)${NC}          - Stop service"
    echo ""
}

# Main execution
main() {
    log_info "Oxigraph Orchestrator Startup"
    echo ""

    # Check if already running
    if check_running; then
        log_warn "Service is already running"
        PID=$(cat "$PID_FILE")
        log_info "PID: $PID"
        log_info "To restart, run: $0 restart"
        exit 0
    fi

    # Install dependencies
    install_deps

    # Stop any existing service
    stop_service

    # Start new service
    start_service

    # Wait for ready
    if ! wait_for_service; then
        log_error "Failed to start service"
        exit 1
    fi

    # Validate
    if ! validate_service; then
        log_error "Service validation failed"
        exit 1
    fi

    # Show info
    show_info

    log_success "Service started successfully!"
}

# Handle command line arguments
case "${1:-start}" in
    start)
        main
        ;;
    stop)
        stop_service
        log_success "Service stopped"
        ;;
    restart)
        stop_service
        main
        ;;
    status)
        if check_running; then
            show_info
        else
            log_warn "Service is not running"
            exit 1
        fi
        ;;
    logs)
        tail -f "$LOG_FILE"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        exit 1
        ;;
esac
