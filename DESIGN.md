# Distributed File System Design Document

## Components

### 1. Controller

- Acts as the central coordinator (similar to HDFS NameNode)
- Maintains metadata:
  - Active storage nodes and their status
  - File to chunk mappings
  - Chunk locations and replicas
- Handles:
  - Storage node registration and health monitoring
  - Chunk placement decisions
  - Replication management
  - Failure detection and recovery

### 2. Storage Node

- Responsible for storing and managing chunks
- Features:
  - Chunk storage with checksums
  - Corruption detection and recovery
  - Replica forwarding
  - Regular heartbeats to controller
  - Disk space monitoring
  - Request tracking

### 3. Client

- Provides user interface to the DFS
- Features:
  - File chunking and storage
  - Parallel file retrieval
  - File listing and deletion
  - Node status viewing
  - Interactive command-line interface

## Design Decisions

### 1. Chunk Size

- Default: 64MB
- Rationale:
  - Large enough to minimize metadata overhead
  - Small enough for efficient parallel transfer
  - User-configurable for specific needs

### 2. Replication Strategy

- 3x replication (as per requirements)
- Replica Placement:
  - Primary copy on node with most available space
  - Secondary copies on different nodes for fault tolerance
  - Pipeline replication: client → node1 → node2 → node3
  - Geographic distribution when possible (using node IDs)

### 3. Failure Detection

- Heartbeat Mechanism:
  - 5-second intervals
  - 15-second timeout for node failure detection
- Recovery Process:
  - Immediate re-replication when node failure detected
  - Prioritize chunks with fewer replicas
  - Balance load across remaining nodes

### 4. Corruption Handling

- SHA-256 checksums for each chunk
- Verification on every read
- Automatic repair using replicas
- Checksum stored with chunk data on disk

### 5. Message Protocol

- Custom protocol using Protocol Buffers
- Message Format:
  - Header: [Type (1 byte)][Length (4 bytes)]
  - Body: Serialized protobuf message
- Efficient binary serialization
- Language-agnostic format

## Message Communication

### Controller Messages

1. Storage Node Registration

   - Node sends: ID, total space, available space
   - Controller responds: Registration confirmation

2. Heartbeat

   - Node sends: ID, available space, requests handled, new files
   - Controller processes: Updates node status

3. Storage Request

   - Client sends: Filename, size, chunk size
   - Controller responds: Chunk placement map

4. Retrieval Request
   - Client sends: Filename
   - Controller responds: Chunk locations

### Storage Node Messages

1. Chunk Storage

   - Receives: Chunk data, checksum, replica list
   - Forwards to replicas in pipeline
   - Responds: Success/failure

2. Chunk Retrieval
   - Receives: Chunk identifier
   - Verifies checksum
   - Responds: Chunk data or error

### Client Messages

1. File Storage

   - Coordinates with controller
   - Splits file into chunks
   - Transfers chunks to assigned nodes

2. File Retrieval
   - Gets chunk locations from controller
   - Parallel chunk retrieval
   - Reassembles file

## Performance Considerations

1. Parallel Operations

   - Multi-threaded chunk transfers
   - Pipeline replication
   - Parallel file retrieval

2. Load Balancing

   - Distribute chunks based on available space
   - Consider node load in placement decisions
   - Balance replica distribution

3. Network Efficiency
   - Pipeline replication reduces network usage
   - Batch operations when possible
   - Local reads preferred

## Retrospective Questions

### 1. What were the main challenges in implementing this system?

- Coordinating distributed components
- Handling node failures gracefully
- Ensuring data consistency with replication
- Managing parallel operations effectively

### 2. How would you improve the system?

- Add rack awareness for better replica placement
- Implement more sophisticated load balancing
- Add compression support
- Support variable replication factors
- Add support for append operations
- Implement quota management

### 3. What are the system's limitations?

- Single controller is a potential bottleneck
- No support for file modifications (append/update)
- Limited security features
- Basic replication strategy
- No support for small files optimization

### 4. What were the key learning points?

- Importance of proper error handling
- Complexity of distributed systems coordination
- Value of clear protocol definitions
- Significance of proper testing in distributed environments
