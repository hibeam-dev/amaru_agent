package app

import (
	"context"
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
	app := NewApp("test-config.toml", true)

	if app == nil {
		t.Fatal("Expected app to be non-nil")
	}
}

func TestLoadConfig(t *testing.T) {
	t.Skip("Need to mock ConfigService for proper testing")
}

func TestRunOnce(t *testing.T) {
	t.Skip("Need to mock ConnectionService for proper testing")
}

func TestRunWithReconnect(t *testing.T) {
	t.Skip("Need to mock dependencies for proper testing")
}

func TestExecuteConnection(t *testing.T) {
	t.Skip("Need to mock SSH connection for proper testing")
}

func TestAppRunCancel(t *testing.T) {
	t.Skip("Need to mock services for proper testing")

	app := NewApp("test-config.toml", true)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := app.Run(ctx)
	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}
}
