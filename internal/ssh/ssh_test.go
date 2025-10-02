package ssh

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/hibeam-dev/amaru_agent/internal/config"
	"github.com/hibeam-dev/amaru_agent/internal/transport"
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
	t.Run("SuccessfulSend", func(t *testing.T) {
		mockStdin := &MockWriteCloser{Buffer: bytes.NewBuffer(nil)}

		conn := &Conn{
			stdin: mockStdin,
		}

		testPayload := map[string]string{"type": "test", "value": "data"}
		err := conn.SendPayload(testPayload)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		writtenData := mockStdin.String()

		expected := `{"type":"test","value":"data"}` + "\n"
		if writtenData != expected {
			t.Errorf("Expected payload: %s, got: %s", expected, writtenData)
		}
	})

	t.Run("MarshalError", func(t *testing.T) {
		conn := &Conn{
			stdin: &MockWriteCloser{Buffer: bytes.NewBuffer(nil)},
		}

		badPayload := map[string]any{"func": func() {}}
		err := conn.SendPayload(badPayload)

		if err == nil {
			t.Errorf("Expected marshal error, got nil")
		}
	})

	t.Run("ConnectionClosed", func(t *testing.T) {
		conn := &Conn{
			stdin: nil, // Simulate closed connection
		}

		err := conn.SendPayload(map[string]string{"type": "test"})

		if err == nil {
			t.Errorf("Expected closed connection error, got nil")
		}
	})

	t.Run("WriteError", func(t *testing.T) {
		expectedErr := io.ErrClosedPipe
		mockStdin := &MockWriteCloser{
			Buffer:   bytes.NewBuffer(nil),
			writeErr: expectedErr,
		}

		conn := &Conn{
			stdin: mockStdin,
		}

		err := conn.SendPayload(map[string]string{"type": "test"})

		if err != expectedErr {
			t.Errorf("Expected write error %v, got: %v", expectedErr, err)
		}
	})
}

type MockWriteCloser struct {
	*bytes.Buffer
	closeErr error
	writeErr error
}

func (m *MockWriteCloser) Close() error {
	return m.closeErr
}

func (m *MockWriteCloser) Write(p []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}

	return m.Buffer.Write(p)
}
