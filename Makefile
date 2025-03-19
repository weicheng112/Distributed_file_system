# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Protoc parameters
PROTOC=protoc
PROTO_GO_OUT=proto

# Binary names
CONTROLLER_BINARY=controller.exe
STORAGE_BINARY=storage.exe
CLIENT_BINARY=client.exe

# Source directories
CONTROLLER_DIR=controller
STORAGE_DIR=storage
CLIENT_DIR=client

# Build directory
BUILD_DIR=build

.PHONY: all proto deps build clean test controller storage client

all: proto deps build

# Generate protocol buffer code
proto:
	@echo "Generating Protocol Buffer code..."
	$(PROTOC) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/dfs.proto

# Get dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go
	$(GOGET) google.golang.org/grpc/cmd/protoc-gen-go-grpc
	$(GOMOD) tidy

# Build all components
build: controller storage client

# Build controller
controller:
	@echo "Building controller..."
	$(GOBUILD) -o $(BUILD_DIR)/$(CONTROLLER_BINARY) ./$(CONTROLLER_DIR)

# Build storage node
storage:
	@echo "Building storage node..."
	$(GOBUILD) -o $(BUILD_DIR)/$(STORAGE_BINARY) ./$(STORAGE_DIR)

# Build client
client:
	@echo "Building client..."
	$(GOBUILD) -o $(BUILD_DIR)/$(CLIENT_BINARY) ./$(CLIENT_DIR)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f $(PROTO_GO_OUT)/*.pb.go

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Create build directory if it doesn't exist
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Install required tools
.PHONY: tools
tools:
	@echo "Installing required tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Run controller (for development)
.PHONY: run-controller
run-controller: controller
	./$(BUILD_DIR)/$(CONTROLLER_BINARY)

# Run storage node (for development)
.PHONY: run-storage
run-storage: storage
	./$(BUILD_DIR)/$(STORAGE_BINARY)

# Run client (for development)
.PHONY: run-client
run-client: client
	./$(BUILD_DIR)/$(CLIENT_BINARY)

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Generate proto files, get dependencies, and build all components"
	@echo "  proto        - Generate Go code from Protocol Buffer definitions"
	@echo "  deps         - Download and verify dependencies"
	@echo "  build        - Build all components"
	@echo "  controller   - Build only the controller"
	@echo "  storage      - Build only the storage node"
	@echo "  client       - Build only the client"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  tools        - Install required tools"
	@echo "  run-controller - Build and run the controller"
	@echo "  run-storage    - Build and run a storage node"
	@echo "  run-client     - Build and run the client"
	@echo "  help         - Show this help message"