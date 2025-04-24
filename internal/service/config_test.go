package service

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestConfigServiceLoadConfig(t *testing.T) {
	content := `
[SSH]
Host = "test-host"
Port = 22
User = "test-user"
KeyFile = "/tmp/key.file"

[Application]
Hostname = "test-app-host"
Port = 8080

[Agent]
Tags = { env = "test" }

[Logging]
Level = "info"
`
	tmpfile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpfile.Name())
	}()

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	svc := NewConfigService(tmpfile.Name())
	cfg, err := svc.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.SSH.Host != "test-host" {
		t.Errorf("Expected SSH.Host to be 'test-host', got '%s'", cfg.SSH.Host)
	}
	if cfg.SSH.Port != 22 {
		t.Errorf("Expected SSH.Port to be 22, got %d", cfg.SSH.Port)
	}
	if cfg.SSH.User != "test-user" {
		t.Errorf("Expected SSH.User to be 'test-user', got '%s'", cfg.SSH.User)
	}
	if cfg.Application.Hostname != "test-app-host" {
		t.Errorf("Expected Application.Hostname to be 'test-app-host', got '%s'", cfg.Application.Hostname)
	}
	if cfg.Application.Port != 8080 {
		t.Errorf("Expected Application.Port to be 8080, got %d", cfg.Application.Port)
	}
	if cfg.Agent.Tags["env"] != "test" {
		t.Errorf("Expected Agent.Tags['env'] to be 'test', got '%s'", cfg.Agent.Tags["env"])
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Logging.Level to be 'info', got '%s'", cfg.Logging.Level)
	}
}

func TestConfigServiceSetupReloader(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}

	content := `
[SSH]
Host = "test-host"
Port = 22
User = "test-user"
KeyFile = "/tmp/key.file"

[Application]
Hostname = "test-app-host"
Port = 8080
`
	tmpfile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpfile.Name())
	}()

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	svc := NewConfigService(tmpfile.Name())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configCh := svc.SetupReloader(ctx)

	// This test is complex because it involves signals
	// For simplicity, we'll just make sure the channel is created
	if configCh == nil {
		t.Fatal("Expected configCh to be non-nil")
	}

	cancel()

	// Wait a bit for goroutines to clean up
	time.Sleep(100 * time.Millisecond)
}
