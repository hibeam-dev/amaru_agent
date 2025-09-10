package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/util"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Connection struct {
		Host       string
		Port       int
		Timeout    time.Duration
		KeyFile    string
		KnownHosts string `toml:",omitempty"`
	}
	Application struct {
		Hostname string
		Port     int
		IP       string `toml:",omitempty"`
		Tags     map[string]string
		Security map[string]bool
	}
	Logging struct {
		Level   string
		LogFile string `toml:",omitempty"`
	}
}

func getDefaultLogFile() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = os.Getenv("APPDATA")
		}
		if appData != "" {
			return filepath.Join(appData, "Amaru", "amaru_agent.log")
		}
		return filepath.Join(os.TempDir(), "amaru_agent.log")
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, "Library", "Logs", "amaru_agent.log")
		}
		return filepath.Join(os.TempDir(), "amaru_agent.log")
	default:
		if os.Geteuid() == 0 {
			return "/var/log/amaru_agent.log"
		}
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, ".local", "share", "amaru", "amaru_agent.log")
		}
		return filepath.Join(os.TempDir(), "amaru_agent.log")
	}
}

func Load(path string) (Config, error) {
	var config Config

	// Defaults
	config.Connection.Timeout = 30 * time.Second

	_, err := toml.DecodeFile(path, &config)
	if err != nil {
		return config, util.WrapWithBase(util.ErrConfigLoad, "failed to parse config file", err)
	}

	if config.Logging.LogFile == "" {
		config.Logging.LogFile = getDefaultLogFile()
	}

	if config.Connection.Host == "" {
		return config, util.WrapWithBase(util.ErrConfigLoad, i18n.T("connection_host_missing", map[string]any{}), nil)
	}
	if config.Connection.KeyFile == "" {
		return config, util.WrapWithBase(util.ErrConfigLoad, i18n.T("connection_keyfile_missing", map[string]any{}), nil)
	}

	return config, nil
}
