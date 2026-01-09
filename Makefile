# ╔════════════════════════════════════════════════════════════════════════════╗
# ║                              REGLET MAKEFILE                               ║
# ╚════════════════════════════════════════════════════════════════════════════╝
#
# Usage: make <target>
# Run 'make help' for a list of available targets
#

.PHONY: all build clean test test-race test-coverage lint fmt vet help install dev
.PHONY: fuzz fuzz-extended profile-cpu profile-mem test-bench tidy changelog

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────

BINARY_NAME := reglet
VERSION     ?= dev
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS := -ldflags "\
	-X github.com/whiskeyjimbo/reglet/internal/infrastructure/build.Version=$(VERSION) \
	-X github.com/whiskeyjimbo/reglet/internal/infrastructure/build.Commit=$(COMMIT) \
	-X github.com/whiskeyjimbo/reglet/internal/infrastructure/build.BuildDate=$(BUILD_DATE)"

# Go commands
GOCMD   := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST  := $(GOCMD) test
GOGET   := $(GOCMD) get
GOMOD   := $(GOCMD) mod
GOFMT   := $(GOCMD) fmt
GOVET   := $(GOCMD) vet

# ─────────────────────────────────────────────────────────────────────────────
# Colors and Formatting
# ─────────────────────────────────────────────────────────────────────────────

