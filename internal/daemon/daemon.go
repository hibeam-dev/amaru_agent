package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hibeam-dev/amaru_agent/internal/i18n"
	"github.com/hibeam-dev/amaru_agent/internal/util"
)

func WritePidFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return util.NewError(util.ErrTypeConfig, i18n.T("home_dir_error", map[string]any{}), err)
	}

	pidDir := filepath.Join(home, ".amaru")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return util.NewError(util.ErrTypeConfig, i18n.T("pid_dir_create_error", map[string]any{}), err)
	}

	pidPath := filepath.Join(pidDir, "amaru.pid")
	pid := os.Getpid()

	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d", pid), 0644); err != nil {
		return util.NewError(util.ErrTypeConfig, i18n.T("pid_file_write_error", map[string]any{}), err)
	}

	util.Info(i18n.T("pid_file_written", map[string]any{
		"Path": pidPath,
		"PID":  pid,
	}), map[string]any{"component": "daemon"})

	return nil
}
