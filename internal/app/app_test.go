package app

import (
	"context"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func TestSendInitialConfig(t *testing.T) {
	t.Skip("Need to fix the implementation to make it more testable")
}

func TestSetupTerminationHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	SetupTerminationHandler(ctx, cancel)

	time.Sleep(10 * time.Millisecond)
}

func TestAppCreation(t *testing.T) {
	app := New("test-config.toml", true)

	if app.ConfigFile != "test-config.toml" {
		t.Errorf("Expected ConfigFile to be 'test-config.toml', got '%s'", app.ConfigFile)
	}

	if !app.JSONMode {
		t.Error("Expected JSONMode to be true")
	}
}

func TestSendInitialConfigError(t *testing.T) {
	app := &App{}
	mockConn := ssh.NewMockConnection()

	mockConn.SetSendPayloadError(context.Canceled)

	cfg := config.Config{}

	err := app.sendInitialConfig(mockConn, cfg)
	if err == nil {
		t.Fatal("Expected an error but got none")
	}
}
