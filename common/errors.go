package common

import "fmt"

// ChunkCorruptionError indicates that a chunk's data is corrupted
type ChunkCorruptionError struct {
	Filename  string
	ChunkNum int
}

func (e ChunkCorruptionError) Error() string {
	return fmt.Sprintf("chunk %s_%d is corrupted", e.Filename, e.ChunkNum)
}

// NodeNotFoundError indicates that a storage node could not be found
type NodeNotFoundError struct {
	NodeID string
}

func (e NodeNotFoundError) Error() string {
	return fmt.Sprintf("node %s not found", e.NodeID)
}

// FileExistsError indicates that a file already exists in the system
type FileExistsError struct {
	Filename string
}

func (e FileExistsError) Error() string {
	return fmt.Sprintf("file %s already exists", e.Filename)
}

// FileNotFoundError indicates that a file does not exist in the system
type FileNotFoundError struct {
	Filename string
}

func (e FileNotFoundError) Error() string {
	return fmt.Sprintf("file %s not found", e.Filename)
}

// NotEnoughNodesError indicates that there are not enough storage nodes available
type NotEnoughNodesError struct {
	Required int
	Available int
}

func (e NotEnoughNodesError) Error() string {
	return fmt.Sprintf("not enough storage nodes available (required: %d, available: %d)", e.Required, e.Available)
}

// ChunkNotFoundError indicates that a chunk could not be found
type ChunkNotFoundError struct {
	Filename  string
	ChunkNum int
}

func (e ChunkNotFoundError) Error() string {
	return fmt.Sprintf("chunk %s_%d not found", e.Filename, e.ChunkNum)
}

// StorageFullError indicates that a storage node is out of space
type StorageFullError struct {
	NodeID string
	Required uint64
	Available uint64
}

func (e StorageFullError) Error() string {
	return fmt.Sprintf("storage node %s is full (required: %d bytes, available: %d bytes)", e.NodeID, e.Required, e.Available)
}

// ConnectionError indicates a network connection error
type ConnectionError struct {
	Address string
	Err     error
}

func (e ConnectionError) Error() string {
	return fmt.Sprintf("connection error to %s: %v", e.Address, e.Err)
}

// ProtocolError indicates a protocol-level error
type ProtocolError struct {
	Message string
	Err     error
}

func (e ProtocolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("protocol error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("protocol error: %s", e.Message)
}

// TimeoutError indicates an operation timed out
type TimeoutError struct {
	Operation string
	Duration  string
}

func (e TimeoutError) Error() string {
	return fmt.Sprintf("operation %s timed out after %s", e.Operation, e.Duration)
}

// ValidationError indicates invalid input or parameters
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}