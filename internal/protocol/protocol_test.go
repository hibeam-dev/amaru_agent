package protocol

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"erlang-solutions.com/cortex_agent/internal/transport/mocks"
	"erlang-solutions.com/cortex_agent/internal/util"
)

type testReader struct {
	mu          sync.Mutex
	data        bytes.Buffer
	readError   error
	readTimeout bool
}

func (t *testReader) Read(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.readError != nil {
		return 0, t.readError
	}

	if t.readTimeout {
		return 0, errors.New("i/o timeout")
	}

	return t.data.Read(p)
}

func (t *testReader) SetReadDeadline(deadline time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.readTimeout = time.Now().After(deadline)
	return nil
}

func (t *testReader) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.data.Write(p)
}

func createMockConn(ctrl *gomock.Controller) (*mocks.MockConnection, *testReader, *testReader) {
	mockConn := mocks.NewMockConnection(ctrl)

	stdoutReader := &testReader{}
	stderrReader := &testReader{}

	mockConn.EXPECT().Stdout().Return(stdoutReader).AnyTimes()
	mockConn.EXPECT().Stderr().Return(stderrReader).AnyTimes()

	return mockConn, stdoutReader, stderrReader
}

func TestRunHeartbeat(t *testing.T) {
	t.Run("SendsHeartbeats", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mocks.NewMockConnection(ctrl)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		mockConn.EXPECT().SendPayload(map[string]string{"type": "heartbeat"}).Return(nil).AnyTimes()

		err := RunHeartbeat(ctx, mockConn, 100*time.Millisecond)

		if err != context.DeadlineExceeded {
			t.Errorf("Expected context deadline exceeded, got: %v", err)
		}
	})

	t.Run("HandlesConnectionError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mocks.NewMockConnection(ctrl)
		ctx := t.Context()

		expectedErr := errors.New("connection failed")
		mockConn.EXPECT().SendPayload(map[string]string{"type": "heartbeat"}).Return(expectedErr)

		err := RunHeartbeat(ctx, mockConn, 10*time.Millisecond)

		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		var appErr *util.AppError
		if !errors.As(err, &appErr) {
			t.Errorf("Expected app error, got: %v", err)
		}

		if appErr.Type != util.ErrTypeConnection {
			t.Errorf("Expected connection error type, got: %v", appErr.Type)
		}
	})
}

func TestRunMainLoop(t *testing.T) {
	t.Run("HandlesContextCancellation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn, _, _ := createMockConn(ctrl)

		mockConn.EXPECT().SendPayload(gomock.Any()).Return(nil).AnyTimes()

		parentCtx, parentCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer parentCancel()

		ctx, cancel := context.WithCancel(parentCtx)
		reconnectCh := make(chan struct{})

		done := make(chan error, 1)
		go func() {
			done <- RunMainLoop(ctx, mockConn, reconnectCh)
		}()

		cancel()

		select {
		case err := <-done:
			if err != context.Canceled {
				t.Errorf("Expected context canceled error, got: %v", err)
			}
		case <-parentCtx.Done():
			t.Fatal("Test timed out waiting for RunMainLoop to complete")
		}
	})

	t.Run("HandlesReconnectSignal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn, _, _ := createMockConn(ctrl)

		mockConn.EXPECT().SendPayload(gomock.Any()).Return(nil).AnyTimes()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		reconnectCh := make(chan struct{}, 1)

		done := make(chan error, 1)
		go func() {
			done <- RunMainLoop(ctx, mockConn, reconnectCh)
		}()

		reconnectCh <- struct{}{}

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Expected nil error on reconnect, got: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("Test timed out waiting for RunMainLoop to complete")
		}
	})

	t.Run("HandlesDataReadError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn, stdoutReader, _ := createMockConn(ctrl)

		mockConn.EXPECT().SendPayload(gomock.Any()).Return(nil).AnyTimes()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		reconnectCh := make(chan struct{})

		expectedErr := errors.New("read failed")
		stdoutReader.readError = expectedErr

		errCh := make(chan error, 1)

		go func() {
			errCh <- RunMainLoop(ctx, mockConn, reconnectCh)
		}()

		var err error

		select {
		case err = <-errCh:
		case <-time.After(1 * time.Second):
			t.Fatal("Test timed out waiting for RunMainLoop to return")
		}

		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		var appErr *util.AppError
		if !errors.As(err, &appErr) {
			t.Errorf("Expected app error, got: %v", err)
		}

		if appErr.Type != util.ErrTypeConnection {
			t.Errorf("Expected connection error type, got: %v", appErr.Type)
		}
	})
}
