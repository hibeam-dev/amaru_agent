package main

import (
	"context"
	"errors"
	"flag"
	"log"

	"erlang-solutions.com/cortex_agent/internal/app"
	"erlang-solutions.com/cortex_agent/internal/daemon"
)

var (
	jsonMode   = flag.Bool("json", false, "Enable JSON communication mode for application integration")
	writePid   = flag.Bool("pid", false, "Write PID file to ~/.cortex_agent/cortex_agent.pid")
	configFile = flag.String("config", "config.toml", "Path to TOML configuration file")
)

func main() {
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Write PID file if requested
	if *writePid {
		if err := daemon.WritePidFile(); err != nil {
			log.Printf("Warning: Failed to write PID file: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	app.SetupTerminationHandler(ctx, cancel)

	// Run the application
	application := app.New(*configFile, *jsonMode)
	if err := application.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println("Application terminated due to context cancellation")
		} else {
			log.Fatalf("Application error: %v", err)
		}
	}
}
