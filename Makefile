# Claude Squad Makefile
# Build and development tools for claude-squad CLI

# =============================================================================
# Configuration Variables
# =============================================================================

# Basic project configuration
BINARY_NAME := claude-squad
BUILD_DIR := ./bin
MAIN_PACKAGE := .
GO_VERSION := 1.23.0

# Version information (dynamically generated)
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build configuration
LDFLAGS := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT_SHA)' \
	-X 'main.buildTime=$(BUILD_TIME)'
BUILD_FLAGS := -ldflags "$(LDFLAGS)" -trimpath

# Tool paths and detection
GOPATH_BIN := $(shell go env GOPATH)/bin
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo "$(GOPATH_BIN)/golangci-lint")
STATICCHECK := $(shell command -v staticcheck 2>/dev/null || echo "$(GOPATH_BIN)/staticcheck")
GOSEC := $(shell command -v gosec 2>/dev/null || echo "$(GOPATH_BIN)/gosec")

# Platform detection
OS := $(shell uname)
ifeq ($(OS),Darwin)
	PACKAGE_MANAGER := brew
else ifeq ($(OS),Linux)
	PACKAGE_MANAGER := $(shell command -v apt-get >/dev/null 2>&1 && echo "apt" || \
		command -v yum >/dev/null 2>&1 && echo "yum" || \
		command -v pacman >/dev/null 2>&1 && echo "pacman" || echo "unknown")
endif

# Output colors
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m

# =============================================================================
# Helper Functions
# =============================================================================

# Check if a tool exists (either in PATH or GOPATH/bin)
define tool_exists
$(shell command -v $(1) >/dev/null 2>&1 || test -f "$(GOPATH_BIN)/$(1)")
endef

# Print status with color
define status_msg
@echo "$(GREEN)$(1)$(NC)"
endef

define warning_msg
@echo "$(YELLOW)$(1)$(NC)"
endef

define error_msg
@echo "$(RED)$(1)$(NC)"
endef

# =============================================================================
# Main Targets
# =============================================================================

# Default target - common development workflow
.PHONY: all
all: clean lint-basic test build

# Help target with organized sections
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
	@echo "  release      Create release build (clean + lint + test + build-all)"
	@echo ""
	@echo "$(YELLOW)Installation Commands:$(NC)"
	@echo "  install-symlink     Create 'cs' symlink via brew"
	@echo "  symlink-local       Create 'cs' symlink in local bin"
	@echo "  symlink-system      Create 'cs' symlink in /usr/local/bin"
	@echo "  alias-fish          Add 'cs' alias to Fish shell"
	@echo ""
	@echo "$(YELLOW)Quality Commands:$(NC)"
	@echo "  lint         Run linters (golangci-lint with fallback)"
	@echo "  lint-basic   Run basic Go tools (vet + fmt check)"
	@echo "  lint-all     Run all linters including security tools"
	@echo "  fmt          Format code"
	@echo "  vet          Run go vet"
	@echo ""
	@echo "$(YELLOW)Maintenance Commands:$(NC)"
	@echo "  clean        Clean build artifacts"
	@echo "  clean-all    Clean all generated files"
	@echo "  deps         Download and verify dependencies"
	@echo "  tidy         Tidy go modules"
	@echo ""
	@echo "$(YELLOW)Tool Management:$(NC)"
	@echo "  install-tools       Install development tools"
	@echo "  install-runtime     Install runtime dependencies"
	@echo "  check-tools         Check development environment"
	@echo "  doctor              Complete environment check"
	@echo ""
	@echo "$(YELLOW)Info Commands:$(NC)"
	@echo "  version      Show version information"
	@echo ""
	@echo "$(YELLOW)Quick Aliases:$(NC)"
	@echo "  b=build t=test l=lint c=clean it=install-tools ct=check-tools"

# =============================================================================
# Development Targets
# =============================================================================

.PHONY: dev
dev:
	$(call status_msg,Running in development mode...)
	go run $(MAIN_PACKAGE)

