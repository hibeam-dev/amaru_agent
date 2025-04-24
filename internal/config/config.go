package config

import (
	"fmt"
	"time"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/pkg/errors"
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
	}
	Application struct {
		Hostname string
		Port     int
	}
	Agent struct {
		Tags map[string]string
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
		return config, fmt.Errorf("%w: %v", errors.ErrConfigLoad, err)
	}

	// Validate essential config fields
	if config.Connection.Host == "" {
		return config, fmt.Errorf("%w: %s", errors.ErrConfigLoad,
			i18n.T("connection_host_missing", nil))
	}
	if config.Connection.User == "" {
		return config, fmt.Errorf("%w: %s", errors.ErrConfigLoad,
			i18n.T("connection_user_missing", nil))
	}
	if config.Connection.KeyFile == "" {
		return config, fmt.Errorf("%w: %s", errors.ErrConfigLoad,
			i18n.T("connection_keyfile_missing", nil))
	}

	return config, nil
}
