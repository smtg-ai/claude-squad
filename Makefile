# Claude Squad Makefile
# Build and development tools for claude-squad CLI

# Variables
BINARY_NAME=claude-squad
BUILD_DIR=./bin
MAIN_PACKAGE=.
GO_VERSION=1.23.0

# Version info
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS=-s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT_SHA)' \
	-X 'main.buildTime=$(BUILD_TIME)'
BUILD_FLAGS=-ldflags "$(LDFLAGS)" -trimpath

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[0;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

# Default target
.PHONY: all
all: clean lint-basic test build

# Help target
.PHONY: help
help:
	@echo "$(BLUE)Claude Squad Build System$(NC)"
	@echo ""
	@echo "$(YELLOW)Development Commands:$(NC)"
	@echo "  dev          Run in development mode (go run)"
	@echo "  test         Run all tests"
	@echo "  test-v       Run tests with verbose output"
	@echo "  test-race    Run tests with race detection"
	@echo "  bench        Run benchmarks"
	@echo ""
	@echo "$(YELLOW)Build Commands:$(NC)"
	@echo "  build        Build binary for current platform"
	@echo "  build-all    Build binaries for all platforms"
	@echo "  install      Install binary to GOPATH/bin"
	@echo ""
	@echo "$(YELLOW)Quality Commands:$(NC)"
	@echo "  lint         Run linters (golangci-lint with fallback)"
	@echo "  lint-basic   Run basic Go tools only (vet + fmt check)"
	@echo "  lint-all     Run all linters including security tools"
	@echo "  fmt          Format code"
	@echo "  vet          Run go vet"
	@echo ""
	@echo "$(YELLOW)Maintenance Commands:$(NC)"
	@echo "  clean        Clean build artifacts"
	@echo "  clean-all    Clean all generated files including dependencies"
	@echo "  deps         Download and verify dependencies"
	@echo "  tidy         Tidy go modules"
	@echo ""
	@echo "$(YELLOW)Tool Management:$(NC)"
	@echo "  install-tools Install development tools"
	@echo "  check-tools   Check if development tools are installed"
	@echo ""
	@echo "$(YELLOW)Project Commands:$(NC)"
	@echo "  release      Create release build"
	@echo "  version      Show version info"
	@echo "  doctor       Check development environment"

# Development
.PHONY: dev
dev:
	@echo "$(GREEN)Running in development mode...$(NC)"
	go run $(MAIN_PACKAGE)

# Testing
.PHONY: test
test:
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v ./...

.PHONY: test-v
test-v:
	@echo "$(GREEN)Running tests with verbose output...$(NC)"
	go test -v -count=1 ./...

.PHONY: test-race
test-race:
	@echo "$(GREEN)Running tests with race detection...$(NC)"
	go test -race -v ./...

.PHONY: bench
bench:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	go test -bench=. -benchmem ./...

# Building
.PHONY: build
build: fmt
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "$(GREEN)Built: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: build-all
build-all: fmt
	@echo "$(GREEN)Building for all platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	# Linux
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	# macOS
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	# Windows
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	@echo "$(GREEN)All builds completed in $(BUILD_DIR)/$(NC)"

.PHONY: install
install: build
	@echo "$(GREEN)Installing to GOPATH/bin...$(NC)"
	go install $(BUILD_FLAGS) $(MAIN_PACKAGE)
	@echo "$(GREEN)Installed: $(shell go env GOPATH)/bin/$(BINARY_NAME)$(NC)"

# Code quality
.PHONY: lint
lint:
	@echo "$(GREEN)Running linters...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run || (echo "$(YELLOW)golangci-lint failed, running basic checks...$(NC)" && go vet ./... && gofmt -d .); \
	elif [ -f "$(shell go env GOPATH)/bin/golangci-lint" ]; then \
		$(shell go env GOPATH)/bin/golangci-lint run || (echo "$(YELLOW)golangci-lint failed, running basic checks...$(NC)" && go vet ./... && gofmt -d .); \
	else \
		echo "$(YELLOW)golangci-lint not found, running basic checks...$(NC)"; \
		go vet ./...; \
		gofmt -d .; \
	fi

.PHONY: fmt
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...

.PHONY: lint-basic
lint-basic:
	@echo "$(GREEN)Running basic Go tools...$(NC)"
	go vet ./...
	@echo "Checking formatting..."
	@gofmt -d . | (read line && echo "$(YELLOW)Code formatting issues found:$(NC)" && echo "$$line" && cat && exit 1 || true)

.PHONY: vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	go vet ./...

# Dependencies
.PHONY: deps
deps:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	go mod download
	go mod verify

.PHONY: tidy
tidy:
	@echo "$(GREEN)Tidying modules...$(NC)"
	go mod tidy

# Cleaning
.PHONY: clean
clean:
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	go clean -cache

.PHONY: clean-all
clean-all: clean
	@echo "$(GREEN)Deep cleaning...$(NC)"
	go clean -modcache
	rm -rf worktree*
	rm -rf ~/.claude-squad

# Tool management
.PHONY: check-tools
check-tools:
	@echo "$(BLUE)Checking development tools...$(NC)"
	@echo "Core tools:"
	@command -v go >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) go" || echo "  $(RED)✗$(NC) go (required)"
	@command -v git >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) git" || echo "  $(RED)✗$(NC) git (required)"
	@echo ""
	@echo "Runtime dependencies:"
	@command -v tmux >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) tmux" || echo "  $(RED)✗$(NC) tmux (required for runtime)"
	@command -v gh >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) gh (GitHub CLI)" || echo "  $(RED)✗$(NC) gh (GitHub CLI required for runtime)"
	@echo ""
	@echo "Development tools:"
	@command -v golangci-lint >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) golangci-lint" || echo "  $(YELLOW)⚠$(NC) golangci-lint (recommended)"
	@command -v staticcheck >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) staticcheck" || echo "  $(YELLOW)⚠$(NC) staticcheck (optional)"
	@command -v gosec >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) gosec" || echo "  $(YELLOW)⚠$(NC) gosec (optional)"