# Colors
RESET   := \033[0m
BOLD    := \033[1m
RED     := \033[31m
GREEN   := \033[32m
YELLOW  := \033[33m
BLUE    := \033[34m
MAGENTA := \033[35m
CYAN    := \033[36m
WHITE   := \033[37m

# Styled prefixes
INFO    := @printf "$(BOLD)$(CYAN)▸$(RESET) "
SUCCESS := @printf "$(BOLD)$(GREEN)✓$(RESET) "
WARN    := @printf "$(BOLD)$(YELLOW)⚠$(RESET) "
ERROR   := @printf "$(BOLD)$(RED)✗$(RESET) "
STEP    := @printf "$(BOLD)$(MAGENTA)→$(RESET) "

# ═══════════════════════════════════════════════════════════════════════════════
# PRIMARY TARGETS
# ═══════════════════════════════════════════════════════════════════════════════

.DEFAULT_GOAL := help

##@ Primary

all: clean lint test build  ## Run full pipeline: clean → lint → test → build
	$(SUCCESS)
	@printf "$(GREEN)All tasks completed successfully!$(RESET)\n"

build:  ## Build the reglet binary
	$(INFO)
	@printf "Building $(BOLD)$(BINARY_NAME)$(RESET)...\n"
	@$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/reglet
	$(SUCCESS)
	@printf "Binary built: $(GREEN)bin/$(BINARY_NAME)$(RESET)\n"

dev: build  ## Build and run locally
	$(STEP)
	@printf "Running $(BOLD)$(BINARY_NAME)$(RESET)...\n"
	@./bin/$(BINARY_NAME)

install: build  ## Install binary to $$GOPATH/bin
	$(INFO)
	@printf "Installing $(BOLD)$(BINARY_NAME)$(RESET) to $(GOPATH)/bin...\n"
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/
	$(SUCCESS)
	@printf "Installed to $(GREEN)$(GOPATH)/bin/$(BINARY_NAME)$(RESET)\n"

# ═══════════════════════════════════════════════════════════════════════════════
# TESTING
# ═══════════════════════════════════════════════════════════════════════════════

##@ Testing

test:  ## Run all tests with race detector
	$(INFO)
	@printf "Running tests with race detector...\n"
	@$(GOTEST) -v -race ./...
	$(SUCCESS)
	@printf "$(GREEN)All tests passed$(RESET)\n"

test-race:  ## Run tests with race detector (alias)
	$(INFO)
	@printf "Running tests with race detector...\n"
	@$(GOTEST) -race ./...

test-coverage:  ## Run tests with coverage report
	$(INFO)
	@printf "Running tests with coverage...\n"
	@$(GOTEST) -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	$(SUCCESS)
	@printf "Coverage report: $(GREEN)coverage.html$(RESET)\n"

test-bench:  ## Run benchmark tests
	$(INFO)
	@printf "Running benchmarks...\n"
	@$(GOTEST) -bench=. -benchmem ./...

# ═══════════════════════════════════════════════════════════════════════════════
# CODE QUALITY
# ═══════════════════════════════════════════════════════════════════════════════

##@ Code Quality

lint:  ## Run golangci-lint
	$(INFO)
	@printf "Running linters...\n"
	@which golangci-lint > /dev/null || (printf "$(RED)golangci-lint not found$(RESET)\n" && \
		printf "Install: $(CYAN)https://golangci-lint.run/usage/install/$(RESET)\n" && exit 1)
	@golangci-lint run ./...
	$(SUCCESS)
	@printf "$(GREEN)Linting passed$(RESET)\n"

fmt:  ## Format Go source files
	$(INFO)
	@printf "Formatting code...\n"
	@$(GOFMT) ./...
	$(SUCCESS)
	@printf "$(GREEN)Code formatted$(RESET)\n"

vet:  ## Run go vet
	$(INFO)
	@printf "Running go vet...\n"
	@$(GOVET) ./...
	$(SUCCESS)
	@printf "$(GREEN)Vet passed$(RESET)\n"

tidy:  ## Tidy go.mod dependencies
	$(INFO)
	@printf "Tidying dependencies...\n"
	@$(GOMOD) tidy
	$(SUCCESS)
	@printf "$(GREEN)Dependencies tidied$(RESET)\n"

changelog:  ## Generate CHANGELOG.md from git history
	$(INFO)
	@printf "Generating changelog...\n"
	@which git-cliff > /dev/null || (printf "$(RED)git-cliff not found$(RESET)\n" && \
		printf "Install: $(CYAN)cargo install git-cliff$(RESET)\n" && exit 1)
	@git-cliff -o CHANGELOG.md
	$(SUCCESS)
	@printf "$(GREEN)CHANGELOG.md updated$(RESET)\n"

# ═══════════════════════════════════════════════════════════════════════════════
# PROFILING
# ═══════════════════════════════════════════════════════════════════════════════

##@ Profiling

profile-cpu:  ## Run CPU profiling (opens browser)
	$(INFO)
	@printf "Running CPU profiling...\n"
	@$(GOTEST) -cpuprofile=cpu.prof -bench=. ./...
	$(STEP)
	@printf "Opening profile viewer at $(CYAN)http://localhost:8080$(RESET)\n"
	@$(GOCMD) tool pprof -http=:8080 cpu.prof

profile-mem:  ## Run memory profiling (opens browser)
	$(INFO)
	@printf "Running memory profiling...\n"
	@$(GOTEST) -memprofile=mem.prof -bench=. ./...
	$(STEP)
	@printf "Opening profile viewer at $(CYAN)http://localhost:8080$(RESET)\n"
	@$(GOCMD) tool pprof -http=:8080 mem.prof

# ═══════════════════════════════════════════════════════════════════════════════
# FUZZING
# ═══════════════════════════════════════════════════════════════════════════════

##@ Fuzzing

fuzz:  ## Run all fuzz tests (5s each, for CI)
	$(INFO)
	@printf "Running fuzz tests $(YELLOW)(5s each)$(RESET)...\n"
	@printf "\n$(BOLD)$(BLUE)◆ Capabilities$(RESET)\n"
	@go test -fuzz=^FuzzNetworkPatternMatching$$ -fuzztime=5s ./internal/domain/capabilities/
	@go test -fuzz=^FuzzFilesystemPatternMatching$$ -fuzztime=5s ./internal/domain/capabilities/
	@go test -fuzz=^FuzzExecPatternMatching$$ -fuzztime=5s ./internal/domain/capabilities/
	@go test -fuzz=^FuzzEnvironmentPatternMatching$$ -fuzztime=5s ./internal/domain/capabilities/
	@printf "\n$(BOLD)$(BLUE)◆ Config$(RESET)\n"
	@go test -fuzz=^FuzzYAMLLoading$$ -fuzztime=5s ./internal/infrastructure/config/
	@go test -fuzz=^FuzzVariableSubstitution$$ -fuzztime=5s ./internal/infrastructure/config/
	@printf "\n$(BOLD)$(BLUE)◆ Host Functions$(RESET)\n"
	@go test -fuzz=^FuzzHTTPRequestParsing$$ -fuzztime=5s ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzDNSRequestParsing$$ -fuzztime=5s ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzTCPRequestParsing$$ -fuzztime=5s ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzSMTPRequestParsing$$ -fuzztime=5s ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzPackedPtrLen$$ -fuzztime=5s ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzSSRFProtection$$ -fuzztime=5s ./internal/infrastructure/wasm/hostfuncs/
	@printf "\n$(BOLD)$(BLUE)◆ Validation$(RESET)\n"
	@go test -fuzz=^FuzzPluginNameValidation$$ -fuzztime=5s ./internal/infrastructure/validation/
	@go test -fuzz=^FuzzVersionValidation$$ -fuzztime=5s ./internal/infrastructure/validation/
	@go test -fuzz=^FuzzSchemaValidation$$ -fuzztime=5s ./internal/infrastructure/validation/
	@printf "\n$(BOLD)$(BLUE)◆ Redaction$(RESET)\n"
	@go test -fuzz=^FuzzRedactorScrubString$$ -fuzztime=5s ./internal/infrastructure/redaction/
	@go test -fuzz=^FuzzRedactorWalker$$ -fuzztime=5s ./internal/infrastructure/redaction/
	@printf "\n$(BOLD)$(BLUE)◆ Output$(RESET)\n"
	@go test -fuzz=^FuzzSARIFGeneration$$ -fuzztime=5s ./internal/infrastructure/output/
	$(SUCCESS)
	@printf "$(GREEN)Fuzz tests completed$(RESET)\n"

fuzz-extended:  ## Run extended fuzz tests (30m each)
	$(WARN)
	@printf "Running $(BOLD)extended$(RESET) fuzz tests $(YELLOW)(30m each)$(RESET)...\n"
	@printf "$(YELLOW)This will take several hours to complete!$(RESET)\n\n"
	@printf "$(BOLD)$(BLUE)◆ Capabilities$(RESET)\n"
	@go test -fuzz=^FuzzNetworkPatternMatching$$ -fuzztime=30m ./internal/domain/capabilities/
	@go test -fuzz=^FuzzFilesystemPatternMatching$$ -fuzztime=30m ./internal/domain/capabilities/
	@go test -fuzz=^FuzzExecPatternMatching$$ -fuzztime=30m ./internal/domain/capabilities/
	@go test -fuzz=^FuzzEnvironmentPatternMatching$$ -fuzztime=30m ./internal/domain/capabilities/
	@printf "\n$(BOLD)$(BLUE)◆ Config$(RESET)\n"
	@go test -fuzz=^FuzzYAMLLoading$$ -fuzztime=30m ./internal/infrastructure/config/
	@go test -fuzz=^FuzzVariableSubstitution$$ -fuzztime=30m ./internal/infrastructure/config/
	@printf "\n$(BOLD)$(BLUE)◆ Host Functions$(RESET)\n"
	@go test -fuzz=^FuzzHTTPRequestParsing$$ -fuzztime=30m ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzDNSRequestParsing$$ -fuzztime=30m ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzTCPRequestParsing$$ -fuzztime=30m ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzSMTPRequestParsing$$ -fuzztime=30m ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzPackedPtrLen$$ -fuzztime=30m ./internal/infrastructure/wasm/hostfuncs/
	@go test -fuzz=^FuzzSSRFProtection$$ -fuzztime=30m ./internal/infrastructure/wasm/hostfuncs/
	@printf "\n$(BOLD)$(BLUE)◆ Validation$(RESET)\n"
	@go test -fuzz=^FuzzPluginNameValidation$$ -fuzztime=30m ./internal/infrastructure/validation/
	@go test -fuzz=^FuzzVersionValidation$$ -fuzztime=30m ./internal/infrastructure/validation/
	@go test -fuzz=^FuzzSchemaValidation$$ -fuzztime=30m ./internal/infrastructure/validation/
	@printf "\n$(BOLD)$(BLUE)◆ Redaction$(RESET)\n"
	@go test -fuzz=^FuzzRedactorScrubString$$ -fuzztime=30m ./internal/infrastructure/redaction/
	@go test -fuzz=^FuzzRedactorWalker$$ -fuzztime=30m ./internal/infrastructure/redaction/
	@printf "\n$(BOLD)$(BLUE)◆ Output$(RESET)\n"
	@go test -fuzz=^FuzzSARIFGeneration$$ -fuzztime=30m ./internal/infrastructure/output/
	$(SUCCESS)
	@printf "$(GREEN)Extended fuzz tests completed$(RESET)\n"

# ═══════════════════════════════════════════════════════════════════════════════
# CLEANUP
# ═══════════════════════════════════════════════════════════════════════════════

##@ Cleanup

clean:  ## Remove build artifacts and generated files
	$(INFO)
	@printf "Cleaning build artifacts...\n"
	@$(GOCLEAN)
	@rm -rf bin/
	@rm -rf coverage.out coverage.html
	@rm -rf *.prof
	$(SUCCESS)
	@printf "$(GREEN)Clean complete$(RESET)\n"

# ═══════════════════════════════════════════════════════════════════════════════
# HELP
# ═══════════════════════════════════════════════════════════════════════════════

##@ Help

help:  ## Show this help message
	@printf "\n"
	@printf "$(BOLD)$(CYAN)╔════════════════════════════════════════════════════════════════╗$(RESET)\n"
	@printf "$(BOLD)$(CYAN)║$(RESET)                    $(BOLD)REGLET$(RESET) - Makefile Help                   $(BOLD)$(CYAN)║$(RESET)\n"
	@printf "$(BOLD)$(CYAN)╚════════════════════════════════════════════════════════════════╝$(RESET)\n"
	@printf "\n"
	@printf "$(BOLD)Usage:$(RESET) make $(CYAN)<target>$(RESET)\n\n"
	@awk 'BEGIN {FS = ":.*##"; section=""} \
		/^##@/ { \
			section=substr($$0, 5); \
			printf "\n$(BOLD)$(YELLOW)%s$(RESET)\n", section \
		} \
		/^[a-zA-Z_-]+:.*?##/ { \
			if (section != "") { \
				printf "  $(CYAN)%-18s$(RESET) %s\n", $$1, $$2 \
			} \
		}' $(MAKEFILE_LIST)
	@printf "\n"
	@printf "$(BOLD)Examples:$(RESET)\n"
	@printf "  make build         $(WHITE)# Build the binary$(RESET)\n"
	@printf "  make test          $(WHITE)# Run all tests$(RESET)\n"
	@printf "  make all           $(WHITE)# Full CI pipeline$(RESET)\n"
	@printf "\n"
