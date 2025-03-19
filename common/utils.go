package common

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

const (
	// Message types
	MsgTypeHeartbeat        = 1
	MsgTypeStorageRequest   = 2
	MsgTypeStorageResponse  = 3
	MsgTypeRetrievalRequest = 4
	MsgTypeRetrievalResponse = 5
	MsgTypeDeleteRequest    = 6
	MsgTypeDeleteResponse   = 7
	MsgTypeListRequest      = 8
	MsgTypeListResponse     = 9
	MsgTypeNodeStatusRequest = 10
	MsgTypeNodeStatusResponse = 11
	MsgTypeChunkStore       = 12
	MsgTypeChunkRetrieve    = 13

	// Default values
	DefaultChunkSize    = 64 * 1024 * 1024 // 64MB
	DefaultReplication  = 3
	HeartbeatInterval  = 5  // seconds
	HeartbeatTimeout   = 15 // seconds
)

// WriteMessage writes a protobuf message to a connection with a header
func WriteMessage(conn net.Conn, msgType byte, data []byte) error {
	// Header: [Type (1 byte)][Length (4 bytes)][Data (length bytes)]
	header := make([]byte, 5)
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))

	// Write header
	if _, err := conn.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Write data
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %v", err)
	}

	return nil
}

// ReadMessage reads a protobuf message from a connection
func ReadMessage(conn net.Conn) (byte, []byte, error) {
	// Read header
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, fmt.Errorf("failed to read header: %v", err)
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])

	// Read data
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return 0, nil, fmt.Errorf("failed to read data: %v", err)
	}

	return msgType, data, nil
}

// CalculateChecksum calculates SHA-256 checksum of data
func CalculateChecksum(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// VerifyChecksum verifies if data matches the given checksum
func VerifyChecksum(data, checksum []byte) bool {
	return string(CalculateChecksum(data)) == string(checksum)
}

// GetAvailableDiskSpace returns available disk space in bytes for a given path
func GetAvailableDiskSpace(path string) (uint64, error) {
	var stat os.FileInfo
	var err error
	
	if stat, err = os.Stat(path); err != nil {
		return 0, fmt.Errorf("failed to get path stats: %v", err)
	}

	if !stat.IsDir() {
		return 0, fmt.Errorf("path is not a directory")
	}

	// Create a temporary file to check available space
	tmpFile := fmt.Sprintf("%s/space_check_%d", path, os.Getpid())
	f, err := os.Create(tmpFile)
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmpFile)
	}()

	// Try to write increasing amounts until it fails
	var space uint64 = 1024 * 1024 // Start with 1MB
	buffer := make([]byte, 1024)
	for {
		_, err := f.Write(buffer)
		if err != nil {
			break
		}
		space += 1024
	}

	return space, nil
}

// SplitFile splits a file into chunks of specified size
func SplitFile(file *os.File, chunkSize int64) ([][]byte, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	fileSize := fileInfo.Size()
	numChunks := (fileSize + chunkSize - 1) / chunkSize
	chunks := make([][]byte, 0, numChunks)

	for i := int64(0); i < numChunks; i++ {
		chunk := make([]byte, chunkSize)
		n, err := file.ReadAt(chunk, i*chunkSize)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read chunk %d: %v", i, err)
		}
		if n < len(chunk) {
			chunk = chunk[:n]
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// JoinChunks combines file chunks in order
func JoinChunks(chunks [][]byte, output *os.File) error {
	for i, chunk := range chunks {
		if _, err := output.Write(chunk); err != nil {
			return fmt.Errorf("failed to write chunk %d: %v", i, err)
		}
	}
	return nil
}