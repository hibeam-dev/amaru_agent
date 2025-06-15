package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/event"
	"erlang-solutions.com/amaru_agent/internal/transport"
)

type ProxyService struct {
	Service
	connectionMu   sync.RWMutex
	config         config.Config
	cancelFunc     context.CancelFunc
	backendConns   []net.Conn
	backendConnsMu sync.Mutex
	connPool       *ConnectionPool
}

func NewProxyService(bus *event.Bus) *ProxyService {
	svc := &ProxyService{}
	svc.Service = NewService("proxy", bus)
	svc.connPool = NewConnectionPool(svc.createLocalConnection)
	return svc
}

func (s *ProxyService) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	unsub := s.bus.Subscribe(event.ConfigUpdated, s.handleConfigEvent)
	s.AddSubscription(unsub)

	connSub := s.bus.SubscribeMultiple(
		[]string{event.ConnectionEstablished, event.ConnectionClosed},
		s.handleConnectionEvents,
	)
	for _, sub := range connSub {
		s.AddSubscription(sub)
	}

	return nil
}

func (s *ProxyService) Stop(ctx context.Context) error {
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}

	s.closeBackendConnections()
	s.connPool.CleanupPool()

	return s.Service.Stop(ctx)
}

func (s *ProxyService) SetConfig(cfg config.Config) {
	s.connectionMu.Lock()
	oldCfg := s.config
	s.config = cfg
	s.connectionMu.Unlock()

	if oldCfg.Application.Port != cfg.Application.Port ||
		oldCfg.Application.IP != cfg.Application.IP {
		s.connPool.CleanupPool()
	}
}

func (s *ProxyService) GetConfig() config.Config {
	s.connectionMu.RLock()
	defer s.connectionMu.RUnlock()
	return s.config
}

func (s *ProxyService) handleConfigEvent(evt event.Event) {
	if evt.Type != event.ConfigUpdated {
		return
	}

	if cfg, ok := evt.Data.(config.Config); ok {
		s.SetConfig(cfg)
	}
}

func (s *ProxyService) handleConnectionEvents(evt event.Event) {
	switch evt.Type {
	case event.ConnectionEstablished:
		if conn, ok := evt.Data.(transport.Connection); ok {
			s.startTunnelConnections(s.Context(), conn)
		}

	case event.ConnectionClosed:
		if s.cancelFunc != nil {
			s.cancelFunc()
			s.cancelFunc = nil
		}

		s.closeBackendConnections()
		s.connPool.CleanupPool()
	}
}

func (s *ProxyService) startTunnelConnections(ctx context.Context, conn transport.Connection) {
	cfg := s.GetConfig()
	if !cfg.Connection.Tunnel {
		return
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	monitorCtx, cancelFunc := context.WithCancel(ctx)
	s.cancelFunc = cancelFunc

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.createBackendConnections(monitorCtx)
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.poolMaintenance(monitorCtx)
	}()
}

func (s *ProxyService) poolMaintenance(ctx context.Context) {
	s.connPool.CleanIdleConnections()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.connPool.CleanIdleConnections()
		}
	}
}

func (s *ProxyService) createBackendConnections(ctx context.Context) {
	s.backendConnsMu.Lock()
	s.backendConns = make([]net.Conn, 0, poolSize)

	for range poolSize {
		conn, err := s.createBackendConnection()
		if err != nil {
			s.bus.Publish(event.Event{
				Type: event.ConnectionFailed,
				Data: err,
				Ctx:  ctx,
			})
			continue
		}
		s.backendConns = append(s.backendConns, conn)
	}
	s.backendConnsMu.Unlock()

	maintenanceTicker := time.NewTicker(10 * time.Second)
	keepAliveTicker := time.NewTicker(30 * time.Second)
	defer maintenanceTicker.Stop()
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-maintenanceTicker.C:
			s.maintainBackendConnections(ctx)
		case <-keepAliveTicker.C:
			s.keepAliveBackendConnections()
		}
	}
}

func (s *ProxyService) createBackendConnection() (net.Conn, error) {
	cfg := s.GetConfig()
	backendAddr := net.JoinHostPort(cfg.Connection.Host, "9090")

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tlsConn, err := tls.DialWithDialer(dialer, "tcp", backendAddr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := tlsConn.NetConn().(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	return tlsConn, nil
}

func (s *ProxyService) keepAliveBackendConnections() {
	s.backendConnsMu.Lock()
	defer s.backendConnsMu.Unlock()

	for _, conn := range s.backendConns {
		if conn != nil {
			_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			_, _ = conn.Write([]byte{0})
			_ = conn.SetWriteDeadline(time.Time{})
		}
	}
}

func (s *ProxyService) maintainBackendConnections(ctx context.Context) {
	s.backendConnsMu.Lock()
	defer s.backendConnsMu.Unlock()

	for i, conn := range s.backendConns {
		if conn == nil || s.isConnectionBroken(conn) {
			if conn != nil {
				_ = conn.Close()
			}
			newConn, err := s.createBackendConnection()
			if err == nil {
				s.backendConns[i] = newConn
			} else {
				s.backendConns[i] = nil
			}
		}
	}
}

func (s *ProxyService) isConnectionBroken(conn net.Conn) bool {
	if conn == nil {
		return true
	}

	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	buffer := make([]byte, 1)
	_, err := conn.Read(buffer)

	if err == nil {
		return false
	}

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return false
	}

	return true
}

func (s *ProxyService) closeBackendConnections() {
	s.backendConnsMu.Lock()
	defer s.backendConnsMu.Unlock()

	for _, conn := range s.backendConns {
		if conn != nil {
			_ = conn.Close()
		}
	}
	s.backendConns = nil
}

func (s *ProxyService) createLocalConnection(cfg config.Config) (net.Conn, error) {
	useTLS := false
	if val, ok := cfg.Application.Security["tls"]; ok {
		useTLS = val
	}

	ip := "127.0.0.1"
	if cfg.Application.IP != "" {
		ip = cfg.Application.IP
	}

	localAddr := net.JoinHostPort(ip, fmt.Sprintf("%d", cfg.Application.Port))

	dialer := &net.Dialer{
		Timeout: connTimeout,
	}

	if useTLS {
		return tls.DialWithDialer(dialer, "tcp", localAddr, &tls.Config{
			InsecureSkipVerify: true, // This is a local connection
		})
	}
	return dialer.Dial("tcp", localAddr)
}
