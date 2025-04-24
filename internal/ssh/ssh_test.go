package ssh

import (
	"context"
	"io"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/pkg/errors"
)

func TestConnClose(t *testing.T) {
	conn := &Conn{}

	err := conn.Close()
	if err != nil {
		t.Errorf("First close returned error: %v", err)
	}

	err = conn.Close()
	if err != nil {
		t.Errorf("Second close returned error: %v", err)
	}
}

func TestConnAccessors(t *testing.T) {
	mockConn := NewMockConnection()

	conn := &Conn{
		stdin:  mockConn.Stdin(),
		stdout: mockConn.Stdout(),
		stderr: mockConn.Stderr(),
	}

	if conn.Stdin() != mockConn.Stdin() {
		t.Error("Stdin() did not return the expected writer")
	}

	if conn.Stdout() != mockConn.Stdout() {
		t.Error("Stdout() did not return the expected reader")
	}

	if conn.Stderr() != mockConn.Stderr() {
		t.Error("Stderr() did not return the expected reader")
	}
}

func TestConnectFailures(t *testing.T) {
	ctx := context.Background()

	cfg := config.Config{}
	cfg.Connection.Host = "localhost"
	cfg.Connection.Port = 22
	cfg.Connection.User = "testuser"
	cfg.Connection.KeyFile = "/does/not/exist/key.file"
	cfg.Connection.Timeout = 1 * time.Second

	_, err := Connect(ctx, cfg)
	if err == nil {
		t.Fatal("Expected error for nonexistent keyfile, got nil")
	}
}

func TestSendPayload(t *testing.T) {
	t.Skip("Need to fix the implementation to make it more testable")
}

func TestSendPayloadError(t *testing.T) {
	mockConn := NewMockConnection()

	mockConn.SetSendPayloadError(errors.ErrSubsystemFailed)

	payload := ConfigPayload{
		Application: ApplicationConfig{
			Hostname: "test-host",
		},
	}

	// Send the payload, expecting error
	err := mockConn.SendPayload(payload)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err != errors.ErrSubsystemFailed {
		t.Errorf("Expected ErrSubsystemFailed, got: %v", err)
	}
}

func TestMockConnection(t *testing.T) {
	mock := NewMockConnection()

	data := []byte("test data")
	n, err := mock.Stdin().Write(data)
	if err != nil {
		t.Errorf("Unexpected error writing to stdin: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	if string(mock.GetWrittenData()) != "test data" {
		t.Errorf("Expected GetWrittenData to return 'test data', got '%s'", mock.GetWrittenData())
	}

	mock.WriteToStdout([]byte("stdout data"))
	buffer := make([]byte, 100)
	n, err = mock.Stdout().Read(buffer)
	if err != nil {
		t.Errorf("Unexpected error reading from stdout: %v", err)
	}
	if string(buffer[:n]) != "stdout data" {
		t.Errorf("Expected to read 'stdout data', got '%s'", string(buffer[:n]))
	}

	mock.WriteToStderr([]byte("stderr data"))
	n, err = mock.Stderr().Read(buffer)
	if err != nil {
		t.Errorf("Unexpected error reading from stderr: %v", err)
	}
	if string(buffer[:n]) != "stderr data" {
		t.Errorf("Expected to read 'stderr data', got '%s'", string(buffer[:n]))
	}

	if err = mock.Close(); err != nil {
		t.Errorf("Unexpected error closing mock: %v", err)
	}

	mock.SetCloseError(io.ErrClosedPipe)
	if err = mock.Close(); err != io.ErrClosedPipe {
		t.Errorf("Expected ErrClosedPipe, got: %v", err)
	}
}
