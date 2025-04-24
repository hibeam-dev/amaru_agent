package service

import (
	"errors"
	"testing"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func TestConnectionServiceIsJSONMode(t *testing.T) {
	svc := NewConnectionService(true)
	if !svc.IsJSONMode() {
		t.Error("Expected IsJSONMode() to return true")
	}

	svc = NewConnectionService(false)
	if svc.IsJSONMode() {
		t.Error("Expected IsJSONMode() to return false")
	}
}

func TestSendInitialConfig(t *testing.T) {
	mockConn := ssh.NewMockConnection()

	svc := NewConnectionService(false)

	cfg := config.Config{}
	cfg.Application.Hostname = "test-host"
	cfg.Application.Port = 8080
	cfg.Agent.Tags = map[string]string{"env": "test"}

	err := svc.SendInitialConfig(mockConn, cfg)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	writtenData := mockConn.GetWrittenData()
	if len(writtenData) == 0 {
		t.Error("Expected data to be written, got empty buffer")
	}

	testErr := errors.New("test error")
	mockConn.SetSendPayloadError(testErr)

	err = svc.SendInitialConfig(mockConn, cfg)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestConnectionServiceRun(t *testing.T) {
	t.Skip("This test needs to be redesigned")
}