.PHONY: install-tools
install-tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	@echo "Installing golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin latest; \
	else \
		echo "  $(YELLOW)golangci-lint already installed$(NC)"; \
	fi
	@echo "Installing staticcheck..."
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	else \
		echo "  $(YELLOW)staticcheck already installed$(NC)"; \
	fi
	@echo "Installing gosec..."
	@if ! command -v gosec >/dev/null 2>&1; then \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	else \
		echo "  $(YELLOW)gosec already installed$(NC)"; \
	fi
	@echo "Installing runtime dependencies (if missing)..."
	@$(MAKE) install-runtime-deps
	@echo "$(GREEN)Development tools installation completed!$(NC)"

.PHONY: install-runtime-deps
install-runtime-deps:
	@echo "Checking runtime dependencies..."
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing tmux...$(NC)"; \
		if [[ "$(shell uname)" == "Darwin" ]]; then \
			if command -v brew >/dev/null 2>&1; then \
				brew install tmux; \
			else \
				echo "$(RED)Homebrew required to install tmux on macOS$(NC)"; \
				exit 1; \
			fi; \
		elif [[ "$(shell uname)" == "Linux" ]]; then \
			if command -v apt-get >/dev/null 2>&1; then \
				sudo apt-get update && sudo apt-get install -y tmux; \
			elif command -v yum >/dev/null 2>&1; then \
				sudo yum install -y tmux; \
			elif command -v pacman >/dev/null 2>&1; then \
				sudo pacman -S --noconfirm tmux; \
			else \
				echo "$(RED)Please install tmux manually$(NC)"; \
				exit 1; \
			fi; \
		fi; \
	fi
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing GitHub CLI...$(NC)"; \
		if [[ "$(shell uname)" == "Darwin" ]]; then \
			if command -v brew >/dev/null 2>&1; then \
				brew install gh; \
			else \
				echo "$(RED)Homebrew required to install gh on macOS$(NC)"; \
				exit 1; \
			fi; \
		elif [[ "$(shell uname)" == "Linux" ]]; then \
			if command -v apt-get >/dev/null 2>&1; then \
				curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg; \
				sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg; \
				echo "deb [arch=$$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list; \
				sudo apt-get update && sudo apt-get install -y gh; \
			elif command -v yum >/dev/null 2>&1; then \
				sudo yum install -y gh; \
			else \
				echo "$(RED)Please install GitHub CLI manually$(NC)"; \
				exit 1; \
			fi; \
		fi; \
	fi

# Enhanced linting with more tools
.PHONY: lint-all
lint-all: lint
	@echo "$(GREEN)Running extended linting...$(NC)"
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	elif [ -f "$(shell go env GOPATH)/bin/staticcheck" ]; then \
		$(shell go env GOPATH)/bin/staticcheck ./...; \
	else \
		echo "$(YELLOW)staticcheck not available, run 'make install-tools'$(NC)"; \
	fi
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	elif [ -f "$(shell go env GOPATH)/bin/gosec" ]; then \
		$(shell go env GOPATH)/bin/gosec ./...; \
	else \
		echo "$(YELLOW)gosec not available, run 'make install-tools'$(NC)"; \
	fi

# Project utilities
.PHONY: release
release: clean lint-basic test build-all
	@echo "$(GREEN)Release build completed$(NC)"

.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT_SHA)"
	@echo "Built:   $(BUILD_TIME)"
	@echo "Go:      $(GO_VERSION)"

.PHONY: doctor
doctor:
	@echo "$(BLUE)Checking development environment...$(NC)"
	@echo "Go version: $(shell go version)"
	@echo "Git status: $(shell git status --porcelain | wc -l | xargs) uncommitted files"
	@echo ""
	@echo "Required tools:"
	@command -v tmux >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) tmux" || echo "  $(RED)✗$(NC) tmux (missing)"
	@command -v gh >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) gh (GitHub CLI)" || echo "  $(RED)✗$(NC) gh (GitHub CLI missing)"
	@echo ""
	@echo "Development tools:"
	@(command -v golangci-lint >/dev/null 2>&1 || [ -f "$(shell go env GOPATH)/bin/golangci-lint" ]) && echo "  $(GREEN)✓$(NC) golangci-lint" || echo "  $(YELLOW)⚠$(NC) golangci-lint (run 'make install-tools')"
	@(command -v staticcheck >/dev/null 2>&1 || [ -f "$(shell go env GOPATH)/bin/staticcheck" ]) && echo "  $(GREEN)✓$(NC) staticcheck" || echo "  $(YELLOW)⚠$(NC) staticcheck (run 'make install-tools')"
	@(command -v gosec >/dev/null 2>&1 || [ -f "$(shell go env GOPATH)/bin/gosec" ]) && echo "  $(GREEN)✓$(NC) gosec" || echo "  $(YELLOW)⚠$(NC) gosec (run 'make install-tools')"
	@echo ""
	@echo "Module status:"
	@go list -m -f '  {{.Path}}: {{.Version}}' all | head -5
	@echo "  ... ($(shell go list -m all | wc -l | xargs) total modules)"

# Quick aliases
.PHONY: b t l c it ct
b: build
t: test  
l: lint
c: clean
it: install-tools
ct: check-tools