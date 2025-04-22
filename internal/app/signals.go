package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
)

func SetupTerminationHandler(ctx context.Context, cancel context.CancelFunc) {
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-termCh
		log.Println("Termination signal received, shutting down...")
		cancel()

		// Set a timeout for graceful shutdown
		go func() {
			time.Sleep(5 * time.Second)
			log.Println("Forced exit after timeout")
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
				log.Println("SIGHUP received, reloading configuration...")

				newConfig, err := config.Load(configFile)
				if err != nil {
					log.Printf("Failed to reload configuration: %v", err)
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
