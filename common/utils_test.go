package common

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: bytes.NewBuffer(nil),
	}
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return m.readBuf.Read(b) }
func (m *mockConn) Write(b []byte) (n int, err error)  { return m.writeBuf.Write(b) }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr               { return nil }
func (m *mockConn) RemoteAddr() net.Addr              { return nil }
func (m *mockConn) SetDeadline(t time.Time) error     { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestWriteAndReadMessage(t *testing.T) {
	tests := []struct {
		name    string
		msgType byte
		data    []byte
		wantErr bool
	}{
		{
			name:    "basic message",
			msgType: 1,
			data:    []byte("test message"),
			wantErr: false,
		},
		{
			name:    "empty message",
			msgType: 2,
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "large message",
			msgType: 3,
			data:    bytes.Repeat([]byte("large message "), 1000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newMockConn()

			// Write message
			if err := WriteMessage(conn, tt.msgType, tt.data); err != nil {
				if !tt.wantErr {
					t.Errorf("WriteMessage() error = %v", err)
				}
				return
			}

			// Copy written data to read buffer
			conn.readBuf.Write(conn.writeBuf.Bytes())

			// Read message
			gotType, gotData, err := ReadMessage(conn)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("ReadMessage() error = %v", err)
				}
				return
			}

			if gotType != tt.msgType {
				t.Errorf("ReadMessage() type = %v, want %v", gotType, tt.msgType)
			}
			if !bytes.Equal(gotData, tt.data) {
				t.Errorf("ReadMessage() data = %v, want %v", gotData, tt.data)
			}
		})
	}
}

func TestCalculateAndVerifyChecksum(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "simple string",
			data: []byte("test data"),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0xFF, 0x42, 0x13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum := CalculateChecksum(tt.data)

			if !VerifyChecksum(tt.data, checksum) {
				t.Error("VerifyChecksum() failed for valid data")
			}

			// Modify data to test failure
			if len(tt.data) > 0 {
				modifiedData := make([]byte, len(tt.data))
				copy(modifiedData, tt.data)
				modifiedData[0] ^= 0xFF // Flip bits in first byte

				if VerifyChecksum(modifiedData, checksum) {
					t.Error("VerifyChecksum() succeeded for modified data")
				}
			}
		})
	}
}

func TestSplitAndJoinFile(t *testing.T) {
	// Create test data
	testData := bytes.Repeat([]byte("test data block "), 1000)
	chunkSize := int64(1024) // 1KB chunks

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_file_*.dat")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Seek(0, 0)

	// Split file
	chunks, err := SplitFile(tmpFile, chunkSize)
	if err != nil {
		t.Fatalf("SplitFile() error = %v", err)
	}

	// Verify number of chunks
	expectedChunks := (int64(len(testData)) + chunkSize - 1) / chunkSize
	if int64(len(chunks)) != expectedChunks {
		t.Errorf("Got %d chunks, want %d", len(chunks), expectedChunks)
	}

	// Create output file
	outFile, err := os.CreateTemp("", "test_output_*.dat")
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	defer os.Remove(outFile.Name())

	// Join chunks
	if err := JoinChunks(chunks, outFile); err != nil {
		t.Fatalf("JoinChunks() error = %v", err)
	}

	// Verify output
	outFile.Seek(0, 0)
	outputData, err := io.ReadAll(outFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !bytes.Equal(outputData, testData) {
		t.Error("Output data does not match input data")
	}
}

func TestGetAvailableDiskSpace(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "disk_space_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with directory
	space, err := GetAvailableDiskSpace(tmpDir)
	if err != nil {
		t.Errorf("GetAvailableDiskSpace() error = %v", err)
	}
	if space == 0 {
		t.Error("GetAvailableDiskSpace() returned 0 space")
	}

	// Test with non-existent path
	_, err = GetAvailableDiskSpace(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("GetAvailableDiskSpace() succeeded with non-existent path")
	}

	// Test with file instead of directory
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	_, err = GetAvailableDiskSpace(tmpFile)
	if err == nil {
		t.Error("GetAvailableDiskSpace() succeeded with file path")
	}
}