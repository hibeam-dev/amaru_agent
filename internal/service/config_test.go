package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/event"
)

const testConfigContent = `
[Connection]
Host = "test-host"
Port = 22
User = "test-user"
KeyFile = "/tmp/key.file"

[Application]
Hostname = "test-app-host"
Port = 8080
Tags = { env = "test" }
Security = { secure = true }

[Logging]
Level = "info"
`

func setupTestConfig(t *testing.T, content string) (string, func()) {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	cleanup := func() {
		_ = os.Remove(tmpfile.Name())
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		cleanup()
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		cleanup()
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpfile.Name(), cleanup
}

func TestConfigService(t *testing.T) {
	t.Run("LoadConfig", func(t *testing.T) {
		configPath, cleanup := setupTestConfig(t, testConfigContent)
		defer cleanup()

		bus := event.NewBus()
		svc := NewConfigService(configPath, bus)

		cfg, err := svc.LoadConfig()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if cfg.Connection.Host != "test-host" {
			t.Errorf("Expected Connection.Host to be 'test-host', got '%s'", cfg.Connection.Host)
		}
		if cfg.Connection.Port != 22 {
			t.Errorf("Expected Connection.Port to be 22, got %d", cfg.Connection.Port)
		}
		if cfg.Application.Hostname != "test-app-host" {
			t.Errorf("Expected Application.Hostname to be 'test-app-host', got '%s'", cfg.Application.Hostname)
		}
		if cfg.Application.Tags["env"] != "test" {
			t.Errorf("Expected Application.Tags['env'] to be 'test', got '%s'", cfg.Application.Tags["env"])
		}
		if cfg.Logging.Level != "info" {
			t.Errorf("Expected Logging.Level to be 'info', got '%s'", cfg.Logging.Level)
		}

		storedCfg := svc.GetConfig()
		if storedCfg.Connection.Host != "test-host" {
			t.Errorf("Expected stored config Connection.Host to be 'test-host', got '%s'", storedCfg.Connection.Host)
		}
	})

	t.Run("StartStop", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "config.toml")
		bus := event.NewBus()
		svc := NewConfigService(tmpFile, bus)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Expected no error from Start(), got: %v", err)
		}

		if err := svc.Stop(ctx); err != nil {
			t.Fatalf("Expected no error from Stop(), got: %v", err)
		}
	})

	t.Run("GetSetConfig", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConfigService("config.toml", bus)

		testCfg := config.Config{}
		testCfg.Application.Hostname = "test-host"
		testCfg.Application.Port = 8080
		testCfg.Application.Tags = map[string]string{"env": "test"}
		testCfg.Application.Security = map[string]bool{"secure": true}

		svc.SetConfig(testCfg)

		retrievedCfg := svc.GetConfig()
		if retrievedCfg.Application.Hostname != "test-host" {
			t.Errorf("Expected hostname to be test-host, got %s", retrievedCfg.Application.Hostname)
		}
		if retrievedCfg.Application.Port != 8080 {
			t.Errorf("Expected port to be 8080, got %d", retrievedCfg.Application.Port)
		}
		if retrievedCfg.Application.Tags["env"] != "test" {
			t.Errorf("Expected env tag to be test, got %s", retrievedCfg.Application.Tags["env"])
		}
	})

	t.Run("ReloadAndPublishConfig", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping test in short mode")
		}

		configPath, cleanup := setupTestConfig(t, testConfigContent)
		defer cleanup()

		bus := event.NewBus()
		svc := NewConfigService(configPath, bus)

		var receivedConfig config.Config
		configReceived := make(chan struct{})

		unsub := bus.Subscribe(event.ConfigUpdated, func(evt event.Event) {
			cfg, ok := evt.Data.(config.Config)
			if ok {
				receivedConfig = cfg
				close(configReceived)
			}
		})
		defer unsub()

		svc.reloadAndPublishConfig()

		select {
		case <-configReceived:
			if receivedConfig.Connection.Host != "test-host" {
				t.Errorf("Expected Connection.Host to be 'test-host', got '%s'", receivedConfig.Connection.Host)
			}

		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for config update event")
		}

		storedCfg := svc.GetConfig()
		if storedCfg.Connection.Host != "test-host" {
			t.Errorf("Expected stored config Connection.Host to be 'test-host', got '%s'", storedCfg.Connection.Host)
		}
	})

	t.Run("SIGHUPHandler", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping test in short mode - requires file system")
		}

		initialContent := `
[Connection]
Host = "initial-host"
Port = 22
User = "test-user"
KeyFile = "/tmp/key.file"

[Application]
Hostname = "initial-app"
Port = 8080
`
		configPath, cleanup := setupTestConfig(t, initialContent)
		defer cleanup()

		bus := event.NewBus()
		svc := NewConfigService(configPath, bus)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}

		var receivedConfig config.Config
		configReceived := make(chan struct{})

		unsub := bus.Subscribe(event.ConfigUpdated, func(evt event.Event) {
			cfg, ok := evt.Data.(config.Config)
			if ok {
				receivedConfig = cfg
				close(configReceived)
			}
		})
		defer unsub()

		updatedContent := `
[Connection]
Host = "updated-host"
Port = 22
User = "test-user"
KeyFile = "/tmp/key.file"

[Application]
Hostname = "updated-app"
Port = 8080
`
		if err := os.WriteFile(configPath, []byte(updatedContent), 0644); err != nil {
			t.Fatalf("Failed to update config file: %v", err)
		}

		bus.Publish(event.Event{
			Ctx:  context.Background(),
			Type: event.SIGHUPReceived,
		})

		select {
		case <-configReceived:
			if receivedConfig.Connection.Host != "updated-host" {
				t.Errorf("Expected Connection.Host to be 'updated-host', got '%s'", receivedConfig.Connection.Host)
			}
			if receivedConfig.Application.Hostname != "updated-app" {
				t.Errorf("Expected Application.Hostname to be 'updated-app', got '%s'", receivedConfig.Application.Hostname)
			}

		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for config update event after SIGHUP")
		}

		storedCfg := svc.GetConfig()
		if storedCfg.Connection.Host != "updated-host" {
			t.Errorf("Expected stored config Connection.Host to be 'updated-host', got '%s'", storedCfg.Connection.Host)
		}

		if err := svc.Stop(ctx); err != nil {
			t.Fatalf("Failed to stop service: %v", err)
		}
	})
}
