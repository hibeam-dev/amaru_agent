package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"erlang-solutions.com/amaru_agent/internal/app"
	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/daemon"
	"erlang-solutions.com/amaru_agent/internal/event"
	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/register"
	"erlang-solutions.com/amaru_agent/internal/registry"
	"erlang-solutions.com/amaru_agent/internal/transport"
	"erlang-solutions.com/amaru_agent/internal/util"
)

var (
	writePid     = flag.Bool("pid", false, "")
	configFile   = flag.String("config", "config.toml", "")
	genKey       = flag.Bool("genkey", false, "")
	registerFlag = flag.String("register", "", "")
	keyFile      = flag.String("key", "", "")
)

func init() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(w, "%s\n\n", i18n.T("usage_header", map[string]any{}))
		_, _ = fmt.Fprintf(w, "%s\n", i18n.T("usage_format", map[string]any{}))
		_, _ = fmt.Fprintf(w, "\n%s\n", i18n.T("options_header", map[string]any{}))

		_, _ = fmt.Fprintf(w, "  -pid\n    \t%s\n", i18n.T("flag_pid_desc", map[string]any{}))
		_, _ = fmt.Fprintf(w, "  -config %s\n    \t%s\n", "filename", i18n.T("flag_config_desc", map[string]any{}))
		_, _ = fmt.Fprintf(w, "  -genkey\n    \t%s\n", i18n.T("flag_genkey_desc", map[string]any{}))
		_, _ = fmt.Fprintf(w, "  -register %s\n    \t%s\n", "token", i18n.T("flag_register_desc", map[string]any{}))
		_, _ = fmt.Fprintf(w, "  -key %s\n    \t%s\n", "filepath", i18n.T("flag_key_desc", map[string]any{}))
	}
}

func setupLogging(cfg config.Config) error {
	logLevel := util.ParseLogLevel(cfg.Logging.Level)

	if cfg.Logging.LogFile != "" {
		logDir := filepath.Dir(cfg.Logging.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		logFile, err := os.OpenFile(cfg.Logging.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		util.SetDefaultLogger(logLevel, logFile)
	} else {
		util.SetDefaultLogger(logLevel, os.Stderr)
	}

	return nil
}

func main() {
	util.InitDefaultLogger()

	if err := i18n.InitDefaultFS(); err != nil {
		// Can't use i18n.T here since i18n init failed
		util.Warn("Failed to initialize localization", map[string]any{"component": "main", "error": err})
	}

	registry.InitTransports()

	flag.Parse()

	if *genKey {
		generator := transport.NewEd25519Generator()
		keyPath := config.DefaultKeyFile()
		keyPair, err := generator.GenerateKey(keyPath)
		if err != nil {
			util.Error(i18n.T("genkey_error", map[string]any{"Error": err}), map[string]any{"component": "main"})
			os.Exit(1)
		}
		fmt.Print(string(keyPair.PublicKey))
		os.Exit(0)
	}

	if *registerFlag != "" {
		backendHost := os.Getenv("AMARU_BACKEND")
		if backendHost == "" {
			backendHost = "https://app.amaru.cloud"
		}

		var keyPath string
		if *keyFile != "" {
			keyPath = *keyFile
		} else {
			keyPath = config.DefaultKeyFile() + ".pub"
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				generator := transport.NewEd25519Generator()
				privateKeyPath := config.DefaultKeyFile()
				_, genErr := generator.GenerateKey(privateKeyPath)
				if genErr != nil {
					util.Error(i18n.T("genkey_error", map[string]any{"Error": genErr}), map[string]any{"component": "main"})
					os.Exit(1)
				}
				util.Info(i18n.T("genkey_auto_success", map[string]any{"Path": privateKeyPath}), map[string]any{"component": "main"})
			}
		}

		registerClient := register.NewClient()
		if err := registerClient.RegisterWithBackend(*registerFlag, keyPath, backendHost); err != nil {
			util.Error(i18n.T("register_error", map[string]any{"Error": err}), map[string]any{"component": "main"})
			os.Exit(1)
		}

		util.Info(i18n.T("register_success", map[string]any{}), map[string]any{"component": "main"})
		os.Exit(0)
	}

	if *writePid {
		if err := daemon.WritePidFile(); err != nil {
			util.Error(i18n.T("pid_file_error", map[string]any{"Error": err}), map[string]any{"component": "main"})
		}
	}

	ctx := context.Background()

	application := app.NewApp(*configFile)

	cfg, err := config.Load(*configFile)
	if err == nil {
		if setupErr := setupLogging(cfg); setupErr != nil {
			util.Warn("Failed to set up initial logging configuration", map[string]any{"component": "main", "error": setupErr})
		}
	}

	eventBus := application.GetEventBus()
	eventBus.Subscribe(event.ConfigUpdated, func(evt event.Event) {
		if cfg, ok := evt.Data.(config.Config); ok {
			if err := setupLogging(cfg); err != nil {
				util.Error("Failed to update logging configuration", map[string]any{"component": "main", "error": err})
			}
		}
	})

	if err := application.Run(ctx); err != nil {
		if util.IsExpectedError(err) {
			util.Info(i18n.T("app_terminated", map[string]any{"Reason": "expected termination"}), map[string]any{"component": "main"})
		} else {
			util.Error(i18n.T("app_error", map[string]any{"Error": err}), map[string]any{"component": "main"})
			os.Exit(1)
		}
	}
}
