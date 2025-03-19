package main

import (
	"net"
	"sync"
	"testing"
	"time"

	"distributed_file_system/common"
	pb "distributed_file_system/proto"
	"google.golang.org/protobuf/proto"
)

// mockStorageNode simulates a storage node for testing
type mockStorageNode struct {
	id        string
	freeSpace uint64
	requests  uint64
}

func (m *mockStorageNode) sendHeartbeat(conn net.Conn) error {
	heartbeat := &pb.Heartbeat{
		NodeId:           m.id,
		FreeSpace:       m.freeSpace,
		RequestsProcessed: m.requests,
	}
	data, err := proto.Marshal(heartbeat)
	if err != nil {
		return err
	}
	return common.WriteMessage(conn, common.MsgTypeHeartbeat, data)
}

func TestControllerStartup(t *testing.T) {
	controller := NewController(0) // Use port 0 for random available port
	
	// Start controller in goroutine
	errCh := make(chan error)
	go func() {
		errCh <- controller.Start()
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Verify controller is running
	if controller.listener == nil {
		t.Error("Controller listener not initialized")
	}

	// Clean up
	controller.listener.Close()
}

func TestNodeRegistrationAndHeartbeat(t *testing.T) {
	controller := NewController(0)
	
	// Start controller
	go controller.Start()
	time.Sleep(100 * time.Millisecond)
	
	addr := controller.listener.Addr().String()

	// Create mock storage node
	node := &mockStorageNode{
		id:        "test-node-1",
		freeSpace: 1024 * 1024 * 1024, // 1GB
		requests:  0,
	}

	// Connect to controller
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to controller: %v", err)
	}
	defer conn.Close()

	// Send heartbeat
	if err := node.sendHeartbeat(conn); err != nil {
		t.Fatalf("Failed to send heartbeat: %v", err)
	}

	// Verify node registration
	time.Sleep(100 * time.Millisecond)
	controller.mu.RLock()
	registeredNode, exists := controller.nodes[node.id]
	controller.mu.RUnlock()

	if !exists {
		t.Error("Node not registered with controller")
	}
	if registeredNode.FreeSpace != node.freeSpace {
		t.Errorf("Node free space mismatch: got %d, want %d", registeredNode.FreeSpace, node.freeSpace)
	}

	// Clean up
	controller.listener.Close()
}

func TestStorageRequest(t *testing.T) {
	controller := NewController(0)
	
	// Start controller
	go controller.Start()
	time.Sleep(100 * time.Millisecond)
	
	addr := controller.listener.Addr().String()

	// Register some storage nodes
	nodes := []*mockStorageNode{
		{id: "node-1", freeSpace: 2 * 1024 * 1024 * 1024}, // 2GB
		{id: "node-2", freeSpace: 3 * 1024 * 1024 * 1024}, // 3GB
		{id: "node-3", freeSpace: 1 * 1024 * 1024 * 1024}, // 1GB
	}

	// Register nodes
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(n *mockStorageNode) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("Failed to connect node %s: %v", n.id, err)
				return
			}
			defer conn.Close()
			if err := n.sendHeartbeat(conn); err != nil {
				t.Errorf("Failed to send heartbeat for node %s: %v", n.id, err)
			}
		}(node)
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// Send storage request
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to controller: %v", err)
	}
	defer conn.Close()

	request := &pb.StorageRequest{
		Filename:  "test.txt",
		FileSize:  1024 * 1024, // 1MB
		ChunkSize: 64 * 1024,   // 64KB
	}
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	if err := common.WriteMessage(conn, common.MsgTypeStorageRequest, data); err != nil {
		t.Fatalf("Failed to send storage request: %v", err)
	}

	// Read response
	msgType, respData, err := common.ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if msgType != common.MsgTypeStorageResponse {
		t.Errorf("Unexpected response type: got %d, want %d", msgType, common.MsgTypeStorageResponse)
	}

	response := &pb.StorageResponse{}
	if err := proto.Unmarshal(respData, response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Error != "" {
		t.Errorf("Storage request failed: %s", response.Error)
	}

	// Verify chunk placements
	expectedChunks := (request.FileSize + uint64(request.ChunkSize) - 1) / uint64(request.ChunkSize)
	if uint64(len(response.ChunkPlacements)) != expectedChunks {
		t.Errorf("Wrong number of chunk placements: got %d, want %d",
			len(response.ChunkPlacements), expectedChunks)
	}

	for _, placement := range response.ChunkPlacements {
		if len(placement.StorageNodes) != controller.replicationFactor {
			t.Errorf("Wrong number of replicas: got %d, want %d",
				len(placement.StorageNodes), controller.replicationFactor)
		}
	}

	// Clean up
	controller.listener.Close()
}

