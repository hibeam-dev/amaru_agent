package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

type MockConnection struct {
	stdin          *bytes.Buffer
	stdout         *bytes.Buffer
	stderr         *bytes.Buffer
	stdinCloser    *MockWriteCloser
	sendPayloadErr error
	closeErr       error
	mu             sync.Mutex
	closed         bool
}

var _ Connection = (*MockConnection)(nil)

type MockWriteCloser struct {
	*bytes.Buffer
	closeErr error
}

func (m *MockWriteCloser) Close() error {
	return m.closeErr
}

func NewMockConnection() *MockConnection {
	stdin := bytes.NewBuffer(nil)
	stdinCloser := &MockWriteCloser{Buffer: stdin}
	return &MockConnection{
		stdin:       stdin,
		stdinCloser: stdinCloser,
		stdout:      bytes.NewBuffer(nil),
		stderr:      bytes.NewBuffer(nil),
	}
}

func (m *MockConnection) Stdin() io.WriteCloser {
	return m.stdinCloser
}

func (m *MockConnection) Stdout() io.Reader {
	return m.stdout
}

func (m *MockConnection) Stderr() io.Reader {
	return m.stderr
}

func (m *MockConnection) SendPayload(payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("connection closed")
	}

	if m.sendPayloadErr != nil {
		return m.sendPayloadErr
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize payload to JSON: %w", err)
	}

	_, err = m.stdin.Write(jsonData)
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	return nil
}

func (m *MockConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return m.closeErr
}

func (m *MockConnection) WriteToStdout(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stdout.Write(data)
}

func (m *MockConnection) WriteToStderr(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stderr.Write(data)
}

func (m *MockConnection) GetWrittenData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stdin.Bytes()
}

func (m *MockConnection) SetSendPayloadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendPayloadErr = err
}

func (m *MockConnection) SetCloseError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeErr = err
}
