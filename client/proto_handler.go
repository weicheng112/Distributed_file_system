package main

import (
	"fmt"
	"net"
	"os"
	"sync"

	"google.golang.org/protobuf/proto"
)

// getStorageLocations requests chunk storage locations from the controller
func (c *Client) getStorageLocations(filename string, fileSize int64, chunkSize int64) (map[int][]string, error) {
	// Connect to controller
	conn, err := net.Dial("tcp", c.controllerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %v", err)
	}
	defer conn.Close()

	// Create request
	request := &dfs.StorageRequest{
		Filename:  filename,
		FileSize:  uint64(fileSize),
		ChunkSize: uint32(chunkSize),
	}

	// Serialize request
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if err := common.WriteMessage(conn, common.MsgTypeStorageRequest, requestData); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	msgType, responseData, err := common.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if msgType != common.MsgTypeStorageResponse {
		return nil, fmt.Errorf("unexpected response type: %d", msgType)
	}

	response := &dfs.StorageResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("controller error: %s", response.Error)
	}

	// Convert response to map
	locations := make(map[int][]string)
	for _, placement := range response.ChunkPlacements {
		locations[int(placement.ChunkNumber)] = placement.StorageNodes
	}

	return locations, nil
}

// storeChunk stores a chunk on a storage node
func (c *Client) storeChunk(file *os.File, chunkNum int, chunkSize int64, nodes []string) error {
	// Read chunk data
	data := make([]byte, chunkSize)
	n, err := file.ReadAt(data, int64(chunkNum)*chunkSize)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read chunk: %v", err)
	}
	data = data[:n] // Trim to actual size

	// Connect to primary storage node
	conn, err := net.Dial("tcp", nodes[0])
	if err != nil {
		return fmt.Errorf("failed to connect to storage node: %v", err)
	}
	defer conn.Close()

	// Create request
	request := &dfs.ChunkStoreRequest{
		Filename:    filepath.Base(file.Name()),
		ChunkNumber: uint32(chunkNum),
		Data:       data,
		ReplicaNodes: nodes[1:], // Remaining nodes for replication
	}

	// Serialize request
	requestData, err := proto.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if err := common.WriteMessage(conn, common.MsgTypeChunkStore, requestData); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	msgType, responseData, err := common.ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	if msgType != common.MsgTypeChunkStore {
		return fmt.Errorf("unexpected response type: %d", msgType)
	}

	response := &dfs.ChunkStoreResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("storage node error: %s", response.Error)
	}

	return nil
}

// getChunkLocations requests chunk locations from the controller
func (c *Client) getChunkLocations(filename string) (map[int][]string, error) {
	// Connect to controller
	conn, err := net.Dial("tcp", c.controllerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %v", err)
	}
	defer conn.Close()

	// Create request
	request := &dfs.RetrievalRequest{
		Filename: filename,
	}

	// Serialize request
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if err := common.WriteMessage(conn, common.MsgTypeRetrievalRequest, requestData); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	msgType, responseData, err := common.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if msgType != common.MsgTypeRetrievalResponse {
		return nil, fmt.Errorf("unexpected response type: %d", msgType)
	}

	response := &dfs.RetrievalResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("controller error: %s", response.Error)
	}

	// Convert response to map
	locations := make(map[int][]string)
	for _, chunk := range response.Chunks {
		locations[int(chunk.ChunkNumber)] = chunk.StorageNodes
	}

	return locations, nil
}

// retrieveChunk retrieves a chunk from a storage node
func (c *Client) retrieveChunk(filename string, chunkNum int, nodes []string) ([]byte, error) {
	// Try each node until successful
	var lastErr error
	for _, node := range nodes {
		data, err := c.retrieveChunkFromNode(filename, chunkNum, node)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("failed to retrieve chunk from all nodes: %v", lastErr)
}

func (c *Client) retrieveChunkFromNode(filename string, chunkNum int, node string) ([]byte, error) {
	// Connect to storage node
	conn, err := net.Dial("tcp", node)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to storage node: %v", err)
	}
	defer conn.Close()

	// Create request
	request := &dfs.ChunkRetrieveRequest{
		Filename:    filename,
		ChunkNumber: uint32(chunkNum),
	}

	// Serialize request
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if err := common.WriteMessage(conn, common.MsgTypeChunkRetrieve, requestData); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	msgType, responseData, err := common.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if msgType != common.MsgTypeChunkRetrieve {
		return nil, fmt.Errorf("unexpected response type: %d", msgType)
	}

	response := &dfs.ChunkRetrieveResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("storage node error: %s", response.Error)
	}

	return response.Data, nil
}

// listFiles requests the list of files from the controller
func (c *Client) listFiles() ([]*dfs.FileInfo, error) {
	// Connect to controller
	conn, err := net.Dial("tcp", c.controllerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %v", err)
	}
	defer conn.Close()

	// Create empty request
	request := &dfs.ListFilesRequest{}

	// Serialize request
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if err := common.WriteMessage(conn, common.MsgTypeListRequest, requestData); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	msgType, responseData, err := common.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if msgType != common.MsgTypeListResponse {
		return nil, fmt.Errorf("unexpected response type: %d", msgType)
	}

	response := &dfs.ListFilesResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return response.Files, nil
}

// getNodeStatus requests the status of all nodes from the controller
func (c *Client) getNodeStatus() (*dfs.NodeStatusResponse, error) {
	// Connect to controller
	conn, err := net.Dial("tcp", c.controllerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %v", err)
	}
	defer conn.Close()

	// Create empty request
	request := &dfs.NodeStatusRequest{}

	// Serialize request
	requestData, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Send request
	if err := common.WriteMessage(conn, common.MsgTypeNodeStatusRequest, requestData); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	// Read response
	msgType, responseData, err := common.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if msgType != common.MsgTypeNodeStatusResponse {
		return nil, fmt.Errorf("unexpected response type: %d", msgType)
	}

	response := &dfs.NodeStatusResponse{}
	if err := proto.Unmarshal(responseData, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return response, nil
}