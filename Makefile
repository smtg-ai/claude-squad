# chronOS claude-squad Performance Optimized Build
# Optimized for Apple M3 Ultra with 32 cores and 512GB RAM

.PHONY: build clean install test bench profile

# Build configuration for maximum performance on M3 Ultra
GOFLAGS := -ldflags="-s -w -X main.version=$(shell git describe --tags --always)" \
		   -gcflags="-l=4" \
		   -tags="netgo,osusergo,static_build" \
		   -trimpath

# M3 Ultra specific optimizations
CGO_ENABLED := 0
GOOS := darwin
GOARCH := arm64
GOMACOS := 14.0
GOMAXPROCS := 32

# Performance build targets
build:
	@echo "🚀 Building claude-squad for M3 Ultra performance..."
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) GOMAXPROCS=$(GOMAXPROCS) \
	go build $(GOFLAGS) -o csq .
	@echo "✅ Build complete: csq"

# Ultra-fast build (development)
fast:
	@echo "⚡ Fast build for development..."
	GOMAXPROCS=32 go build -o csq .

# Production optimized build
release:
	@echo "🎯 Building production release..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 GOMAXPROCS=32 \
	go build -ldflags="-s -w -X main.version=$(shell git describe --tags --always)" \
		-gcflags="-l=4 -B" \
		-tags="netgo,osusergo,static_build" \
		-trimpath \
		-o csq-release .
	@echo "✅ Release build complete: csq-release"

# Install to local bin
install: build
	@echo "📦 Installing to ~/.local/bin/csq..."
	@mkdir -p ~/.local/bin
	cp csq ~/.local/bin/csq
	@echo "✅ Installed successfully"

# Performance testing
bench:
	@echo "📊 Running performance benchmarks..."
	GOMAXPROCS=32 go test -bench=. -benchmem -cpu=1,8,16,32 ./...

# Memory profiling
profile:
	@echo "🔍 Building with profiling enabled..."
	GOMAXPROCS=32 go build -o csq-profile -tags="profile" .

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f csq csq-release csq-profile
	@echo "✅ Clean complete"

# Test with performance optimizations
test:
	@echo "🧪 Running tests with M3 Ultra optimization..."
	GOMAXPROCS=32 go test -v -race -timeout=30s ./...

# Development server with hot reload
dev:
	@echo "🔥 Starting development mode..."
	GOMAXPROCS=32 go run . --daemon=false

# Show build info
info:
	@echo "System Information:"
	@echo "  CPU Cores: $(shell sysctl -n hw.ncpu)"
	@echo "  Memory: $(shell echo $$(( $(shell sysctl -n hw.memsize) / 1024 / 1024 / 1024 )) GB)"
	@echo "  Go Version: $(shell go version)"
	@echo "  GOOS: $(GOOS)"
	@echo "  GOARCH: $(GOARCH)"
	@echo "  GOMAXPROCS: $(GOMAXPROCS)"
