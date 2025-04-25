package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
)

type mockConnection struct {
	stdin          *bytes.Buffer
	stdout         *bytes.Buffer
	stderr         *bytes.Buffer
	stdinCloser    *mockWriteCloser
	sendPayloadErr error
	closeErr       error
	mu             sync.Mutex
}

type mockWriteCloser struct {
	*bytes.Buffer
	closeErr error
}

func (m *mockWriteCloser) Close() error {
	return m.closeErr
}

func newMockConnection() *mockConnection {
	stdin := bytes.NewBuffer(nil)
	stdinCloser := &mockWriteCloser{Buffer: stdin}
	return &mockConnection{
		stdin:       stdin,
		stdinCloser: stdinCloser,
		stdout:      bytes.NewBuffer(nil),
		stderr:      bytes.NewBuffer(nil),
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

func (m *mockConnection) Close() error {
	return m.closeErr
}

func (m *mockConnection) getWrittenData() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stdin.Bytes()
}

func (m *mockConnection) setSendPayloadError(err error) {
	m.sendPayloadErr = err
}

func TestConnectionService(t *testing.T) {
	t.Run("JSONMode", func(t *testing.T) {
		bus := event.NewBus()
		svc1 := NewConnectionService(true, bus)
		svc2 := NewConnectionService(false, bus)

		if !svc1.IsJSONMode() {
			t.Error("Should return true when initialized with JSON mode")
		}

		if svc2.IsJSONMode() {
			t.Error("Should return false when initialized without JSON mode")
		}
	})

	t.Run("ConfigManagement", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConnectionService(false, bus)

		testConfig := config.Config{}
		testConfig.Application.Hostname = "test-host"
		testConfig.Application.Port = 8080
		testConfig.Agent.Tags = map[string]string{"env": "test"}

		svc.SetConfig(testConfig)
		retrievedCfg := svc.GetConfig()

		expected := []struct {
			name     string
			actual   interface{}
			expected interface{}
		}{
			{"hostname", retrievedCfg.Application.Hostname, "test-host"},
			{"port", retrievedCfg.Application.Port, 8080},
			{"tags.env", retrievedCfg.Agent.Tags["env"], "test"},
		}

		for _, e := range expected {
			if e.actual != e.expected {
				t.Errorf("Expected %s to be %v, got %v", e.name, e.expected, e.actual)
			}
		}
	})

	t.Run("HasConnection", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConnectionService(false, bus)

		if svc.HasConnection() {
			t.Error("New service should not have an active connection")
		}
	})

	t.Run("SendConfig", func(t *testing.T) {
		mockConn := newMockConnection()
		bus := event.NewBus()
		svc := NewConnectionService(false, bus)

		cfg := config.Config{}
		cfg.Application.Hostname = "test-host"
		cfg.Application.Port = 8080
		cfg.Agent.Tags = map[string]string{"env": "test"}

		if err := svc.sendConfig(mockConn, cfg); err != nil {
			t.Errorf("Expected no error on send, got: %v", err)
		}

		writtenData := mockConn.getWrittenData()
		if len(writtenData) == 0 {
			t.Error("No data was written to connection")
		}

		if !strings.Contains(string(writtenData), `"type":"config_update"`) {
			t.Errorf("Expected config_update in payload, got: %s", string(writtenData))
		}

		testErr := errors.New("test error")
		mockConn.setSendPayloadError(testErr)

		if err := svc.sendConfig(mockConn, cfg); err == nil {
			t.Error("Expected error when connection fails")
		}
	})

	t.Run("ServiceLifecycle", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConnectionService(false, bus)

		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Errorf("Start should not return error, got: %v", err)
		}

		if err := svc.Stop(ctx); err != nil {
			t.Errorf("Stop should not return error, got: %v", err)
		}
	})

	t.Run("EventHandling", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConnectionService(false, bus)

		// Need to start the service to initialize the context
		ctx := context.Background()
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		defer func() { _ = svc.Stop(ctx) }()

		reconnectReceived := make(chan struct{})

		bus.Subscribe(event.ReconnectRequested, func(evt event.Event) {
			close(reconnectReceived)
		})

		cfg := config.Config{}
		cfg.Application.Hostname = "updated-host"
		bus.Publish(event.Event{Type: event.ConfigUpdated, Data: cfg})

		time.Sleep(10 * time.Millisecond)
		if svc.GetConfig().Application.Hostname != "updated-host" {
			t.Error("Config should be updated after event")
		}

		select {
		case <-reconnectReceived:
			// Expected behavior
		case <-time.After(100 * time.Millisecond):
			t.Error("Should have received reconnect event")
		}
	})
}
