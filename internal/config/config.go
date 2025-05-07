package config

import (
	"time"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/util"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Connection struct {
		Host       string
		Port       int
		User       string
		Timeout    time.Duration
		KeyFile    string
		KnownHosts string `toml:",omitempty"`
		Tunnel     bool
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

func Load(path string) (Config, error) {
	var config Config

	// Defaults
	config.Connection.Timeout = 30 * time.Second

	_, err := toml.DecodeFile(path, &config)
	if err != nil {
		return config, util.WrapWithBase(util.ErrConfigLoad, "failed to parse config file", err)
	}

	if config.Connection.Host == "" {
		return config, util.WrapWithBase(util.ErrConfigLoad, i18n.T("connection_host_missing", map[string]any{}), nil)
	}
	if config.Connection.User == "" {
		return config, util.WrapWithBase(util.ErrConfigLoad, i18n.T("connection_user_missing", map[string]any{}), nil)
	}
	if config.Connection.KeyFile == "" {
		return config, util.WrapWithBase(util.ErrConfigLoad, i18n.T("connection_keyfile_missing", map[string]any{}), nil)
	}

	return config, nil
}
