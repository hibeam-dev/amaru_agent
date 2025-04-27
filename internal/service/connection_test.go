package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/transport/mocks"
	"go.uber.org/mock/gomock"
)

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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockConn := mocks.NewMockConnection(ctrl)
		bus := event.NewBus()
		svc := NewConnectionService(false, bus)

		var capturedPayload []byte
		mockConn.EXPECT().SendPayload(gomock.Any()).DoAndReturn(func(payload any) error {
			data, _ := json.Marshal(payload)
			capturedPayload = data
			return nil
		})

		cfg := config.Config{}
		cfg.Application.Hostname = "test-host"
		cfg.Application.Port = 8080
		cfg.Agent.Tags = map[string]string{"env": "test"}

		if err := svc.sendConfig(mockConn, cfg); err != nil {
			t.Errorf("Expected no error on send, got: %v", err)
		}

		if len(capturedPayload) == 0 {
			t.Error("No data was captured in the payload")
		}

		if !strings.Contains(string(capturedPayload), `"type":"config_update"`) {
			t.Errorf("Expected config_update in payload, got: %s", string(capturedPayload))
		}

		testErr := errors.New("test error")
		mockConn.EXPECT().SendPayload(gomock.Any()).Return(testErr)

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
		bus.Publish(event.Event{Type: event.ConfigUpdated, Data: cfg, Ctx: context.Background()})

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
