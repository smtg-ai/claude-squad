# Claude Squad Development Makefile

# Variables
BINARY_NAME := claude-squad
INSTALL_DIR := /usr/local/bin
CONFIG_DIR := ~/.claude-squad

# Go related variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOFMT := gofmt
GOMOD := $(GOCMD) mod

# Build targets
.PHONY: all build clean test fmt vet install uninstall dev-setup config-reset config-show config-test run debug help

## Build Commands
all: clean fmt vet test build	## Run all checks and build

build:	## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v

clean:	## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

install: build	## Install binary to system
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	sudo mv $(BINARY_NAME) $(INSTALL_DIR)/

uninstall:	## Uninstall binary from system
	@echo "Uninstalling $(BINARY_NAME) from $(INSTALL_DIR)..."
	sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)

## Development Commands
dev-setup:	## Setup development environment
	@echo "Setting up development environment..."
	$(GOMOD) tidy
	$(GOMOD) download

fmt:	## Format Go code
	@echo "Formatting code..."
	$(GOFMT) -w .

vet:	## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

test:	## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-race:	## Run tests with race detection
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

## Configuration Commands
config-reset:	## Reset configuration to defaults
	@echo "Resetting configuration..."
	rm -f $(CONFIG_DIR)/config.json
	./$(BINARY_NAME) debug

config-show:	## Show current configuration
	@echo "Current configuration:"
	./$(BINARY_NAME) debug

config-test:	## Test with example configuration
	@echo "Testing with example configuration..."
	cp example-config.json $(CONFIG_DIR)/config.json
	./$(BINARY_NAME) debug

## Key Mapping Testing
test-keys-default: build config-reset	## Test with default key mappings
	@echo "=== Testing Default Key Mappings ==="
	./$(BINARY_NAME) debug | grep -A 50 "key_mappings" | head -20

test-keys-custom: build	## Test with custom key mappings (ctrl combinations)
	@echo "=== Testing Custom Key Mappings ==="
	cp example-config.json $(CONFIG_DIR)/config.json
	./$(BINARY_NAME) debug | grep -A 50 "key_mappings" | head -20

test-keys-partial: build	## Test partial key mapping configuration
	@echo "=== Testing Partial Key Mappings ==="
	@echo '{"key_mappings":{"quit":["q","esc"],"new":["n","ctrl+n"]}}' > $(CONFIG_DIR)/config.json
	./$(BINARY_NAME) debug | grep -A 50 "key_mappings" | head -20

test-keys-show: build	## Show key mappings in clean JSON format
	@echo "=== Current Key Mappings ==="
	@./$(BINARY_NAME) debug 2>/dev/null | sed -n '/^{/,/^}/p' | jq '.key_mappings' 2>/dev/null || echo "Could not parse JSON, showing raw output:"
	@./$(BINARY_NAME) debug | grep -A 50 "key_mappings"

## Quick Run Commands
run: build	## Build and run (requires git repo)
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

debug: build	## Build and show debug info
	@echo "Debug information:"
	./$(BINARY_NAME) debug

version:	## Show version information
	./$(BINARY_NAME) version

## Development Workflows
dev-test: clean fmt vet test build config-test	## Full development test cycle

quick-test: build test-keys-default test-keys-custom test-keys-partial	## Quick key mapping tests

check-build:	## Verify build works
	@echo "Checking if build works..."
	$(GOBUILD) -o /tmp/$(BINARY_NAME)-test
	@echo "Build successful!"
	rm -f /tmp/$(BINARY_NAME)-test

## Documentation
help:	## Show this help message
	@echo "Claude Squad Development Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Default target
.DEFAULT_GOAL := help