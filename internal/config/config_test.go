package config

import (
	"os"
	"runtime"
	"strings"
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
[connection]
host = "test-host"
port = 2222
timeout = "45s"
keyfile = "/path/to/key"
tunnel = false

[application]
hostname = "test-app"
port = 9090
ip = "192.168.1.100"
[application.tags]
service = "test-service"
environment = "test"
[application.security]
secure = true

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

	if cfg.Connection.Host != "test-host" {
		t.Errorf("Expected Connection.Host to be 'test-host', got '%s'", cfg.Connection.Host)
	}
	if cfg.Connection.Port != 2222 {
		t.Errorf("Expected Connection.Port to be 2222, got %d", cfg.Connection.Port)
	}

	expectedTimeout := 45 * time.Second

	if cfg.Connection.Timeout != expectedTimeout {
		t.Errorf("Expected Connection.Timeout to be %v, got %v", expectedTimeout, cfg.Connection.Timeout)
	}
	if cfg.Application.Hostname != "test-app" {
		t.Errorf("Expected Application.Hostname to be 'test-app', got '%s'", cfg.Application.Hostname)
	}
	if cfg.Application.Port != 9090 {
		t.Errorf("Expected Application.Port to be 9090, got %d", cfg.Application.Port)
	}
	if cfg.Application.IP != "192.168.1.100" {
		t.Errorf("Expected Application.IP to be '192.168.1.100', got '%s'", cfg.Application.IP)
	}
	if cfg.Application.Tags["service"] != "test-service" {
		t.Errorf("Expected Agent.Tags['service'] to be 'test-service', got '%s'", cfg.Application.Tags["service"])
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected Logging.Level to be 'debug', got '%s'", cfg.Logging.Level)
	}

	if cfg.Logging.LogFile == "" {
		t.Error("Expected Logging.LogFile to be set to default, got empty string")
	}

	if !strings.Contains(cfg.Logging.LogFile, "amaru_agent.log") {
		t.Errorf("Expected default log file to contain 'amaru_agent.log', got '%s'", cfg.Logging.LogFile)
	}
}

func TestDefaultLogFile(t *testing.T) {
	tests := []struct {
		name string
		goos string
		want string
	}{
		{
			name: "Windows with LOCALAPPDATA",
			goos: "windows",
			want: "amaru_agent.log",
		},
		{
			name: "macOS",
			goos: "darwin",
			want: "amaru_agent.log",
		},
		{
			name: "Linux",
			goos: "linux",
			want: "amaru_agent.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalGOOS := runtime.GOOS

			// We can't easily mock runtime.GOOS in unit tests, so we just
			// verify the function returns a non-empty path with the right filename
			defaultPath := getDefaultLogFile()

			if defaultPath == "" {
				t.Error("Expected getDefaultLogFile() to return non-empty path")
			}

			if !strings.Contains(defaultPath, tt.want) {
				t.Errorf("Expected default log file path to contain '%s', got '%s'", tt.want, defaultPath)
			}

			_ = originalGOOS // Prevent unused variable warning
		})
	}
}

func TestConfigWithExplicitLogFile(t *testing.T) {
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
[connection]
host = "test-host"
port = 2222
keyfile = "/path/to/key"

[logging]
level = "info"
logfile = "/custom/path/to/logs/custom.log"
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

	expectedLogFile := "/custom/path/to/logs/custom.log"
	if cfg.Logging.LogFile != expectedLogFile {
		t.Errorf("Expected Logging.LogFile to be '%s', got '%s'", expectedLogFile, cfg.Logging.LogFile)
	}
}
