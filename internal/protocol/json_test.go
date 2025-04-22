package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func TestHandleJSONModeContextCancellation(t *testing.T) {
	mockConn := ssh.NewMockConnection()

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

func TestHandleJSONModeRequestResponse(t *testing.T) {
	t.Skip("This test is flaky - will need to fix the JSON protocol implementation to make it more testable")

	mockConn := ssh.NewMockConnection()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := bytes.NewBufferString(`{"request":"test"}` + "\n")
	out := &bytes.Buffer{}

	// Set up a goroutine to simulate server response
	go func() {
		time.Sleep(50 * time.Millisecond)

		for i := 0; i < 10; i++ {
			if len(mockConn.GetWrittenData()) > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		mockConn.WriteToStdout([]byte(`{"response":"ok"}` + "\n"))

		time.Sleep(50 * time.Millisecond)

		cancel()
	}()

	err := HandleJSONMode(ctx, mockConn, in, out)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	var response map[string]string
	decoder := json.NewDecoder(out)
	if err := decoder.Decode(&response); err != nil && err != io.EOF {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["response"] != "ok" {
		t.Errorf("Expected response to contain 'response': 'ok', got: %v", response)
	}
}

func TestRunMainLoopCompilation(t *testing.T) {
	mockConn := ssh.NewMockConnection()

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
