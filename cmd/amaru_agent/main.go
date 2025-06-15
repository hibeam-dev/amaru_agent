package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"erlang-solutions.com/amaru_agent/internal/app"
	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/daemon"
	"erlang-solutions.com/amaru_agent/internal/event"
	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/registry"
	"erlang-solutions.com/amaru_agent/internal/util"
)

var (
	writePid   = flag.Bool("pid", false, "")
	configFile = flag.String("config", "config.toml", "")
)

func init() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(w, "%s\n\n", i18n.T("usage_header", map[string]any{}))
		_, _ = fmt.Fprintf(w, "%s\n", i18n.T("usage_format", map[string]any{}))
		_, _ = fmt.Fprintf(w, "\n%s\n", i18n.T("options_header", map[string]any{}))

		_, _ = fmt.Fprintf(w, "  -pid\n    \t%s\n", i18n.T("flag_pid_desc", map[string]any{}))
		_, _ = fmt.Fprintf(w, "  -config %s\n    \t%s\n", "filename", i18n.T("flag_config_desc", map[string]any{}))
	}
}

func setupLogging(cfg config.Config) error {
	logLevel := util.ParseLogLevel(cfg.Logging.Level)

	if cfg.Logging.LogFile != "" {
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
		util.Warn("Warning: Failed to initialize localization", "error", err)
	}

	registry.InitTransports()

	flag.Parse()

	if *writePid {
		if err := daemon.WritePidFile(); err != nil {
			util.Error(i18n.T("pid_file_error", map[string]any{"Error": err}))
		}
	}

	ctx := context.Background()

	application := app.NewApp(*configFile)

	cfg, err := config.Load(*configFile)
	if err == nil {
		if setupErr := setupLogging(cfg); setupErr != nil {
			util.Warn("Failed to set up initial logging configuration", "error", setupErr)
		}
	}

	eventBus := application.GetEventBus()
	eventBus.Subscribe(event.ConfigUpdated, func(evt event.Event) {
		if cfg, ok := evt.Data.(config.Config); ok {
			if err := setupLogging(cfg); err != nil {
				util.Error("Failed to update logging configuration", "error", err)
			}
		}
	})

	if err := application.Run(ctx); err != nil {
		if util.IsExpectedError(err) {
			util.Info(i18n.T("app_terminated", map[string]any{"Reason": "expected termination"}))
		} else {
			util.Error(i18n.T("app_error", map[string]any{"Error": err}))
			os.Exit(1)
		}
	}
}
