# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build directory
BUILD_DIR=build

# Binary names (no .exe extension for Linux)
CONTROLLER_BINARY=controller
STORAGE_BINARY=storage
CLIENT_BINARY=client

# Source directories
CONTROLLER_DIR=./controller
STORAGE_DIR=./storage
CLIENT_DIR=./client

.PHONY: all clean proto deps build test controller storage client

all: clean $(BUILD_DIR) proto deps build

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
build: controller storage client

# Build controller
controller: $(BUILD_DIR)
	@echo "Building controller..."
	$(GOBUILD) -o $(BUILD_DIR)/$(CONTROLLER_BINARY) $(CONTROLLER_DIR)

# Build storage node
storage: $(BUILD_DIR)
	@echo "Building storage node..."
	$(GOBUILD) -o $(BUILD_DIR)/$(STORAGE_BINARY) $(STORAGE_DIR)

# Build client
client: $(BUILD_DIR)
	@echo "Building client..."
	$(GOBUILD) -o $(BUILD_DIR)/$(CLIENT_BINARY) $(CLIENT_DIR)

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

# Run controller (for development)
run-controller: controller
	./$(BUILD_DIR)/$(CONTROLLER_BINARY) -port 8000

# Run storage node (for development)
run-storage: storage
	./$(BUILD_DIR)/$(STORAGE_BINARY) -id 8001 -controller localhost:8000 -data /tmp/dfs/storage1

# Run client (for development)
run-client: client
	./$(BUILD_DIR)/$(CLIENT_BINARY) -controller localhost:8000

# Help target
help:
	@echo "Available targets:"
	@echo "  all          - Clean, create build dir, generate proto files, get dependencies, and build all components"
	@echo "  clean        - Remove build artifacts"
	@echo "  proto        - Generate Go code from Protocol Buffer definitions"
	@echo "  deps         - Download and verify dependencies"
	@echo "  build        - Build all components"
	@echo "  controller   - Build only the controller"
	@echo "  storage      - Build only the storage node"
	@echo "  client       - Build only the client"
	@echo "  test         - Run tests"
	@echo "  tools        - Install required tools"
	@echo "  run-controller - Build and run the controller"
	@echo "  run-storage    - Build and run a storage node"
	@echo "  run-client     - Build and run the client"
	@echo "  help         - Show this help message"