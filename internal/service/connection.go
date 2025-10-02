package service

import (
	"bufio"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hibeam-dev/amaru_agent/internal/config"
	"github.com/hibeam-dev/amaru_agent/internal/event"
	"github.com/hibeam-dev/amaru_agent/internal/i18n"
	"github.com/hibeam-dev/amaru_agent/internal/protocol"
	"github.com/hibeam-dev/amaru_agent/internal/transport"
	"github.com/hibeam-dev/amaru_agent/internal/util"
)

type ConnectionService struct {
	Service
	connectionMu      sync.RWMutex
	connection        transport.Connection
	config            config.Config
	monitorCancelFunc context.CancelFunc
}

func NewConnectionService(bus *event.Bus) *ConnectionService {
	svc := &ConnectionService{}
	svc.Service = NewService("connection", bus)
	return svc
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
	if s.monitorCancelFunc != nil {
		s.monitorCancelFunc()
		s.monitorCancelFunc = nil
	}

	_ = s.closeConnection(ctx)

	return s.Service.Stop(ctx)
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
		transport.WithKeyFile(cfg.Connection.KeyFile),
		transport.WithTimeout(cfg.Connection.Timeout),
	)
	if err != nil {
		s.bus.Publish(event.Event{Type: event.ConnectionFailed, Data: err, Ctx: ctx})
		return util.WrapError(i18n.T("connection_error", map[string]any{"Error": err}), err)
	}

	if err := s.sendConfig(conn, cfg); err != nil {
		_ = conn.Close()
		return err
	}

	if s.monitorCancelFunc != nil {
		s.monitorCancelFunc()
		s.monitorCancelFunc = nil
	}

	s.connectionMu.Lock()
	s.config = cfg
	s.connection = conn
	s.connectionMu.Unlock()

	s.bus.Publish(event.Event{Type: event.ConnectionEstablished, Data: conn, Ctx: ctx})

	s.wg.Add(1)
	go func(ctx context.Context) {
		defer s.wg.Done()
		s.runConnectionLoop(ctx)
	}(ctx)

	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	s.monitorCancelFunc = cancelMonitor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.monitorConnection(monitorCtx)
	}()

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
	}{
		Type: "config_update",
		Application: transport.ApplicationConfig{
			Hostname: cfg.Application.Hostname,
			Port:     cfg.Application.Port,
			IP:       cfg.Application.IP,
			Tags:     cfg.Application.Tags,
			Security: cfg.Application.Security,
		},
	}

	if err := conn.SendPayload(payload); err != nil {
		return util.NewError(util.ErrTypeConnection, i18n.T("config_send_error", map[string]any{"Error": err}), err)
	}

	wgConfig, err := s.receiveWireGuardConfigWithTimeout(conn, 60*time.Second)
	if err != nil {
		return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_error", map[string]any{"Error": err}), err)
	}

	util.Debug(i18n.T("wireguard_config_parsed", map[string]any{
		"type":      "connection",
		"Endpoint":  wgConfig.Endpoint,
		"IP":        wgConfig.TunnelIP,
		"PublicKey": wgConfig.PublicKey[:16] + "...",
	}), map[string]any{"component": "connection"})

	// Store WireGuard config for proxy service
	s.bus.Publish(event.Event{
		Type: event.WireGuardConfigReceived,
		Data: wgConfig,
		Ctx:  s.Context(),
	})

	return nil
}