# =============================================================================
# Testing Targets
# =============================================================================

.PHONY: test
test:
	$(call status_msg,Running tests...)
	go test -v ./...

.PHONY: test-v
test-v:
	$(call status_msg,Running tests with verbose output...)
	go test -v -count=1 ./...

.PHONY: test-race
test-race:
	$(call status_msg,Running tests with race detection...)
	go test -race -v ./...

.PHONY: bench
bench:
	$(call status_msg,Running benchmarks...)
	go test -bench=. -benchmem ./...

# =============================================================================
# Building Targets
# =============================================================================

.PHONY: build
build: fmt
	$(call status_msg,Building $(BINARY_NAME)...)
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	$(call status_msg,Built: $(BUILD_DIR)/$(BINARY_NAME))

.PHONY: build-all
build-all: fmt
	$(call status_msg,Building for all platforms...)
	@mkdir -p $(BUILD_DIR)
	@echo "Building Linux binaries..."
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	@echo "Building macOS binaries..."
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	@echo "Building Windows binaries..."
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	$(call status_msg,All builds completed in $(BUILD_DIR)/)

.PHONY: install
install: build
	$(call status_msg,Installing to GOPATH/bin...)
	go install $(BUILD_FLAGS) $(MAIN_PACKAGE)
	$(call status_msg,Installed: $(GOPATH_BIN)/$(BINARY_NAME))

# =============================================================================
# Symlink and Alias Targets
# =============================================================================

.PHONY: install-symlink
install-symlink:
	$(call status_msg,Creating symlink cs -> claude-squad...)
	@if command -v claude-squad >/dev/null 2>&1; then \
		if command -v brew >/dev/null 2>&1; then \
			ln -sf "$$(which claude-squad)" "$$(brew --prefix)/bin/cs" && \
			echo "$(GREEN)Symlink created: $$(brew --prefix)/bin/cs -> $$(which claude-squad)$(NC)"; \
		else \
			$(call error_msg,Homebrew not found. Cannot create symlink.); \
			exit 1; \
		fi; \
	else \
		$(call error_msg,claude-squad not found in PATH. Run 'make install' first.); \
		exit 1; \
	fi

.PHONY: symlink-local
symlink-local: build
	$(call status_msg,Creating local symlink cs -> claude-squad...)
	ln -sf "$(shell pwd)/$(BUILD_DIR)/$(BINARY_NAME)" "$(BUILD_DIR)/cs"
	$(call status_msg,Local symlink created: $(BUILD_DIR)/cs)

.PHONY: symlink-system
symlink-system: build
	$(call status_msg,Creating system-wide symlink...)
	@if [ ! -L "$(BUILD_DIR)/cs" ]; then \
		$(call warning_msg,Creating local symlink as well...); \
		ln -sf "$(shell pwd)/$(BUILD_DIR)/$(BINARY_NAME)" "$(BUILD_DIR)/cs"; \
	fi
	sudo ln -sf "$(shell pwd)/$(BUILD_DIR)/$(BINARY_NAME)" /usr/local/bin/cs
	$(call status_msg,System symlink created: /usr/local/bin/cs)
	$(call warning_msg,cs is now available system-wide)

.PHONY: alias-fish
alias-fish: build
	$(call status_msg,Adding Fish shell alias...)
	@if [ -d "$$HOME/.config/fish" ]; then \
		echo "alias cs '$(shell pwd)/$(BUILD_DIR)/$(BINARY_NAME)'" >> $$HOME/.config/fish/config.fish; \
		$(call status_msg,Fish alias added); \
		$(call warning_msg,Run 'source ~/.config/fish/config.fish' or restart shell); \
	else \
		$(call error_msg,Fish config directory not found at ~/.config/fish); \
		exit 1; \
	fi

# =============================================================================
# Code Quality Targets
# =============================================================================

.PHONY: fmt
fmt:
	$(call status_msg,Formatting code...)
	go fmt ./...

.PHONY: vet
vet:
	$(call status_msg,Running go vet...)
	go vet ./...

