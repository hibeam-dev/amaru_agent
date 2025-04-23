package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPidFileCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "daemon-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}()

	pidDir := filepath.Join(tmpDir, ".cortex_agent")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatalf("Failed to create PID directory: %v", err)
	}
	pidPath := filepath.Join(pidDir, "cortex_agent.pid")
	pid := os.Getpid()
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		t.Fatalf("Failed to write test PID file: %v", err)
	}

	content, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	expectedPid := fmt.Sprintf("%d", pid)
	if string(content) != expectedPid {
		t.Errorf("Expected PID file to contain %s, got %s", expectedPid, content)
	}
}
