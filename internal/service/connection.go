package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/protocol"
	"erlang-solutions.com/cortex_agent/internal/transport"
	"erlang-solutions.com/cortex_agent/internal/util"
)

type ConnectionService struct {
	jsonMode bool
	name     string
	bus      *event.Bus
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	mu           sync.Mutex
	conn         transport.Connection
	config       config.Config
	unsubscribes []func()
}

func NewConnectionService(jsonMode bool, bus *event.Bus) *ConnectionService {
	return &ConnectionService{
		name:         "connection",
		bus:          bus,
		jsonMode:     jsonMode,
		unsubscribes: make([]func(), 0),
	}
}

func (s *ConnectionService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	unsub := s.bus.Subscribe(event.ConfigUpdated, s.handleConfigEvent)
	s.mu.Lock()
	s.unsubscribes = append(s.unsubscribes, unsub)
	s.mu.Unlock()

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
	conn, err := transport.NewConnection(ctx, cfg)
	if err != nil {
		s.bus.Publish(event.Event{Type: event.ConnectionFailed, Data: err})
		return fmt.Errorf("%s", i18n.T("connection_error", map[string]interface{}{"Error": err}))
	}

	if err := s.sendConfig(conn, cfg); err != nil {
		_ = conn.Close()
		return err
	}

	s.mu.Lock()
	s.config = cfg
	s.conn = conn
	s.mu.Unlock()

	s.bus.Publish(event.Event{Type: event.ConnectionEstablished})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runConnectionLoop(ctx)
	}()

	return nil
}

func (s *ConnectionService) HasConnection() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

func (s *ConnectionService) closeConnection() error {
	s.mu.Lock()
	conn := s.conn
	s.conn = nil
	s.mu.Unlock()

	if conn == nil {
		return nil
	}

	_ = conn.Close()
	s.bus.Publish(event.Event{Type: event.ConnectionClosed})

	return nil
}

func (s *ConnectionService) sendConfig(conn transport.Connection, cfg config.Config) error {
	payload := struct {
		Type        string                      `json:"type"`
		Application transport.ApplicationConfig `json:"application"`
		Agent       transport.AgentConfig       `json:"agent"`
	}{
		Type: "config_update",
		Application: transport.ApplicationConfig{
			Hostname: cfg.Application.Hostname,
			Port:     cfg.Application.Port,
		},
		Agent: transport.AgentConfig{
			Tags: cfg.Agent.Tags,
		},
	}

	if err := conn.SendPayload(payload); err != nil {
		return fmt.Errorf("%s", i18n.T("config_send_error", map[string]interface{}{"Error": err}))
	}
	return nil
}

func (s *ConnectionService) Stop(ctx context.Context) error {
	s.mu.Lock()
	unsubscribes := s.unsubscribes
	s.unsubscribes = nil
	s.mu.Unlock()

	for _, unsub := range unsubscribes {
		if unsub != nil {
			unsub()
		}
	}

	if s.cancel != nil {
		s.cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		util.WaitWithTimeout(&s.wg, 500*time.Millisecond)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		err := ctx.Err()
		if util.IsExpectedError(err) {
			return nil
		}
		return err
	}
}

func (s *ConnectionService) handleConfigEvent(evt event.Event) {
	if evt.Type != event.ConfigUpdated {
		return
	}

	if cfg, ok := evt.Data.(config.Config); ok {
		s.SetConfig(cfg)

		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()

		if conn != nil {
			if err := s.sendConfig(conn, cfg); err != nil {
				s.bus.Publish(event.Event{Type: event.ReconnectRequested})
			}
		} else {
			s.bus.Publish(event.Event{Type: event.ReconnectRequested})
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

	handler := s.runStandardMode
	if jsonMode {
		handler = s.runJSONMode
	}

	if err := handler(ctx, conn); err != nil && !util.IsExpectedError(err) {
		s.bus.Publish(event.Event{Type: event.ConnectionFailed, Data: err})
	}

	_ = s.closeConnection()
}

func (s *ConnectionService) runJSONMode(ctx context.Context, conn transport.Connection) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- protocol.HandleJSONMode(ctx, conn, os.Stdin, os.Stdout)
	}()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil && !util.IsExpectedError(err) {
				return fmt.Errorf("%s", i18n.T("json_mode_error", map[string]interface{}{"Error": err}))
			}
			return err
		case <-heartbeatTicker.C:
			_ = conn.SendPayload(map[string]string{"type": "heartbeat"})
		}
	}
}

func (s *ConnectionService) runStandardMode(ctx context.Context, conn transport.Connection) error {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	reconnectPassthrough := make(chan struct{}, 1)
	unsub := s.bus.Subscribe(event.ReconnectRequested, func(evt event.Event) {
		select {
		case reconnectPassthrough <- struct{}{}:
		default:
		}
		cancelRun()
	})

	defer unsub()

	err := protocol.RunMainLoop(runCtx, conn, reconnectPassthrough)

	if err != nil && !util.IsExpectedError(err) {
		return fmt.Errorf("%s", i18n.T("main_loop_error", map[string]interface{}{"Error": err}))
	}

	return nil
}