.PHONY: lint-basic
lint-basic:
	$(call status_msg,Running basic Go tools...)
	go vet ./...
	@echo "Checking formatting..."
	@if [ -n "$$(gofmt -d .)" ]; then \
		$(call warning_msg,Code formatting issues found:); \
		gofmt -d .; \
		exit 1; \
	fi

.PHONY: lint
lint:
	$(call status_msg,Running linters...)
	@if command -v golangci-lint >/dev/null 2>&1 || test -f "$(GOLANGCI_LINT)"; then \
		$(GOLANGCI_LINT) run || ($(call warning_msg,golangci-lint failed, running basic checks...) && $(MAKE) lint-basic); \
	else \
		$(call warning_msg,golangci-lint not found, running basic checks...); \
		$(MAKE) lint-basic; \
	fi

.PHONY: lint-all
lint-all: lint
	$(call status_msg,Running extended linting...)
	@if command -v staticcheck >/dev/null 2>&1 || test -f "$(STATICCHECK)"; then \
		$(STATICCHECK) ./...; \
	else \
		$(call warning_msg,staticcheck not available, run 'make install-tools'); \
	fi
	@if command -v gosec >/dev/null 2>&1 || test -f "$(GOSEC)"; then \
		$(GOSEC) ./...; \
	else \
		$(call warning_msg,gosec not available, run 'make install-tools'); \
	fi

# =============================================================================
# Dependency Management
# =============================================================================

.PHONY: deps
deps:
	$(call status_msg,Downloading dependencies...)
	go mod download
	go mod verify

.PHONY: tidy
tidy:
	$(call status_msg,Tidying modules...)
	go mod tidy

# =============================================================================
# Cleaning Targets
# =============================================================================

.PHONY: clean
clean:
	$(call status_msg,Cleaning build artifacts...)
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	go clean -cache

.PHONY: clean-all
clean-all: clean
	$(call status_msg,Deep cleaning...)
	go clean -modcache
	rm -rf worktree*
	rm -rf ~/.claude-squad

# =============================================================================
# Tool Management
# =============================================================================

.PHONY: check-tools
check-tools:
	@echo "$(BLUE)Development Environment Check$(NC)"
	@echo ""
	@echo "Core tools:"
	@command -v go >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) go" || echo "  $(RED)✗$(NC) go (required)"
	@command -v git >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) git" || echo "  $(RED)✗$(NC) git (required)"
	@echo ""
	@echo "Runtime dependencies:"
	@command -v tmux >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) tmux" || echo "  $(RED)✗$(NC) tmux (required for runtime)"
	@command -v gh >/dev/null 2>&1 && echo "  $(GREEN)✓$(NC) gh (GitHub CLI)" || echo "  $(RED)✗$(NC) gh (required for runtime)"
	@echo ""
	@echo "Development tools:"
	@(command -v golangci-lint >/dev/null 2>&1 || test -f "$(GOLANGCI_LINT)") && echo "  $(GREEN)✓$(NC) golangci-lint" || echo "  $(YELLOW)⚠$(NC) golangci-lint (recommended)"
	@(command -v staticcheck >/dev/null 2>&1 || test -f "$(STATICCHECK)") && echo "  $(GREEN)✓$(NC) staticcheck" || echo "  $(YELLOW)⚠$(NC) staticcheck (optional)"
	@(command -v gosec >/dev/null 2>&1 || test -f "$(GOSEC)") && echo "  $(GREEN)✓$(NC) gosec" || echo "  $(YELLOW)⚠$(NC) gosec (optional)"

# Install Go development tools
.PHONY: install-tools
install-tools:
	$(call status_msg,Installing development tools...)
	@$(MAKE) -s install-tool TOOL=golangci-lint URL=https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh
	@$(MAKE) -s install-go-tool TOOL=staticcheck PACKAGE=honnef.co/go/tools/cmd/staticcheck@latest
	@$(MAKE) -s install-go-tool TOOL=gosec PACKAGE=github.com/securego/gosec/v2/cmd/gosec@latest
	@$(MAKE) install-runtime
	$(call status_msg,Development tools installation completed!)

