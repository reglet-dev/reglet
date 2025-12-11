.PHONY: build clean

PLUGIN_NAME = smtp
WASM_FILE = $(PLUGIN_NAME).wasm

build:
	@echo "Building $(PLUGIN_NAME) plugin to WASM..."
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o $(WASM_FILE) .
	@echo "Built: $(WASM_FILE)"
	@ls -lh $(WASM_FILE)

clean:
	rm -f $(WASM_FILE)
