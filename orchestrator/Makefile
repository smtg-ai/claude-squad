.PHONY: help install start stop restart logs test build clean docker-build docker-up docker-down docker-logs

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

## Help:
help: ## Show this help message
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)

## Development:
install: ## Install Python dependencies
	@echo "${GREEN}Installing Python dependencies...${RESET}"
	pip install -r requirements.txt

start: ## Start the orchestrator service
	@echo "${GREEN}Starting Oxigraph orchestrator service...${RESET}"
	python oxigraph_service.py

stop: ## Stop the orchestrator service (if running in background)
	@echo "${YELLOW}Stopping orchestrator service...${RESET}"
	@pkill -f "oxigraph_service.py" || echo "No service running"

restart: stop start ## Restart the orchestrator service

logs: ## Show service logs
	@tail -f /tmp/oxigraph-orchestrator.log

## Testing:
test: ## Run Go tests
	@echo "${GREEN}Running Go tests...${RESET}"
	cd .. && go test -v ./orchestrator/...

test-integration: ## Run integration tests (requires service running)
	@echo "${GREEN}Running integration tests...${RESET}"
	@./test_integration.sh

benchmark: ## Run benchmarks
	@echo "${GREEN}Running benchmarks...${RESET}"
	cd .. && go test -bench=. -benchmem ./orchestrator/...

## Build:
build: ## Build Go components
	@echo "${GREEN}Building Go components...${RESET}"
	cd .. && go build -o bin/orchestrator-client ./orchestrator/example.go
	@echo "${GREEN}Built: bin/orchestrator-client${RESET}"

build-dashboard: ## Build dashboard
	@echo "${GREEN}Building dashboard...${RESET}"
	cd .. && go build -o bin/dashboard ./orchestrator/dashboard.go
	@echo "${GREEN}Built: bin/dashboard${RESET}"

clean: ## Clean build artifacts
	@echo "${YELLOW}Cleaning build artifacts...${RESET}"
	rm -rf ../bin/orchestrator-*
	rm -rf ../bin/dashboard
	find . -type f -name "*.pyc" -delete
	find . -type d -name "__pycache__" -delete

## Docker:
docker-build: ## Build Docker image
	@echo "${GREEN}Building Docker image...${RESET}"
	docker-compose build

docker-up: ## Start Docker services
	@echo "${GREEN}Starting Docker services...${RESET}"
	docker-compose up -d
	@echo "${GREEN}Service available at http://localhost:5000${RESET}"

docker-down: ## Stop Docker services
	@echo "${YELLOW}Stopping Docker services...${RESET}"
	docker-compose down

docker-logs: ## Show Docker logs
	docker-compose logs -f

docker-restart: docker-down docker-up ## Restart Docker services

## Monitoring:
health: ## Check service health
	@echo "${GREEN}Checking service health...${RESET}"
	@curl -s http://localhost:5000/health | jq '.' || echo "${YELLOW}Service not responding${RESET}"

analytics: ## Show current analytics
	@echo "${GREEN}Fetching analytics...${RESET}"
	@curl -s http://localhost:5000/analytics | jq '.' || echo "${YELLOW}Service not responding${RESET}"

dashboard: build-dashboard ## Run the TUI dashboard
	@echo "${GREEN}Starting dashboard...${RESET}"
	../bin/dashboard

## Examples:
example-basic: build ## Run basic example
	@echo "${GREEN}Running basic example...${RESET}"
	../bin/orchestrator-client basic

example-advanced: build ## Run advanced example
	@echo "${GREEN}Running advanced example...${RESET}"
	../bin/orchestrator-client advanced

## Utilities:
format: ## Format code
	@echo "${GREEN}Formatting code...${RESET}"
	cd .. && go fmt ./orchestrator/...
	black oxigraph_service.py

lint: ## Lint code
	@echo "${GREEN}Linting code...${RESET}"
	cd .. && golangci-lint run ./orchestrator/... || echo "Install golangci-lint for linting"
	pylint oxigraph_service.py || echo "Install pylint for Python linting"

deps: ## Show dependencies
	@echo "${CYAN}Python dependencies:${RESET}"
	@cat requirements.txt
	@echo "\n${CYAN}Go dependencies:${RESET}"
	@cd .. && go list -m all | grep -v "claude-squad$$"

## Quick Start:
quickstart: install docker-up ## Quick start: install deps and start services
	@echo ""
	@echo "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
	@echo "${GREEN}  Oxigraph Orchestrator is ready!${RESET}"
	@echo "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
	@echo ""
	@echo "  Service: ${CYAN}http://localhost:5000${RESET}"
	@echo "  Health:  ${CYAN}http://localhost:5000/health${RESET}"
	@echo ""
	@echo "Try these commands:"
	@echo "  ${YELLOW}make analytics${RESET}  - View current statistics"
	@echo "  ${YELLOW}make dashboard${RESET}   - Open TUI dashboard"
	@echo "  ${YELLOW}make example-basic${RESET} - Run basic example"
	@echo ""

all: clean install docker-build docker-up test ## Do everything: clean, install, build, start, test
