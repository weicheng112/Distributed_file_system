package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"distributed_file_system/common"
	pb "distributed_file_system/proto"
	"google.golang.org/protobuf/proto"
)

// mockController simulates a controller for testing
type mockController struct {
	listener net.Listener
	files    map[string]*pb.FileInfo
	nodes    []*pb.NodeInfo
}

func newMockController(t *testing.T) *mockController {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create mock controller: %v", err)
	}

	mc := &mockController{
		listener: listener,
		files:    make(map[string]*pb.FileInfo),
		nodes: []*pb.NodeInfo{
			{NodeId: "node1", FreeSpace: 1024 * 1024 * 1024, RequestsProcessed: 100},
			{NodeId: "node2", FreeSpace: 2 * 1024 * 1024 * 1024, RequestsProcessed: 200},
			{NodeId: "node3", FreeSpace: 3 * 1024 * 1024 * 1024, RequestsProcessed: 300},
		},
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

	msgType, data, err := common.ReadMessage(conn)
	if err != nil {
		return
	}

	switch msgType {
	case common.MsgTypeStorageRequest:
		req := &pb.StorageRequest{}
		if err := proto.Unmarshal(data, req); err != nil {
			t.Errorf("Failed to unmarshal storage request: %v", err)
			return
		}

		// Create response with mock storage locations
		resp := &pb.StorageResponse{
			ChunkPlacements: make([]*pb.ChunkPlacement, 1),
		}
		resp.ChunkPlacements[0] = &pb.ChunkPlacement{
			ChunkNumber:   0,
			StorageNodes: []string{"node1", "node2", "node3"},
		}

		// Add file to mock storage
		mc.files[req.Filename] = &pb.FileInfo{
			Filename:  req.Filename,
			Size:     req.FileSize,
			NumChunks: 1,
		}

		respData, _ := proto.Marshal(resp)
		common.WriteMessage(conn, common.MsgTypeStorageResponse, respData)

	case common.MsgTypeRetrievalRequest:
		req := &pb.RetrievalRequest{}
		if err := proto.Unmarshal(data, req); err != nil {
			t.Errorf("Failed to unmarshal retrieval request: %v", err)
			return
		}

		resp := &pb.RetrievalResponse{
			Chunks: []*pb.ChunkLocation{
				{
					ChunkNumber:   0,
					StorageNodes: []string{"node1", "node2", "node3"},
				},
			},
		}

		respData, _ := proto.Marshal(resp)
		common.WriteMessage(conn, common.MsgTypeRetrievalResponse, respData)

	case common.MsgTypeListRequest:
		resp := &pb.ListFilesResponse{
			Files: make([]*pb.FileInfo, 0, len(mc.files)),
		}
		for _, file := range mc.files {
			resp.Files = append(resp.Files, file)
		}

		respData, _ := proto.Marshal(resp)
		common.WriteMessage(conn, common.MsgTypeListResponse, respData)

	case common.MsgTypeNodeStatusRequest:
		resp := &pb.NodeStatusResponse{
			Nodes:      mc.nodes,
			TotalSpace: 6 * 1024 * 1024 * 1024, // 6GB
		}

		respData, _ := proto.Marshal(resp)
		common.WriteMessage(conn, common.MsgTypeNodeStatusResponse, respData)

	case common.MsgTypeDeleteRequest:
		req := &pb.DeleteRequest{}
		if err := proto.Unmarshal(data, req); err != nil {
			t.Errorf("Failed to unmarshal delete request: %v", err)
			return
		}

		delete(mc.files, req.Filename)

		resp := &pb.DeleteResponse{Success: true}
		respData, _ := proto.Marshal(resp)
		common.WriteMessage(conn, common.MsgTypeDeleteResponse, respData)
	}
}

func TestFileStorage(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create client
	client := NewClient(mc.listener.Addr().String())

	// Create test file
	tmpFile, err := os.CreateTemp("", "test_file_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testData := []byte("test file content")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	// Store file
	if err := client.storeFile(tmpFile.Name(), 64*1024); err != nil {
		t.Fatalf("Failed to store file: %v", err)
	}

	// Verify file was stored
	if _, exists := mc.files[filepath.Base(tmpFile.Name())]; !exists {
		t.Error("File not stored in mock controller")
	}
}

func TestFileRetrieval(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create client
	client := NewClient(mc.listener.Addr().String())

	// Add mock file
	filename := "test.txt"
	mc.files[filename] = &pb.FileInfo{
		Filename:  filename,
		Size:     1024,
		NumChunks: 1,
	}

	// Create output file
	tmpFile, err := os.CreateTemp("", "retrieved_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Retrieve file
	if err := client.retrieveFile(filename, tmpFile.Name()); err != nil {
		t.Fatalf("Failed to retrieve file: %v", err)
	}
}

func TestFileList(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create client
	client := NewClient(mc.listener.Addr().String())

	// Add mock files
	mc.files["file1.txt"] = &pb.FileInfo{Filename: "file1.txt", Size: 1024, NumChunks: 1}
	mc.files["file2.txt"] = &pb.FileInfo{Filename: "file2.txt", Size: 2048, NumChunks: 2}

	// List files
	files, err := client.listFiles()
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Wrong number of files: got %d, want 2", len(files))
	}

	for _, file := range files {
		if _, exists := mc.files[file.Filename]; !exists {
			t.Errorf("Listed file %s not in mock storage", file.Filename)
		}
	}
}

func TestNodeStatus(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create client
	client := NewClient(mc.listener.Addr().String())

	// Get node status
	status, err := client.getNodeStatus()
	if err != nil {
		t.Fatalf("Failed to get node status: %v", err)
	}

	if len(status.Nodes) != len(mc.nodes) {
		t.Errorf("Wrong number of nodes: got %d, want %d", len(status.Nodes), len(mc.nodes))
	}

	if status.TotalSpace != 6*1024*1024*1024 {
		t.Errorf("Wrong total space: got %d, want %d", status.TotalSpace, 6*1024*1024*1024)
	}
}

func TestCommandLineInterface(t *testing.T) {
	// Create mock controller
	mc := newMockController(t)
	defer mc.listener.Close()

	// Create client
	client := NewClient(mc.listener.Addr().String())

	// Test commands
	commands := []struct {
		input    string
		expected string
	}{
		{"help", "DFS Client Commands"},
		{"list", "Files in DFS"},
		{"status", "Storage Node Status"},
		{"invalid", "Unknown command"},
		{"exit", "Goodbye"},
	}

	for _, cmd := range commands {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Process command
		if cmd.input == "exit" {
			// Handle exit command specially
			go func() {
				client.runInteractive()
			}()
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintln(w, cmd.input)
		} else {
			fmt.Fprintln(w, cmd.input)
		}

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, cmd.expected) {
			t.Errorf("Command %q output %q does not contain %q", cmd.input, output, cmd.expected)
		}
	}
}