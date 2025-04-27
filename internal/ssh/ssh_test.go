package ssh

import (
	"bytes"
	"context"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/transport"
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
	stdin := &MockWriteCloser{Buffer: bytes.NewBuffer(nil)}
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	conn := &Conn{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	if conn.Stdin() != stdin {
		t.Error("Stdin() did not return the expected writer")
	}

	if conn.Stdout() != stdout {
		t.Error("Stdout() did not return the expected reader")
	}

	if conn.Stderr() != stderr {
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

	_, err := transport.Connect(ctx, cfg,
		transport.WithKeyFile("/does/not/exist/key.file"),
	)
	if err == nil {
		t.Fatal("Expected error for nonexistent keyfile, got nil")
	}
}

func TestSendPayload(t *testing.T) {
	t.Skip("Need to fix the implementation to make it more testable")
}

type MockWriteCloser struct {
	*bytes.Buffer
	closeErr error
}

func (m *MockWriteCloser) Close() error {
	return m.closeErr
}
