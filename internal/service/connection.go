package service

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

type ConnectionService struct {
	jsonMode bool
}

func NewConnectionService(jsonMode bool) *ConnectionService {
	return &ConnectionService{
		jsonMode: jsonMode,
	}
}

func (s *ConnectionService) IsJSONMode() bool {
	return s.jsonMode
}

func (s *ConnectionService) Connect(ctx context.Context, cfg config.Config) (ssh.Connection, error) {
	conn, err := ssh.Connect(ctx, cfg)
	if err != nil {
		msg := i18n.T("connection_error", map[string]interface{}{"Error": err})
		return nil, fmt.Errorf("%s", msg)
	}
	return conn, nil
}

func (s *ConnectionService) SendInitialConfig(conn ssh.Connection, cfg config.Config) error {
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

func (s *ConnectionService) RunJSONMode(ctx context.Context, conn ssh.Connection) error {
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

func (s *ConnectionService) RunStandardMode(ctx context.Context, conn ssh.Connection, reconnectCh <-chan struct{}) error {
	err := protocol.RunMainLoop(ctx, conn, reconnectCh)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		msg := i18n.T("main_loop_error", map[string]interface{}{"Error": err})
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func (s *ConnectionService) Run(ctx context.Context, conn ssh.Connection, reconnectCh <-chan struct{}) error {
	if s.jsonMode {
		return s.RunJSONMode(ctx, conn)
	}
	return s.RunStandardMode(ctx, conn, reconnectCh)
}
