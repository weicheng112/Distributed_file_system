package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"distributed_file_system/common"
	pb "distributed_file_system/proto"
	"google.golang.org/protobuf/proto"
)

// mockController simulates a controller for testing
type mockController struct {
	listener net.Listener
	nodes    map[string]bool
}

func newMockController(t *testing.T) *mockController {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create mock controller: %v", err)
	}

	mc := &mockController{
		listener: listener,
		nodes:    make(map[string]bool),
	}

	go mc.handleConnections(t)
	return mc
}

func (mc *mockController) handleConnections(t *testing.T) {
	for {
		conn, err := mc.listener.Accept()
		if err != nil {
			return // listener closed
		}
		go mc.handleConnection(t, conn)
	}
}

func (mc *mockController) handleConnection(t *testing.T, conn net.Conn) {
	defer conn.Close()

	for {
		msgType, data, err := common.ReadMessage(conn)
		if err != nil {
			return
		}

		switch msgType {
		case common.MsgTypeHeartbeat:
			heartbeat := &pb.Heartbeat{}
			if err := proto.Unmarshal(data, heartbeat); err != nil {
				t.Errorf("Failed to unmarshal heartbeat: %v", err)
				return
			}
			mc.nodes[heartbeat.NodeId] = true
		}
	}
}

func TestStorageNodeStartup(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create temporary directory for storage
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage node
	node := NewStorageNode("test-node", mc.listener.Addr().String(), tmpDir)

	// Start node in goroutine
	errCh := make(chan error)
	go func() {
		errCh <- node.Start()
	}()

	// Give it time to start and send first heartbeat
	time.Sleep(100 * time.Millisecond)

	// Verify node registered with controller
	if !mc.nodes["test-node"] {
		t.Error("Node failed to register with controller")
	}

	// Verify data directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("Data directory not created")
	}

	// Clean up
	if node.listener != nil {
		node.listener.Close()
	}
}

func TestChunkStorageAndRetrieval(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and start storage node
	node := NewStorageNode("test-node", mc.listener.Addr().String(), tmpDir)
	go node.Start()
	time.Sleep(100 * time.Millisecond)

	// Connect to storage node
	conn, err := net.Dial("tcp", "localhost:"+node.nodeID)
	if err != nil {
		t.Fatalf("Failed to connect to storage node: %v", err)
	}
	defer conn.Close()

	// Create test data
	testData := []byte("test chunk data")
	filename := "test.txt"
	chunkNum := 0

	// Create store request
	request := &pb.ChunkStoreRequest{
		Filename:    filename,
		ChunkNumber: uint32(chunkNum),
		Data:       testData,
	}

	// Serialize and send request
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	if err := common.WriteMessage(conn, common.MsgTypeChunkStore, data); err != nil {
		t.Fatalf("Failed to send store request: %v", err)
	}

	// Read response
	msgType, respData, err := common.ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if msgType != common.MsgTypeChunkStore {
		t.Errorf("Unexpected response type: got %d, want %d", msgType, common.MsgTypeChunkStore)
	}

	response := &pb.ChunkStoreResponse{}
	if err := proto.Unmarshal(respData, response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Success {
		t.Errorf("Chunk storage failed: %s", response.Error)
	}

	// Verify chunk was stored
	chunkPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%d", filename, chunkNum))
	if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
		t.Error("Chunk file not created")
	}

	// Try to retrieve the chunk
	retrieveReq := &pb.ChunkRetrieveRequest{
		Filename:    filename,
		ChunkNumber: uint32(chunkNum),
	}

	data, err = proto.Marshal(retrieveReq)
	if err != nil {
		t.Fatalf("Failed to marshal retrieve request: %v", err)
	}

	if err := common.WriteMessage(conn, common.MsgTypeChunkRetrieve, data); err != nil {
		t.Fatalf("Failed to send retrieve request: %v", err)
	}

	// Read retrieve response
	msgType, respData, err = common.ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read retrieve response: %v", err)
	}

	if msgType != common.MsgTypeChunkRetrieve {
		t.Errorf("Unexpected retrieve response type: got %d, want %d", msgType, common.MsgTypeChunkRetrieve)
	}

	retrieveResp := &pb.ChunkRetrieveResponse{}
	if err := proto.Unmarshal(respData, retrieveResp); err != nil {
		t.Fatalf("Failed to unmarshal retrieve response: %v", err)
	}

	if !bytes.Equal(retrieveResp.Data, testData) {
		t.Error("Retrieved data does not match stored data")
	}

	// Clean up
	if node.listener != nil {
		node.listener.Close()
	}
}

func TestCorruptionDetection(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and start storage node
	node := NewStorageNode("test-node", mc.listener.Addr().String(), tmpDir)
	go node.Start()
	time.Sleep(100 * time.Millisecond)

	// Store a chunk
	filename := "test.txt"
	chunkNum := 0
	testData := []byte("test chunk data")

	if err := node.storeChunk(filename, chunkNum, testData, common.CalculateChecksum(testData)); err != nil {
		t.Fatalf("Failed to store chunk: %v", err)
	}

	// Corrupt the chunk file
	chunkPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%d", filename, chunkNum))
	file, err := os.OpenFile(chunkPath, os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open chunk file: %v", err)
	}

	// Skip past checksum and corrupt the data
	file.Seek(32, 0) // SHA-256 checksum is 32 bytes
	file.Write([]byte("corrupted data"))
	file.Close()

	// Try to retrieve the chunk
	_, err = node.retrieveChunk(filename, chunkNum)
	if _, ok := err.(common.ChunkCorruptionError); !ok {
		t.Errorf("Expected ChunkCorruptionError, got %v", err)
	}

	// Clean up
	if node.listener != nil {
		node.listener.Close()
	}
}

func TestMetadataPersistence(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and store some chunks
	node := NewStorageNode("test-node", mc.listener.Addr().String(), tmpDir)
	testData := []byte("test chunk data")
	checksum := common.CalculateChecksum(testData)

	for i := 0; i < 3; i++ {
		if err := node.storeChunk(fmt.Sprintf("test%d.txt", i), 0, testData, checksum); err != nil {
			t.Fatalf("Failed to store chunk: %v", err)
		}
	}

	// Save metadata
	if err := node.saveMetadata(); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Create new node instance and load metadata
	node2 := NewStorageNode("test-node", mc.listener.Addr().String(), tmpDir)
	if err := node2.loadMetadata(); err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Verify metadata was loaded correctly
	if len(node2.chunks) != 3 {
		t.Errorf("Wrong number of chunks loaded: got %d, want 3", len(node2.chunks))
	}

	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("test%d.txt_0", i)
		if _, exists := node2.chunks[key]; !exists {
			t.Errorf("Chunk %s not loaded", key)
		}
	}
}