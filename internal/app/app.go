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
	"erlang-solutions.com/cortex_agent/internal/i18n"
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
		msg := i18n.T("config_load_error", map[string]interface{}{"Error": err})
		return fmt.Errorf("%s", msg)
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
			log.Println(i18n.T("connection_establishing", nil))

			connErr := a.runConnection(ctx, cfg, reconnectCh)

			select {
			case <-ctx.Done():
				log.Println(i18n.T("app_terminating", nil))
				return ctx.Err()
			case <-reconnectCh:
				log.Println(i18n.T("reconnecting", nil))
				continue
			default:
				if connErr != nil {
					msg := i18n.T("connection_error_terminating", map[string]interface{}{"Error": connErr})
					log.Printf("%s", msg)
					return connErr
				}
				log.Println(i18n.T("connection_normal_termination", nil))
				return nil
			}
		}
	}
}

func (a *App) runConnection(ctx context.Context, cfg config.Config, reconnectCh <-chan struct{}) error {
	conn, err := ssh.Connect(ctx, cfg)
	if err != nil {
		msg := i18n.T("connection_error", map[string]interface{}{"Error": err})
		return fmt.Errorf("%s", msg)
	}

	defer func() {
		log.Println(i18n.T("connection_closing", nil))
		if closeErr := conn.Close(); closeErr != nil {
			msg := i18n.T("connection_close_error", map[string]interface{}{"Error": closeErr})
			log.Printf("%s", msg)
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
		msg := i18n.T("config_send_error", map[string]interface{}{"Error": err})
		return fmt.Errorf("%s", msg)
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
				msg := i18n.T("json_mode_error", map[string]interface{}{"Error": err})
				return fmt.Errorf("%s", msg)
			}
			return err
		case <-heartbeatTicker.C:
			err := conn.SendPayload(map[string]string{"type": "heartbeat"})
			if err != nil {
				msg := i18n.T("heartbeat_error", map[string]interface{}{"Error": err})
				log.Printf("%s", msg)
			}
		}
	}
}

func (a *App) runStandardMode(ctx context.Context, conn ssh.Connection, reconnectCh <-chan struct{}) error {
	err := protocol.RunMainLoop(ctx, conn, reconnectCh)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		msg := i18n.T("main_loop_error", map[string]interface{}{"Error": err})
		return fmt.Errorf("%s", msg)
	}
	return nil
}
