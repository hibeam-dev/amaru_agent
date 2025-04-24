package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/protocol"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

type ConnectionService struct {
	BaseService
	jsonMode bool

	mu     sync.Mutex
	conn   ssh.Connection
	config config.Config
}

func NewConnectionService(jsonMode bool, bus *EventBus) *ConnectionService {
	return &ConnectionService{
		BaseService: NewBaseService("connection", bus),
		jsonMode:    jsonMode,
	}
}

func (s *ConnectionService) Start(ctx context.Context) error {
	if err := s.BaseService.Start(ctx); err != nil {
		return err
	}

	configCh := s.bus.Subscribe(ConfigUpdated, 1)
	s.Go(func() {
		s.handleEvents(configCh)
	})

	return nil
}

func (s *ConnectionService) IsJSONMode() bool {
	return s.jsonMode
}

func (s *ConnectionService) SetConfig(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
}

func (s *ConnectionService) GetConfig() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

func (s *ConnectionService) Connect(ctx context.Context, cfg config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg

	conn, err := ssh.Connect(ctx, cfg)
	if err != nil {
		s.bus.Publish(Event{Type: ConnectionFailed, Data: err})
		return fmt.Errorf("%s", i18n.T("connection_error", map[string]interface{}{"Error": err}))
	}

	s.conn = conn

	if err := s.sendConfig(conn, cfg); err != nil {
		_ = s.conn.Close()
		s.conn = nil
		return err
	}

	s.bus.Publish(Event{Type: ConnectionEstablished})
	s.Go(func() {
		s.runConnectionLoop(ctx)
	})

	return nil
}

func (s *ConnectionService) HasConnection() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

func (s *ConnectionService) closeConnection() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil
	}

	_ = s.conn.Close()
	s.conn = nil
	s.bus.Publish(Event{Type: ConnectionClosed})

	return nil
}

func (s *ConnectionService) sendConfig(conn ssh.Connection, cfg config.Config) error {
	payload := struct {
		Type        string                `json:"type"`
		Application ssh.ApplicationConfig `json:"application"`
		Agent       ssh.AgentConfig       `json:"agent"`
	}{
		Type: "config_update",
		Application: ssh.ApplicationConfig{
			Hostname: cfg.Application.Hostname,
			Port:     cfg.Application.Port,
		},
		Agent: ssh.AgentConfig{
			Tags: cfg.Agent.Tags,
		},
	}

	if err := conn.SendPayload(payload); err != nil {
		return fmt.Errorf("%s", i18n.T("config_send_error", map[string]interface{}{"Error": err}))
	}
	return nil
}

func (s *ConnectionService) handleEvents(configCh <-chan Event) {
	for {
		select {
		case <-s.Context().Done():
			return

		case event, ok := <-configCh:
			if !ok {
				return
			}

			if event.Type == ConfigUpdated {
				if cfg, ok := event.Data.(config.Config); ok {
					s.SetConfig(cfg)

					s.mu.Lock()
					conn := s.conn
					s.mu.Unlock()

					if conn != nil {
						if err := s.sendConfig(conn, cfg); err != nil {
							s.bus.Publish(Event{Type: ReconnectRequested})
						}
					} else {
						s.bus.Publish(Event{Type: ReconnectRequested})
					}
				}
			}
		}
	}
}

func (s *ConnectionService) runConnectionLoop(ctx context.Context) {
	s.mu.Lock()
	conn := s.conn
	jsonMode := s.jsonMode
	s.mu.Unlock()

	if conn == nil {
		return
	}

	var err error
	if jsonMode {
		err = s.runJSONMode(ctx, conn)
	} else {
		err = s.runStandardMode(ctx, conn)
	}

	if err != nil && !isKnownGoodError(err) {
		s.bus.Publish(Event{Type: ConnectionFailed, Data: err})
	}

	_ = s.closeConnection()
}

func (s *ConnectionService) runJSONMode(ctx context.Context, conn ssh.Connection) error {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- protocol.HandleJSONMode(ctx, conn, os.Stdin, os.Stdout)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && !isKnownGoodError(err) {
				return fmt.Errorf("%s", i18n.T("json_mode_error", map[string]interface{}{"Error": err}))
			}
			return err
		case <-heartbeatTicker.C:
			_ = conn.SendPayload(map[string]string{"type": "heartbeat"})
		}
	}
}

func (s *ConnectionService) runStandardMode(ctx context.Context, conn ssh.Connection) error {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	reconnectCh := s.bus.Subscribe(ReconnectRequested, 1)
	reconnectPassthrough := make(chan struct{}, 1)

	go func() {
		select {
		case <-runCtx.Done():
			return
		case <-reconnectCh:
			select {
			case reconnectPassthrough <- struct{}{}:
			default:
			}
			cancelRun()
		}
	}()

	err := protocol.RunMainLoop(runCtx, conn, reconnectPassthrough)

	if err != nil && !isKnownGoodError(err) {
		return fmt.Errorf("%s", i18n.T("main_loop_error", map[string]interface{}{"Error": err}))
	}

	return nil
}

func isKnownGoodError(err error) bool {
	return err == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF)
}
