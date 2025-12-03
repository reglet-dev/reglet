.PHONY: build clean test

PLUGIN_NAME=file.wasm

build: ## Build plugin to WASM
	@echo "Building file plugin to WASM..."
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o $(PLUGIN_NAME) .
	@echo "Built: $(PLUGIN_NAME)"
	@ls -lh $(PLUGIN_NAME)

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -f $(PLUGIN_NAME)

test: ## Run plugin tests (Go tests, not WASM)
	@echo "Running tests..."
	go test -v ./...

help: ## Display this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
