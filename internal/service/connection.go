package service

import (
	"context"
	"fmt"
	"log"
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
	Service
	jsonMode bool

	connectionMu sync.RWMutex
	connection   transport.Connection
	config       config.Config
}

func NewConnectionService(jsonMode bool, bus *event.Bus) *ConnectionService {
	return &ConnectionService{
		Service:  NewService("connection", bus),
		jsonMode: jsonMode,
	}
}

func (s *ConnectionService) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	unsub := s.bus.Subscribe(event.ConfigUpdated, s.handleConfigEvent)
	s.AddSubscription(unsub)

	return nil
}

func (s *ConnectionService) Stop(ctx context.Context) error {
	_ = s.closeConnection(ctx)

	return s.Service.Stop(ctx)
}

func (s *ConnectionService) IsJSONMode() bool {
	return s.jsonMode
}

func (s *ConnectionService) GetConfig() config.Config {
	s.connectionMu.RLock()
	defer s.connectionMu.RUnlock()
	return s.config
}

func (s *ConnectionService) SetConfig(cfg config.Config) {
	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()
	s.config = cfg
}

func (s *ConnectionService) Connect(ctx context.Context, cfg config.Config) error {
	conn, err := transport.Connect(ctx, cfg,
		transport.WithProtocol(transport.DefaultProtocol),
		transport.WithHost(cfg.Connection.Host),
		transport.WithPort(cfg.Connection.Port),
		transport.WithUser(cfg.Connection.User),
		transport.WithKeyFile(cfg.Connection.KeyFile),
		transport.WithTimeout(cfg.Connection.Timeout),
	)
	if err != nil {
		s.bus.Publish(event.Event{Type: event.ConnectionFailed, Data: err, Ctx: ctx})
		return util.WrapError(i18n.T("connection_error", nil), err)
	}

	if err := s.sendConfig(conn, cfg); err != nil {
		_ = conn.Close()
		return err
	}

	s.connectionMu.Lock()
	s.config = cfg
	s.connection = conn
	s.connectionMu.Unlock()

	s.bus.Publish(event.Event{Type: event.ConnectionEstablished, Ctx: ctx})
	s.wg.Add(1)
	go func(ctx context.Context) {
		defer s.wg.Done()
		s.runConnectionLoop(ctx)
	}(ctx)

	return nil
}

func (s *ConnectionService) HasConnection() bool {
	s.connectionMu.RLock()
	defer s.connectionMu.RUnlock()
	return s.connection != nil
}

func (s *ConnectionService) handleConfigEvent(evt event.Event) {
	if evt.Type != event.ConfigUpdated {
		return
	}

	if cfg, ok := evt.Data.(config.Config); ok {
		s.SetConfig(cfg)

		s.connectionMu.RLock()
		conn := s.connection
		s.connectionMu.RUnlock()

		needsReconnect := true
		if conn != nil {
			if err := s.sendConfig(conn, cfg); err == nil {
				needsReconnect = false
			}
		}

		if needsReconnect {
			s.bus.Publish(event.Event{Type: event.ReconnectRequested, Ctx: s.Context()})
		}
	}
}

func (s *ConnectionService) handleReconnectEvent(reconnectCh chan struct{}, cancelFunc context.CancelFunc) func(evt event.Event) {
	return func(evt event.Event) {
		if evt.Type != event.ReconnectRequested {
			return
		}
		select {
		case reconnectCh <- struct{}{}:
		default:
		}
		cancelFunc()
	}
}

func (s *ConnectionService) closeConnection(ctx context.Context) error {
	s.connectionMu.Lock()
	conn := s.connection
	s.connection = nil
	s.connectionMu.Unlock()

	if conn == nil {
		return nil
	}

	_ = conn.Close()
	s.bus.Publish(event.Event{Type: event.ConnectionClosed, Ctx: ctx})

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
		return util.NewError(util.ErrTypeConnection, i18n.T("config_send_error", map[string]any{"Error": err}), err)
	}
	return nil
}

func (s *ConnectionService) runConnectionLoop(ctx context.Context) {
	s.connectionMu.RLock()
	conn := s.connection
	jsonMode := s.jsonMode
	s.connectionMu.RUnlock()

	if conn == nil {
		return
	}

	loopCtx, cancelLoop := context.WithCancel(ctx)
	defer cancelLoop()

	handler := s.runStandardMode
	if jsonMode {
		handler = s.runJSONMode
	}

	var err error
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		err = handler(loopCtx, conn)
	}()

	select {
	case <-handlerDone:
		if err != nil && !util.IsExpectedError(err) {
			if util.IsConnectionError(err) {
				err = util.WrapError(i18n.T("connection_lost", nil), err)
			}
			s.bus.Publish(event.Event{Type: event.ConnectionFailed, Data: err, Ctx: ctx})
		}
	case <-ctx.Done():
		cancelLoop()
		<-handlerDone // Wait for handler to respond to cancellation
	}

	_ = s.closeConnection(ctx)
}

func (s *ConnectionService) runJSONMode(ctx context.Context, conn transport.Connection) error {
	errCh := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- util.NewError(util.ErrTypeConnection, fmt.Sprintf("panic in JSON mode handler: %v", r), nil)
			}
		}()
		errCh <- protocol.HandleJSONMode(ctx, conn, os.Stdin, os.Stdout)
	}()

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()

	heartbeatErr := make(chan error, 1)
	go func() {
		heartbeatErr <- protocol.RunHeartbeat(heartbeatCtx, conn, 30*time.Second)
	}()

	// Wait for either protocol handler or heartbeat to complete
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err != nil && !util.IsExpectedError(err) {
			return util.WrapError(i18n.T("json_mode_error", nil), err)
		}
		return err
	case err := <-heartbeatErr:
		if err != nil && !util.IsExpectedError(err) {
			log.Printf("%s", i18n.T("heartbeat_error", map[string]any{"Error": err}))
		}
		return err
	}
}

func (s *ConnectionService) runStandardMode(ctx context.Context, conn transport.Connection) error {
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	reconnectPassthrough := make(chan struct{}, 1)
	unsub := s.bus.Subscribe(event.ReconnectRequested, s.handleReconnectEvent(reconnectPassthrough, cancelRun))
	defer unsub()

	err := protocol.RunMainLoop(runCtx, conn, reconnectPassthrough)

	if err != nil && !util.IsExpectedError(err) {
		return util.WrapError(i18n.T("main_loop_error", map[string]any{"Error": err}), err)
	}

	return nil
}
