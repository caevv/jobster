.PHONY: build test lint clean build-all install fmt vet coverage help

# Version info (can be overridden)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

# Binary name
BINARY := jobster

help: ## Show this help message
	@echo "Jobster - Development Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build binary with version info
	@echo "Building $(BINARY) $(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/jobster

install: ## Install binary to $GOPATH/bin
	@echo "Installing $(BINARY)..."
	go install -ldflags "$(LDFLAGS)" ./cmd/jobster

test: ## Run tests with race detection
	@echo "Running tests..."
	go test -race -v ./...

coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML coverage report, run: go tool cover -html=coverage.out"

lint: ## Run linters (go vet + gofumpt check)
	@echo "Running go vet..."
	go vet ./...
	@echo "Checking formatting with gofumpt..."
	@if command -v gofumpt >/dev/null 2>&1; then \
		if [ -n "$$(gofumpt -l .)" ]; then \
			echo "Code is not formatted. Run 'make fmt' to fix."; \
			gofumpt -l .; \
			exit 1; \
		fi; \
	else \
		echo "gofumpt not installed. Install with: go install mvdan.cc/gofumpt@latest"; \
	fi

fmt: ## Format code with gofumpt
	@echo "Formatting code..."
	@if command -v gofumpt >/dev/null 2>&1; then \
		gofumpt -l -w .; \
	else \
		echo "gofumpt not installed. Install with: go install mvdan.cc/gofumpt@latest"; \
		exit 1; \
	fi

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -f $(BINARY)
	rm -f coverage.out
	rm -f jobster-*-*

build-all: ## Build binaries for all platforms (Linux amd64/arm64, macOS amd64/arm64)
	@echo "Building for all platforms..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/jobster-linux-amd64 ./cmd/jobster
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/jobster-linux-arm64 ./cmd/jobster
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/jobster-darwin-amd64 ./cmd/jobster
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/jobster-darwin-arm64 ./cmd/jobster
	@echo "Binaries built in dist/"
	@ls -lh dist/

ci-lint: ## Run CI linting checks
	@echo "Running CI lint checks..."
	go vet ./...
	go install mvdan.cc/gofumpt@latest
	@if [ -n "$$(gofumpt -l .)" ]; then \
		echo "Code is not formatted:"; \
		gofumpt -l .; \
		exit 1; \
	fi

ci-test: ## Run CI tests
	@echo "Running CI tests..."
	go test -race -coverprofile=coverage.out ./...

ci-build: ## Run CI build
	@echo "Running CI build..."
	go build -o $(BINARY) ./cmd/jobster
	./$(BINARY) --version

all: lint test build ## Run lint, test, and build

.DEFAULT_GOAL := help
