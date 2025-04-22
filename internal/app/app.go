package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/protocol"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

type App struct {
	ConfigFile string
	JSONMode   bool
}

func New(configFile string, jsonMode bool) *App {
	return &App{
		ConfigFile: configFile,
		JSONMode:   jsonMode,
	}
}

func (a *App) Run(ctx context.Context) error {
	cfg, err := config.Load(a.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	reconnectCh := make(chan struct{}, 1)
	SetupConfigReloader(ctx, a.ConfigFile, &cfg, reconnectCh)

	// In JSON mode, we run once and exit
	if a.JSONMode {
		return a.runConnection(ctx, cfg, reconnectCh)
	}

	// In standard mode, we keep reconnecting
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			log.Println("Establishing SSH connection...")

			connErr := a.runConnection(ctx, cfg, reconnectCh)

			select {
			case <-ctx.Done():
				log.Println("Application terminating due to context cancellation")
				return ctx.Err()
			case <-reconnectCh:
				log.Println("Reconnecting due to configuration change...")
				continue
			default:
				if connErr != nil {
					log.Printf("Connection terminated with error: %v", connErr)
					return connErr
				}
				log.Println("Connection terminated normally, exiting")
				return nil
			}
		}
	}
}

func (a *App) runConnection(ctx context.Context, cfg config.Config, reconnectCh <-chan struct{}) error {
	conn, err := ssh.Connect(ctx, cfg)
	if err != nil {
		return fmt.Errorf("SSH connection error: %w", err)
	}

	defer func() {
		log.Println("Closing SSH connection...")
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Error closing SSH connection: %v", closeErr)
		}
	}()

	if err := a.sendInitialConfig(conn, cfg); err != nil {
		return err
	}

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var protocolErr error
	if a.JSONMode {
		protocolErr = a.runJSONMode(connCtx, conn)
	} else {
		protocolErr = a.runStandardMode(connCtx, conn, reconnectCh)
	}

	if protocolErr != nil && !errors.Is(protocolErr, context.Canceled) {
		return protocolErr
	}

	return nil
}

func (a *App) sendInitialConfig(conn ssh.Connection, cfg config.Config) error {
	payload := ssh.ConfigPayload{
		Application: ssh.ApplicationConfig{
			Hostname: cfg.Application.Hostname,
			Port:     cfg.Application.Port,
		},
		Agent: ssh.AgentConfig{
			Tags: cfg.Agent.Tags,
		},
	}

	if err := conn.SendPayload(payload); err != nil {
		return fmt.Errorf("failed to send configuration: %w", err)
	}
	return nil
}

func (a *App) runJSONMode(ctx context.Context, conn ssh.Connection) error {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	errCh := make(chan error, 1)
	go func() {
		err := protocol.HandleJSONMode(ctx, conn, os.Stdin, os.Stdout)
		errCh <- err
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("JSON mode error: %w", err)
			}
			return err
		case <-heartbeatTicker.C:
			err := conn.SendPayload(map[string]string{"type": "heartbeat"})
			if err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
		}
	}
}

func (a *App) runStandardMode(ctx context.Context, conn ssh.Connection, reconnectCh <-chan struct{}) error {
	err := protocol.RunMainLoop(ctx, conn, reconnectCh)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("main loop error: %w", err)
	}
	return nil
}
