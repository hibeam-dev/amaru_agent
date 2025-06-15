package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/util"
)

func WritePidFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return util.NewError(util.ErrTypeConfig, i18n.T("home_dir_error", map[string]any{}), err)
	}

	pidDir := filepath.Join(home, ".amaru_agent")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return util.NewError(util.ErrTypeConfig, i18n.T("pid_dir_create_error", map[string]any{}), err)
	}

	pidPath := filepath.Join(pidDir, "amaru_agent.pid")
	pid := os.Getpid()

	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d", pid), 0644); err != nil {
		return util.NewError(util.ErrTypeConfig, i18n.T("pid_file_write_error", map[string]any{}), err)
	}

	util.Info(i18n.T("pid_file_written", map[string]any{
		"Path": pidPath,
		"PID":  pid,
	}))

	return nil
}
