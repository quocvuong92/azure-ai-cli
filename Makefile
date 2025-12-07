BINARY_NAME=azure-ai
VERSION=0.1.0
BUILD_DIR=bin

.PHONY: build build-compressed build-all build-all-compressed clean install tidy run help

# Build for current platform
build:
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build and compress for current platform
build-compressed: build
	gzexe $(BUILD_DIR)/$(BINARY_NAME)
	rm -f $(BUILD_DIR)/$(BINARY_NAME)~

# Install to ~/go/bin
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) ~/go/bin/

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Download dependencies
tidy:
	go mod tidy

# Cross-compile for multiple platforms
build-all:
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Build and compress for all platforms
build-all-compressed: build-all
	gzexe $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	gzexe $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	gzexe $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	rm -f $(BUILD_DIR)/*~
	@echo "Note: Windows binary not compressed (gzexe not supported)"

# Run the CLI
run:
	go run . $(ARGS)

# Show help
help:
	@echo "Available targets:"
	@echo "  build               - Build for current platform"
	@echo "  build-compressed    - Build and compress for current platform"
	@echo "  install             - Build and install to ~/go/bin"
	@echo "  clean               - Remove build artifacts"
	@echo "  tidy                - Download dependencies"
	@echo "  build-all           - Cross-compile for all platforms"
	@echo "  build-all-compressed - Cross-compile and compress all platforms"
	@echo "  run                 - Run the CLI (use ARGS=\"query\" to pass arguments)"
