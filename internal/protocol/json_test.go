package protocol

import (
	"bytes"
	"context"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/transport/mocks"
	"go.uber.org/mock/gomock"
)

// Custom WriteCloser for testing
type mockWriteCloser struct {
	*bytes.Buffer
}

func (m *mockWriteCloser) Close() error {
	return nil
}

// Helper function to create a MockConnection with stdin/stdout/stderr buffers
func newMockConnection(t *testing.T) *mocks.MockConnection {
	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	stdin := bytes.NewBuffer(nil)
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	stdinCloser := &mockWriteCloser{stdin}

	mockConn := mocks.NewMockConnection(ctrl)
	mockConn.EXPECT().Stdin().Return(stdinCloser).AnyTimes()
	mockConn.EXPECT().Stdout().Return(stdout).AnyTimes()
	mockConn.EXPECT().Stderr().Return(stderr).AnyTimes()

	return mockConn
}

func TestHandleJSONModeContextCancellation(t *testing.T) {
	mockConn := newMockConnection(t)

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
	mockConn := newMockConnection(t)

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
