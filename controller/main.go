package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"distributed_file_system/common"
)

// FileMetadata stores information about a file in the system
type FileMetadata struct {
	Size      int64
	ChunkSize int
	Chunks    map[int][]string // Map of chunk number to list of storage nodes
}

// NodeInfo stores information about a storage node
type NodeInfo struct {
	ID               string
	Address          string
	FreeSpace        uint64
	RequestsHandled  uint64
	LastHeartbeat    time.Time
	ReplicatedChunks map[string][]int // Map of filename to chunk numbers
}

// Controller manages the distributed file system metadata and coordinates storage nodes
type Controller struct {
	mu sync.RWMutex

	// Map of active storage nodes
	// Key: node ID (ip:port), Value: node information
	nodes map[string]*NodeInfo

	// Map of files to their metadata
	files map[string]*FileMetadata

	// Configuration
	replicationFactor int
	heartbeatTimeout  time.Duration

	// Listener for incoming connections
	listener net.Listener

	// Port number
	port int
}

func NewController(listenPort int) *Controller {
	return &Controller{
		nodes:             make(map[string]*NodeInfo),
		files:             make(map[string]*FileMetadata),
		replicationFactor: common.DefaultReplication,
		heartbeatTimeout:  common.HeartbeatTimeout * time.Second,
		port:             listenPort,
	}
}

func (c *Controller) Start() error {
	// Start listening for connections
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", c.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}
	c.listener = listener

	// Start background tasks
	go c.checkNodeHealth()
	go c.maintainReplication()

	log.Printf("Controller started on port %d", c.port)

	// Accept and handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go c.handleConnection(conn)
	}
}

func (c *Controller) handleConnection(conn net.Conn) {
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
		case common.MsgTypeHeartbeat:
			respErr = c.handleHeartbeat(data)
		case common.MsgTypeStorageRequest:
			response, respErr = c.handleStorageRequest(data)
		case common.MsgTypeRetrievalRequest:
			response, respErr = c.handleRetrievalRequest(data)
		case common.MsgTypeDeleteRequest:
			response, respErr = c.handleDeleteRequest(data)
		case common.MsgTypeListRequest:
			response, respErr = c.handleListRequest(data)
		case common.MsgTypeNodeStatusRequest:
			response, respErr = c.handleNodeStatusRequest(data)
		default:
			respErr = fmt.Errorf("unknown message type: %d", msgType)
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

func (c *Controller) checkNodeHealth() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for nodeID, info := range c.nodes {
			if now.Sub(info.LastHeartbeat) > c.heartbeatTimeout {
				log.Printf("Node %s appears to be down, removing from active nodes", nodeID)
				delete(c.nodes, nodeID)
				go c.handleNodeFailure(nodeID)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Controller) handleNodeFailure(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find all chunks that were stored on the failed node
	affectedChunks := make(map[string][]int) // filename -> chunk numbers
	for filename, metadata := range c.files {
		for chunkNum, nodes := range metadata.Chunks {
			for _, node := range nodes {
				if node == nodeID {
					affectedChunks[filename] = append(affectedChunks[filename], chunkNum)
					break
				}
			}
		}
	}

	// Trigger re-replication for affected chunks
	for filename, chunks := range affectedChunks {
		for _, chunkNum := range chunks {
			go c.replicateChunk(filename, chunkNum)
		}
	}
}

func (c *Controller) maintainReplication() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		c.mu.RLock()
		// Check replication level of all chunks
		for filename, metadata := range c.files {
			for chunkNum, nodes := range metadata.Chunks {
				if len(nodes) < c.replicationFactor {
					go c.replicateChunk(filename, chunkNum)
				}
			}
		}
		c.mu.RUnlock()
	}
}

func (c *Controller) replicateChunk(filename string, chunkNum int) {
	// Implementation will be added for chunk replication
	// This will coordinate with storage nodes to create new replicas
}

func main() {
	listenPort := flag.Int("port", 8000, "Port to listen on")
	flag.Parse()

	controller := NewController(*listenPort)
	if err := controller.Start(); err != nil {
		log.Fatalf("Controller failed to start: %v", err)
	}
}