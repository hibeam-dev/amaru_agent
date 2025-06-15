package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWritePidFile(t *testing.T) {
	originalHome := os.Getenv("HOME")

	tmpDir, err := os.MkdirTemp("", "daemon-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	defer func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Logf("Warning: Failed to restore HOME env var: %v", err)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Warning: Failed to remove temp directory: %v", err)
		}
	}()

	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Failed to set HOME environment variable: %v", err)
	}

	if err := WritePidFile(); err != nil {
		t.Fatalf("WritePidFile() failed: %v", err)
	}

	pidPath := filepath.Join(tmpDir, ".amaru_agent", "amaru_agent.pid")
	content, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	pid, err := strconv.Atoi(string(content))
	if err != nil {
		t.Fatalf("PID file contains invalid data: %s", content)
	}

	if pid != os.Getpid() {
		t.Errorf("Expected PID %d, got %d", os.Getpid(), pid)
	}
}
