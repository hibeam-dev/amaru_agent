package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"

	"erlang-solutions.com/cortex_agent/internal/app"
	"erlang-solutions.com/cortex_agent/internal/daemon"
	"erlang-solutions.com/cortex_agent/internal/i18n"
)

var (
	jsonMode   = flag.Bool("json", false, "")
	writePid   = flag.Bool("pid", false, "")
	configFile = flag.String("config", "config.toml", "")
)

func init() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		_, _ = fmt.Fprintf(w, "%s\n\n", i18n.T("usage_header", nil))
		_, _ = fmt.Fprintf(w, "%s\n", i18n.T("usage_format", nil))
		_, _ = fmt.Fprintf(w, "\n%s\n", i18n.T("options_header", nil))

		_, _ = fmt.Fprintf(w, "  -json\n    \t%s\n", i18n.T("flag_json_desc", nil))
		_, _ = fmt.Fprintf(w, "  -pid\n    \t%s\n", i18n.T("flag_pid_desc", nil))
		_, _ = fmt.Fprintf(w, "  -config %s\n    \t%s\n", "filename", i18n.T("flag_config_desc", nil))
	}
}

func main() {
	if err := i18n.InitDefaultFS(); err != nil {
		// Can't use i18n.T here since i18n init failed
		log.Printf("Warning: Failed to initialize localization: %v", err)
	}

	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if *writePid {
		if err := daemon.WritePidFile(); err != nil {
			log.Printf(i18n.Tf("pid_file_error", nil), i18n.T("pid_file_error", map[string]interface{}{"Error": err}))
		}
	}

	ctx := context.Background()

	application := app.NewApp(*configFile, *jsonMode)
	if err := application.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println(i18n.T("app_terminated", map[string]interface{}{"Reason": "context cancellation"}))
		} else {
			log.Fatalf("%s", i18n.T("app_error", map[string]interface{}{"Error": err}))
		}
	}
}
