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
		return fmt.Errorf("%s: %w", i18n.T("home_dir_error", nil), err)
	}

	pidDir := filepath.Join(home, ".cortex_agent")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("pid_dir_create_error", nil), err)
	}

	pidPath := filepath.Join(pidDir, "cortex_agent.pid")
	pid := os.Getpid()

	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("pid_file_write_error", nil), err)
	}

	log.Printf("%s", i18n.T("pid_file_written", map[string]any{
		"Path": pidPath,
		"PID":  pid,
	}))
	return nil
}
