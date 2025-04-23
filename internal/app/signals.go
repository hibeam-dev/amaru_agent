package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
)

func SetupTerminationHandler(ctx context.Context, cancel context.CancelFunc) {
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-termCh
		log.Println(i18n.T("termination_signal", nil))
		cancel()

		// Set a timeout for graceful shutdown
		go func() {
			time.Sleep(5 * time.Second)
			log.Println(i18n.T("forced_exit", nil))
			os.Exit(1)
		}()
	}()
}

func SetupConfigReloader(ctx context.Context, configFile string, cfg *config.Config, reconnectCh chan<- struct{}) {
	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sighupCh:
				log.Println(i18n.T("sighup_received", nil))

				newConfig, err := config.Load(configFile)
				if err != nil {
					log.Printf(i18n.Tf("config_reload_error", nil), i18n.T("config_reload_error", map[string]interface{}{"Error": err}))
					continue
				}

				*cfg = newConfig

				select {
				case reconnectCh <- struct{}{}:
				default:
					// Channel already has a pending reconnect signal
				}
			}
		}
	}()
}