func (s *ConnectionService) receiveWireGuardConfig(conn transport.Connection) (*WireGuardConfig, error) {
	scanner := bufio.NewScanner(conn.Stdout())
	scanner.Scan()

	if err := scanner.Err(); err != nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_read_error", map[string]any{"Error": err}), err)
	}

	line := scanner.Text()
	if line == "" {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_empty", map[string]any{}), nil)
	}

	var response struct {
		Type      string           `json:"type"`
		WireGuard *WireGuardConfig `json:"wireguard,omitempty"`
		Error     string           `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(line), &response); err != nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_parse_error", map[string]any{"Error": err}), err)
	}

	if response.Error != "" {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_server_error", map[string]any{"Error": response.Error}), nil)
	}

	if response.Type != "wireguard_config" || response.WireGuard == nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_invalid_response", map[string]any{}), nil)
	}

	return response.WireGuard, nil
}

func (s *ConnectionService) receiveWireGuardConfigWithTimeout(conn transport.Connection, timeout time.Duration) (*WireGuardConfig, error) {
	type result struct {
		config *WireGuardConfig
		err    error
	}

	resultCh := make(chan result, 1)

	go func() {
		config, err := s.receiveWireGuardConfig(conn)
		resultCh <- result{config: config, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case res := <-resultCh:
		util.Debug(i18n.T("wireguard_config_received_in_time", map[string]any{
			"type": "connection",
		}), map[string]any{"component": "connection"})
		return res.config, res.err
	case <-timer.C:
		util.Debug(i18n.T("wireguard_config_timeout", map[string]any{
			"type":    "connection",
			"Timeout": timeout.String(),
		}), map[string]any{"component": "connection"})
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_timeout_error", map[string]any{"Timeout": timeout}), nil)
	}
}

func (s *ConnectionService) runConnectionLoop(ctx context.Context) {
	s.connectionMu.RLock()
	conn := s.connection
	s.connectionMu.RUnlock()

	if conn == nil {
		return
	}

	loopCtx, cancelLoop := context.WithCancel(ctx)
	defer cancelLoop()

	var err error
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		err = s.runProtocolMode(loopCtx, conn)
	}()

	select {
	case <-handlerDone:
		if err != nil && !util.IsExpectedError(err) {
			if util.IsConnectionError(err) {
				err = util.WrapError(i18n.T("connection_lost", map[string]any{"Error": err}), err)
			}
			s.bus.Publish(event.Event{Type: event.ConnectionFailed, Data: err, Ctx: ctx})
		}
	case <-ctx.Done():
		cancelLoop()
		<-handlerDone // Wait for handler to respond to cancellation
	}

	_ = s.closeConnection(ctx)
}

func (s *ConnectionService) runProtocolMode(ctx context.Context, conn transport.Connection) error {
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

func (s *ConnectionService) monitorConnection(ctx context.Context) {
	const (
		healthCheckInterval = 5 * time.Second
		minReconnectDelay   = 1 * time.Second
		maxReconnectDelay   = 30 * time.Second
	)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	var reconnectDelay = minReconnectDelay
	var lastReconnectAttempt time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.connectionMu.RLock()
			haveConnection := s.connection != nil
			config := s.config
			s.connectionMu.RUnlock()

			if !haveConnection {
				now := time.Now()
				if now.Sub(lastReconnectAttempt) < reconnectDelay {
					continue
				}

				lastReconnectAttempt = now

				util.Info(i18n.T("attempting_reconnection", map[string]any{
					"type": "connection",
					"Host": config.Connection.Host,
					"Port": config.Connection.Port,
				}), map[string]any{"component": "connection"})

				if err := s.Connect(ctx, config); err != nil {
					reconnectDelay *= 2
					if reconnectDelay > maxReconnectDelay {
						reconnectDelay = maxReconnectDelay
					}

					util.LogError(i18n.T("reconnection_failed", map[string]any{
						"type":  "connection",
						"Error": err,
						"Delay": reconnectDelay.String(),
					}), err, map[string]any{"component": "connection"})
				} else {
					reconnectDelay = minReconnectDelay
					util.Info(i18n.T("reconnection_successful", map[string]any{
						"type": "connection",
					}), map[string]any{"component": "connection"})
				}
				continue
			}

			s.connectionMu.RLock()
			conn := s.connection
			s.connectionMu.RUnlock()

			checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := conn.CheckHealth(checkCtx)
			cancel()

			if err != nil {
				util.LogError(i18n.T("connection_health_check_failed", map[string]any{
					"type":  "connection",
					"Error": err,
				}), err, map[string]any{"component": "connection"})

				_ = s.closeConnection(ctx)

				lastReconnectAttempt = time.Now()
				reconnectDelay = minReconnectDelay

				if err := s.Connect(ctx, config); err != nil {
					util.LogError(i18n.T("immediate_reconnect_failed", map[string]any{
						"type":  "connection",
						"Error": err,
					}), err, map[string]any{"component": "connection"})
				} else {
					util.Info(i18n.T("immediate_reconnect_successful", map[string]any{
						"type": "connection",
					}), map[string]any{"component": "connection"})
				}
			}
		}
	}
}
