package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tempFile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	configContent := `
[ssh]
host = "test-host"
port = 2222
user = "test-user"
timeout = "45s"
keyfile = "/path/to/key"

[application]
hostname = "test-app"
port = 9090

[agent]
[agent.tags]
service = "test-service"
environment = "test"

[logging]
level = "debug"
`
	if _, err := tempFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cfg, err := Load(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.SSH.Host != "test-host" {
		t.Errorf("Expected SSH.Host to be 'test-host', got '%s'", cfg.SSH.Host)
	}
	if cfg.SSH.Port != 2222 {
		t.Errorf("Expected SSH.Port to be 2222, got %d", cfg.SSH.Port)
	}
	if cfg.SSH.User != "test-user" {
		t.Errorf("Expected SSH.User to be 'test-user', got '%s'", cfg.SSH.User)
	}
	expectedTimeout := 45 * time.Second
	if cfg.SSH.Timeout != expectedTimeout {
		t.Errorf("Expected SSH.Timeout to be %v, got %v", expectedTimeout, cfg.SSH.Timeout)
	}
	if cfg.Application.Hostname != "test-app" {
		t.Errorf("Expected Application.Hostname to be 'test-app', got '%s'", cfg.Application.Hostname)
	}
	if cfg.Application.Port != 9090 {
		t.Errorf("Expected Application.Port to be 9090, got %d", cfg.Application.Port)
	}
	if cfg.Agent.Tags["service"] != "test-service" {
		t.Errorf("Expected Agent.Tags['service'] to be 'test-service', got '%s'", cfg.Agent.Tags["service"])
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected Logging.Level to be 'debug', got '%s'", cfg.Logging.Level)
	}
}