# Helper target for installing shell-based tools
.PHONY: install-tool
install-tool:
	@if ! command -v $(TOOL) >/dev/null 2>&1; then \
		echo "Installing $(TOOL)..."; \
		curl -sSfL $(URL) | sh -s -- -b $(GOPATH_BIN) latest; \
	else \
		$(call warning_msg,$(TOOL) already installed); \
	fi

# Helper target for installing Go-based tools
.PHONY: install-go-tool
install-go-tool:
	@if ! command -v $(TOOL) >/dev/null 2>&1; then \
		echo "Installing $(TOOL)..."; \
		go install $(PACKAGE); \
	else \
		$(call warning_msg,$(TOOL) already installed); \
	fi

# Install runtime dependencies
.PHONY: install-runtime
install-runtime:
	$(call status_msg,Installing runtime dependencies...)
ifeq ($(OS),Darwin)
	@$(MAKE) -s install-darwin-deps
else ifeq ($(OS),Linux)
	@$(MAKE) -s install-linux-deps
else
	$(call warning_msg,Unsupported OS: $(OS))
endif

.PHONY: install-darwin-deps
install-darwin-deps:
	@if ! command -v brew >/dev/null 2>&1; then \
		$(call error_msg,Homebrew required for macOS installation); \
		exit 1; \
	fi
	@command -v tmux >/dev/null 2>&1 || brew install tmux
	@command -v gh >/dev/null 2>&1 || brew install gh

.PHONY: install-linux-deps
install-linux-deps:
ifeq ($(PACKAGE_MANAGER),apt)
	@command -v tmux >/dev/null 2>&1 || (sudo apt-get update && sudo apt-get install -y tmux)
	@command -v gh >/dev/null 2>&1 || $(MAKE) -s install-gh-debian
else ifeq ($(PACKAGE_MANAGER),yum)
	@command -v tmux >/dev/null 2>&1 || sudo yum install -y tmux
	@command -v gh >/dev/null 2>&1 || sudo yum install -y gh
else ifeq ($(PACKAGE_MANAGER),pacman)
	@command -v tmux >/dev/null 2>&1 || sudo pacman -S --noconfirm tmux
	@command -v gh >/dev/null 2>&1 || sudo pacman -S --noconfirm github-cli
else
	$(call error_msg,Unsupported package manager. Please install tmux and gh manually)
	exit 1
endif

.PHONY: install-gh-debian
install-gh-debian:
	curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
	sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
	echo "deb [arch=$$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list
	sudo apt-get update && sudo apt-get install -y gh

# =============================================================================
# Project Utilities
# =============================================================================

.PHONY: release
release: clean lint-basic test build-all
	$(call status_msg,Release build completed)

.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT_SHA)"
	@echo "Built:   $(BUILD_TIME)"
	@echo "Go:      $(GO_VERSION)"

.PHONY: doctor
doctor:
	@echo "$(BLUE)Development Environment Status$(NC)"
	@echo ""
	@echo "System Information:"
	@echo "  OS: $(OS)"
	@echo "  Go: $(shell go version)"
	@echo "  Package Manager: $(PACKAGE_MANAGER)"
	@echo ""
	@echo "Git Status:"
	@echo "  Uncommitted files: $(shell git status --porcelain | wc -l | xargs)"
	@echo "  Current branch: $(shell git branch --show-current 2>/dev/null || echo 'unknown')"
	@echo ""
	@$(MAKE) -s check-tools
	@echo ""
	@echo "Module Status:"
	@go list -m -f '  {{.Path}}: {{.Version}}' all | head -5
	@echo "  ... ($(shell go list -m all | wc -l | xargs) total modules)"

# =============================================================================
# Quick Aliases
# =============================================================================

.PHONY: b t l c it ct
b: build
t: test
l: lint
c: clean
it: install-tools
ct: check-tools