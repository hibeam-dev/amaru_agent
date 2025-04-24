package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func TestConnectionService(t *testing.T) {
	t.Run("JSONMode", func(t *testing.T) {
		bus := NewEventBus()
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
		bus := NewEventBus()
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
		bus := NewEventBus()
		svc := NewConnectionService(false, bus)

		if svc.HasConnection() {
			t.Error("New service should not have an active connection")
		}
	})

	t.Run("SendConfig", func(t *testing.T) {
		mockConn := ssh.NewMockConnection()
		bus := NewEventBus()
		svc := NewConnectionService(false, bus)

		cfg := config.Config{}
		cfg.Application.Hostname = "test-host"
		cfg.Application.Port = 8080
		cfg.Agent.Tags = map[string]string{"env": "test"}

		if err := svc.sendConfig(mockConn, cfg); err != nil {
			t.Errorf("Expected no error on send, got: %v", err)
		}

		writtenData := mockConn.GetWrittenData()
		if len(writtenData) == 0 {
			t.Error("No data was written to connection")
		}

		if !strings.Contains(string(writtenData), `"type":"config_update"`) {
			t.Errorf("Expected config_update in payload, got: %s", string(writtenData))
		}

		testErr := errors.New("test error")
		mockConn.SetSendPayloadError(testErr)

		if err := svc.sendConfig(mockConn, cfg); err == nil {
			t.Error("Expected error when connection fails")
		}
	})

	t.Run("ServiceLifecycle", func(t *testing.T) {
		bus := NewEventBus()
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
		bus := NewEventBus()
		svc := NewConnectionService(false, bus)

		// Need to start the service to initialize the context
		ctx := context.Background()
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		defer func() { _ = svc.Stop(ctx) }()

		reconnectCh := bus.Subscribe(ReconnectRequested, 1)

		cfg := config.Config{}
		cfg.Application.Hostname = "updated-host"
		bus.Publish(Event{Type: ConfigUpdated, Data: cfg})

		time.Sleep(10 * time.Millisecond) // Wait for processing
		if svc.GetConfig().Application.Hostname != "updated-host" {
			t.Error("Config should be updated after event")
		}

		select {
		case <-reconnectCh:
			// Expected behavior
		case <-time.After(100 * time.Millisecond):
			t.Error("Should have received reconnect event")
		}
	})
}
