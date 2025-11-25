.PHONY: all build clean test test-race test-coverage lint fmt vet help install dev

# Build variables
BINARY_NAME=reglet
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X github.com/jrose/reglet/internal/version.Version=$(VERSION) \
                   -X github.com/jrose/reglet/internal/version.Commit=$(COMMIT) \
                   -X github.com/jrose/reglet/internal/version.BuildDate=$(BUILD_DATE)"

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

all: clean lint test build ## Run all: clean, lint, test, and build

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/reglet

clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -rf coverage.out
	rm -rf *.prof

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) -race ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-bench: ## Run benchmark tests
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

lint: ## Run golangci-lint
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install it from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

install: build ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/

dev: ## Build and run locally
	@echo "Building and running $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/reglet
	./bin/$(BINARY_NAME)

# Profile targets
profile-cpu: ## Run CPU profiling
	@echo "Running CPU profiling..."
	$(GOTEST) -cpuprofile=cpu.prof -bench=. ./...
	$(GOCMD) tool pprof -http=:8080 cpu.prof

profile-mem: ## Run memory profiling
	@echo "Running memory profiling..."
	$(GOTEST) -memprofile=mem.prof -bench=. ./...
	$(GOCMD) tool pprof -http=:8080 mem.prof

help: ## Display this help message
	@echo "Reglet Makefile commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
