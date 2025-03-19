# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build directory
BUILD_DIR=build

# Binary names
CONTROLLER_BINARY=controller
STORAGE_BINARY=storage
CLIENT_BINARY=client

.PHONY: all clean proto deps build test tools

# Main target
all: clean setup build

# Setup includes creating build dir, generating proto, and getting deps
setup: $(BUILD_DIR) proto deps

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Generate protocol buffer code
proto:
	@echo "Generating Protocol Buffer code..."
	protoc --go_out=. --go_opt=paths=source_relative proto/dfs.proto

# Get dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build all components
build: 
	@echo "Building controller..."
	$(GOBUILD) -o $(BUILD_DIR)/$(CONTROLLER_BINARY) ./controller
	@echo "Building storage node..."
	$(GOBUILD) -o $(BUILD_DIR)/$(STORAGE_BINARY) ./storage
	@echo "Building client..."
	$(GOBUILD) -o $(BUILD_DIR)/$(CLIENT_BINARY) ./client

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f proto/*.pb.go

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Install required tools
tools:
	@echo "Installing required tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Run targets for development
run-controller: all
	./$(BUILD_DIR)/$(CONTROLLER_BINARY) -port 8000

run-storage: all
	./$(BUILD_DIR)/$(STORAGE_BINARY) -id 8001 -controller localhost:8000 -data /tmp/dfs/storage1

run-client: all
	./$(BUILD_DIR)/$(CLIENT_BINARY) -controller localhost:8000

# Help target
help:
	@echo "Available targets:"
	@echo "  all          - Clean, setup, and build all components"
	@echo "  clean        - Remove build artifacts"
	@echo "  setup        - Create build dir, generate proto files, get dependencies"
	@echo "  proto        - Generate Go code from Protocol Buffer definitions"
	@echo "  deps         - Download and verify dependencies"
	@echo "  build        - Build all components"
	@echo "  test         - Run tests"
	@echo "  tools        - Install required tools"
	@echo "  run-controller - Build and run the controller"
	@echo "  run-storage    - Build and run a storage node"
	@echo "  run-client     - Build and run the client"
	@echo "  help         - Show this help message"