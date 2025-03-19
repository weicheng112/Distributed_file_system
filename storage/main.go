package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"distributed_file_system/common"
	pb "distributed_file_system/proto"
)

// ChunkMetadata stores information about a stored chunk
type ChunkMetadata struct {
	Filename    string
	ChunkNumber int
	Size        int64
	Checksum    []byte
	Replicas    []string // List of nodes that have replicas
}

// StorageNode handles chunk storage and retrieval
type StorageNode struct {
	mu sync.RWMutex

	// Configuration
	nodeID         string
	controllerAddr string
	dataDir        string

	// Connection to controller
	controllerConn net.Conn

	// Chunk metadata
	chunks map[string]*ChunkMetadata // Key: filename_chunknumber

	// Statistics
	freeSpace       uint64
	requestsHandled uint64

	// Network
	listener net.Listener

	// Track reported files
	reportedFiles map[string]bool
}

func NewStorageNode(nodeID, controllerAddr, dataDir string) *StorageNode {
	return &StorageNode{
		nodeID:         nodeID,
		controllerAddr: controllerAddr,
		dataDir:        dataDir,
		chunks:         make(map[string]*ChunkMetadata),
		reportedFiles:  make(map[string]bool),
	}
}

func (n *StorageNode) Start() error {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(n.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Load existing chunks metadata
	if err := n.loadMetadata(); err != nil {
		return fmt.Errorf("failed to load metadata: %v", err)
	}

	// Connect to controller
	if err := n.connectToController(); err != nil {
		return fmt.Errorf("failed to connect to controller: %v", err)
	}

	// Start heartbeat
	go n.sendHeartbeats()

	// Start listener for chunk operations
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", n.nodeID))
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}
	n.listener = listener

	log.Printf("Storage node started. ID: %s, Data dir: %s", n.nodeID, n.dataDir)

	// Accept and handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go n.handleConnection(conn)
	}
}

func (n *StorageNode) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		// Read message type and data
		msgType, data, err := common.ReadMessage(conn)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			return
		}

		var response []byte
		var respErr error

		// Handle different message types
		switch msgType {
		case common.MsgTypeChunkStore:
			response, respErr = n.handleChunkStore(data)
		case common.MsgTypeChunkRetrieve:
			response, respErr = n.handleChunkRetrieve(data)
		default:
			respErr = &common.ProtocolError{Message: fmt.Sprintf("unknown message type: %d", msgType)}
		}

		if respErr != nil {
			log.Printf("Error handling message type %d: %v", msgType, respErr)
			// Send error response if applicable
			if response != nil {
				if err := common.WriteMessage(conn, msgType, response); err != nil {
					log.Printf("Error sending error response: %v", err)
				}
			}
			return
		}

		// Send response if one was generated
		if response != nil {
			if err := common.WriteMessage(conn, msgType, response); err != nil {
				log.Printf("Error sending response: %v", err)
				return
			}
		}
	}
}

func (n *StorageNode) storeChunk(filename string, chunkNum int, data []byte, checksum []byte) error {
	// Create chunk file
	chunkPath := filepath.Join(n.dataDir, fmt.Sprintf("%s_%d", filename, chunkNum))
	file, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %v", err)
	}
	defer file.Close()

	// Write checksum and data
	if err := binary.Write(file, binary.LittleEndian, checksum); err != nil {
		return fmt.Errorf("failed to write checksum: %v", err)
	}
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write chunk data: %v", err)
	}

	// Update metadata
	n.mu.Lock()
	n.chunks[fmt.Sprintf("%s_%d", filename, chunkNum)] = &ChunkMetadata{
		Filename:    filename,
		ChunkNumber: chunkNum,
		Size:        int64(len(data)),
		Checksum:    checksum,
	}
	n.requestsHandled++
	n.mu.Unlock()

	// Save metadata to disk
	if err := n.saveMetadata(); err != nil {
		log.Printf("Warning: failed to save metadata: %v", err)
	}

	return nil
}

func (n *StorageNode) retrieveChunk(filename string, chunkNum int) ([]byte, error) {
	chunkPath := filepath.Join(n.dataDir, fmt.Sprintf("%s_%d", filename, chunkNum))
	file, err := os.Open(chunkPath)
	if err != nil {
		return nil, &common.ChunkNotFoundError{
			Filename: filename,
			ChunkNum: chunkNum,
		}
	}
	defer file.Close()

	// Read stored checksum
	var storedChecksum [32]byte
	if err := binary.Read(file, binary.LittleEndian, &storedChecksum); err != nil {
		return nil, fmt.Errorf("failed to read checksum: %v", err)
	}

	// Read data
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk data: %v", err)
	}

	// Skip the checksum bytes from the data
	data = data[32:]

	// Verify checksum
	checksum := common.CalculateChecksum(data)
	if !common.VerifyChecksum(data, storedChecksum[:]) {
		return nil, &common.ChunkCorruptionError{
			Filename: filename,
			ChunkNum: chunkNum,
		}
	}

	n.mu.Lock()
	n.requestsHandled++
	n.mu.Unlock()

	return data, nil
}

func main() {
	nodeID := flag.String("id", "", "Node ID (port number)")
	controllerAddr := flag.String("controller", "localhost:8000", "Controller address")
	dataDir := flag.String("data", "", "Data directory path")
	flag.Parse()

	if *nodeID == "" || *dataDir == "" {
		log.Fatal("Node ID and data directory are required")
	}

	node := NewStorageNode(*nodeID, *controllerAddr, *dataDir)
	if err := node.Start(); err != nil {
		log.Fatalf("Storage node failed to start: %v", err)
	}
}