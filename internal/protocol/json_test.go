package protocol

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

type mockConnection struct {
	stdin       *bytes.Buffer
	stdout      *bytes.Buffer
	stderr      *bytes.Buffer
	stdinCloser *mockWriteCloser
}

type mockWriteCloser struct {
	*bytes.Buffer
}

func (m *mockWriteCloser) Close() error {
	return nil
}

func newMockConnection() *mockConnection {
	stdin := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	stdinCloser := &mockWriteCloser{stdin}
	return &mockConnection{
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		stdinCloser: stdinCloser,
	}
}

func (m *mockConnection) Stdin() io.WriteCloser {
	return m.stdinCloser
}

func (m *mockConnection) Stdout() io.Reader {
	return m.stdout
}

func (m *mockConnection) Stderr() io.Reader {
	return m.stderr
}

func (m *mockConnection) SendPayload(payload interface{}) error {
	return nil
}

func (m *mockConnection) Close() error {
	return nil
}

func TestHandleJSONModeContextCancellation(t *testing.T) {
	mockConn := newMockConnection()

	ctx, cancel := context.WithCancel(context.Background())

	in := bytes.NewBufferString(`{"hello":"world"}`)
	out := &bytes.Buffer{}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := HandleJSONMode(ctx, mockConn, in, out)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestRunMainLoopCompilation(t *testing.T) {
	mockConn := newMockConnection()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reconnectCh := make(chan struct{})

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RunMainLoop(ctx, mockConn, reconnectCh)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}
