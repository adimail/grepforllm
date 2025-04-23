APP_NAME=fs
CMD_PATH=cmd/main.go
DIST_DIR=dist
LDFLAGS=-ldflags="-s -w"

build:
	@echo "Building for local platform..."
	@mkdir -p bin
	@go build -o bin/$(APP_NAME) $(CMD_PATH)

run: build
	@echo "Running local build..."
	@./bin/$(APP_NAME)

# --- Release Build Targets ---
$(DIST_DIR):
	@mkdir -p $(DIST_DIR)

# Build for Linux (amd64)
build-linux: $(DIST_DIR)
	@echo "Building for Linux (amd64)..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 $(CMD_PATH)

# Build for Windows (amd64)
build-windows: $(DIST_DIR)
	@echo "Building for Windows (amd64)..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_PATH)

# Build for macOS (amd64 - Intel)
build-mac-intel: $(DIST_DIR)
	@echo "Building for macOS (amd64 - Intel)..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-macos-amd64 $(CMD_PATH)

# Build for macOS (arm64 - Apple Silicon)
build-mac-arm: $(DIST_DIR)
	@echo "Building for macOS (arm64 - Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-macos-arm64 $(CMD_PATH)

# Build all release binaries
build-all: build-linux build-windows build-mac-intel build-mac-arm
	@echo "All release binaries built in $(DIST_DIR)/"

# --- Cleanup ---

clean:
	@echo "Cleaning up build artifacts..."
	@rm -rf bin $(DIST_DIR)

.PHONY: build run build-linux build-windows build-mac-intel build-mac-arm build-all clean $(DIST_DIR)
