.PHONY: build test clean build-all

BINARY_NAME=deepfind
DIST_DIR=dist

# Default: build for the host operating system
build:
	@echo "Building native binary..."
	go build -o $(BINARY_NAME) main.go
	@echo "Build complete. Run ./$(BINARY_NAME) to start."

# Run all unit tests
test:
	@echo "Running unit tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BINARY_NAME) $(BINARY_NAME).exe $(DIST_DIR)
	@echo "Clean complete."

# Cross-compile for multiplatform support (macOS, Windows, RHEL Linux)
build-all: clean
	@echo "Creating distribution directory: $(DIST_DIR)"
	mkdir -p $(DIST_DIR)
	
	@echo "Compiling for macOS Darwin (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/$(BINARY_NAME)_darwin_arm64 main.go
	
	@echo "Compiling for macOS Darwin (Intel)..."
	GOOS=darwin GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)_darwin_amd64 main.go
	
	@echo "Compiling for Windows (64-bit)..."
	GOOS=windows GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)_windows_amd64.exe main.go
	
	@echo "Compiling for Linux RHEL (64-bit)..."
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)_linux_amd64 main.go
	
	@echo "Cross-compilation complete. Binaries are stored in $(DIST_DIR)/"
