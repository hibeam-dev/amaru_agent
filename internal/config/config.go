package config

import (
	"fmt"
	"time"

	"erlang-solutions.com/cortex_agent/pkg/errors"
	"github.com/BurntSushi/toml"
)

type Config struct {
	SSH struct {
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
	config.SSH.Timeout = 30 * time.Second

	_, err := toml.DecodeFile(path, &config)
	if err != nil {
		return config, fmt.Errorf("%w: %v", errors.ErrConfigLoad, err)
	}

	// Validate essential config fields
	if config.SSH.Host == "" {
		return config, fmt.Errorf("%w: SSH host not specified", errors.ErrConfigLoad)
	}
	if config.SSH.User == "" {
		return config, fmt.Errorf("%w: SSH user not specified", errors.ErrConfigLoad)
	}
	if config.SSH.KeyFile == "" {
		return config, fmt.Errorf("%w: SSH key file not specified", errors.ErrConfigLoad)
	}

	return config, nil
}
