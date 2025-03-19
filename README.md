# Distributed File System

A distributed file system implementation supporting parallel storage/retrieval, fault tolerance, and corruption recovery.

## Features

- Parallel storage and retrieval of files
- Configurable chunk size for file splitting
- 3x replication for fault tolerance
- Automatic corruption detection and recovery
- Pipeline replication for efficient data transfer
- Interactive command-line interface
- Protocol Buffer message serialization

## Components

### Controller

- Manages metadata and coordinates storage nodes
- Handles node registration and health monitoring
- Maintains file chunk mappings
- Coordinates replication and recovery

### Storage Node

- Stores and manages file chunks
- Performs checksum verification
- Handles chunk replication
- Sends regular heartbeats to controller
- Monitors disk space and request statistics

### Client

- Splits files into chunks for storage
- Retrieves files in parallel
- Provides interactive command interface
- Supports file listing and deletion
- Shows system status and statistics

## Requirements

- Go 1.21 or later
- Protocol Buffers compiler (protoc)
- Make (for building)

## Building

1. Install Protocol Buffers compiler:

   ```bash
   # For Windows
   choco install protoc

   # For Linux
   apt-get install protobuf-compiler

   # For macOS
   brew install protobuf
   ```

2. Install Go dependencies and build:
   ```bash
   make tools  # Install required Go tools
   make all    # Generate proto code, get dependencies, and build
   ```

## Running

1. Start the controller:

   ```bash
   ./build/controller -port 8000
   ```

2. Start storage nodes (run multiple instances):

   ```bash
   ./build/storage -id 8001 -controller localhost:8000 -data /path/to/storage1
   ./build/storage -id 8002 -controller localhost:8000 -data /path/to/storage2
   ./build/storage -id 8003 -controller localhost:8000 -data /path/to/storage3
   ```

3. Run the client:
   ```bash
   ./build/client -controller localhost:8000
   ```

## Client Commands

1. Store a file:

   ```
   store <filepath> [chunk_size]
   ```

   - `filepath`: Path to the file to store
   - `chunk_size`: Optional chunk size in bytes (default: 64MB)

2. Retrieve a file:

   ```
   retrieve <filename> <output_path>
   ```

   - `filename`: Name of the file to retrieve
   - `output_path`: Where to save the retrieved file

3. List files:

   ```
   list
   ```

   Shows all files stored in the system

4. Delete a file:

   ```
   delete <filename>
   ```

   Removes a file from the system

5. Show system status:

   ```
   status
   ```

   Displays storage node information and system statistics

6. Exit the client:
   ```
   exit
   ```

## Testing

Run the test suite:

```bash
make test
```

## Design Decisions

See [DESIGN.md](DESIGN.md) for detailed information about:

- System architecture
- Component interactions
- Message protocols
- Replication strategy
- Failure handling
- Performance considerations

## Error Handling

The system handles various error conditions:

- Node failures (up to 2 concurrent failures)
- Network issues
- Disk corruption
- Storage space exhaustion
- Invalid requests

## Limitations

- Single controller (potential bottleneck)
- No support for file modifications
- Basic replication strategy
- No security features
- No compression

## Future Improvements

- Add rack awareness
- Implement sophisticated load balancing
- Add compression support
- Support variable replication factors
- Add append operations
- Implement quota management
- Add security features
