package main

import (
	"context"
	"errors"
	"flag"
	"log"

	"erlang-solutions.com/cortex_agent/internal/app"
	"erlang-solutions.com/cortex_agent/internal/daemon"
	"erlang-solutions.com/cortex_agent/internal/i18n"
)

var (
	jsonMode   = flag.Bool("json", false, "Enable JSON communication mode for application integration")
	writePid   = flag.Bool("pid", false, "Write PID file to ~/.cortex_agent/cortex_agent.pid")
	configFile = flag.String("config", "config.toml", "Path to TOML configuration file")
)

func main() {
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if err := i18n.InitDefaultFS(); err != nil {
		log.Printf(i18n.Tf("i18n_init_error", nil), i18n.T("i18n_init_error", map[string]interface{}{"Error": err}))
	}

	if *writePid {
		if err := daemon.WritePidFile(); err != nil {
			log.Printf(i18n.Tf("pid_file_error", nil), i18n.T("pid_file_error", map[string]interface{}{"Error": err}))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	app.SetupTerminationHandler(ctx, cancel)

	application := app.New(*configFile, *jsonMode)
	if err := application.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println(i18n.T("app_context_terminated", nil))
		} else {
			log.Fatalf(i18n.Tf("app_error", nil), i18n.T("app_error", map[string]interface{}{"Error": err}))
		}
	}
}
