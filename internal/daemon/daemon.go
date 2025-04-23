package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"erlang-solutions.com/cortex_agent/internal/i18n"
)

func WritePidFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unable to determine user home directory: %w", err)
	}

	pidDir := filepath.Join(home, ".cortex_agent")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	pidPath := filepath.Join(pidDir, "cortex_agent.pid")
	pid := os.Getpid()

	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	log.Printf(i18n.Tf("pid_file_written", nil),
		i18n.T("pid_file_written", map[string]interface{}{
			"Path": pidPath,
			"PID":  pid,
		}))
	return nil
}