func TestNodeFailureDetection(t *testing.T) {
	controller := NewController(0)
	controller.heartbeatTimeout = 500 * time.Millisecond // Shorter timeout for testing
	
	// Start controller
	go controller.Start()
	time.Sleep(100 * time.Millisecond)
	
	addr := controller.listener.Addr().String()

	// Register a node
	node := &mockStorageNode{
		id:        "test-node-1",
		freeSpace: 1024 * 1024 * 1024,
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to controller: %v", err)
	}

	if err := node.sendHeartbeat(conn); err != nil {
		t.Fatalf("Failed to send heartbeat: %v", err)
	}

	// Verify node is registered
	time.Sleep(100 * time.Millisecond)
	controller.mu.RLock()
	_, exists := controller.nodes[node.id]
	controller.mu.RUnlock()
	if !exists {
		t.Fatal("Node not registered")
	}

	// Close connection to simulate node failure
	conn.Close()

	// Wait for node to be detected as failed
	time.Sleep(controller.heartbeatTimeout + 100*time.Millisecond)

	// Verify node is removed
	controller.mu.RLock()
	_, exists = controller.nodes[node.id]
	controller.mu.RUnlock()
	if exists {
		t.Error("Failed node not removed")
	}

	// Clean up
	controller.listener.Close()
}

func TestReplicationMaintenance(t *testing.T) {
	controller := NewController(0)
	
	// Start controller
	go controller.Start()
	time.Sleep(100 * time.Millisecond)
	
	addr := controller.listener.Addr().String()

	// Register nodes
	nodes := []*mockStorageNode{
		{id: "node-1", freeSpace: 1024 * 1024 * 1024},
		{id: "node-2", freeSpace: 1024 * 1024 * 1024},
		{id: "node-3", freeSpace: 1024 * 1024 * 1024},
		{id: "node-4", freeSpace: 1024 * 1024 * 1024},
	}

	// Connect and register nodes
	for _, node := range nodes {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to connect node %s: %v", node.id, err)
		}
		defer conn.Close()
		if err := node.sendHeartbeat(conn); err != nil {
			t.Fatalf("Failed to send heartbeat for node %s: %v", node.id, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Simulate storing a file
	controller.mu.Lock()
	controller.files["test.txt"] = &FileMetadata{
		Size:      1024,
		ChunkSize: 64,
		Chunks: map[int][]string{
			0: {"node-1", "node-2", "node-3"},
		},
	}
	controller.mu.Unlock()

	// Simulate node failure
	controller.mu.Lock()
	delete(controller.nodes, "node-1")
	controller.mu.Unlock()

	// Wait for replication to be triggered
	time.Sleep(2 * time.Second)

	// Verify replication was maintained
	controller.mu.RLock()
	metadata := controller.files["test.txt"]
	nodes := metadata.Chunks[0]
	controller.mu.RUnlock()

	if len(nodes) != controller.replicationFactor {
		t.Errorf("Replication not maintained: got %d replicas, want %d",
			len(nodes), controller.replicationFactor)
	}

	// Clean up
	controller.listener.Close()
}