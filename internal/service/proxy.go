package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/transport"
)

const (
	poolSize        = 5
	connIdleTimeout = 60 * time.Second
	connTimeout     = 5 * time.Second
	initialReadWait = 5 * time.Second
	bufferSize      = 32 * 1024 // Buffer size for data transfer (32KB)
)

type ProxyService struct {
	Service
	connectionMu sync.RWMutex
	config       config.Config
	cancelFunc   context.CancelFunc

	tunnelReadMu sync.Mutex

	poolMu sync.Mutex
	pool   []*pooledConnection
}

type pooledConnection struct {
	conn     net.Conn
	inUse    bool
	lastUsed time.Time
}

func NewProxyService(bus *event.Bus) *ProxyService {
	svc := &ProxyService{
		pool: make([]*pooledConnection, poolSize),
	}
	svc.Service = NewService("proxy", bus)
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

	s.poolMu.Lock()
	for i, conn := range s.pool {
		if conn != nil && conn.conn != nil {
			_ = conn.conn.Close()
			s.pool[i] = nil
		}
	}
	s.poolMu.Unlock()

	return s.Service.Stop(ctx)
}

func (s *ProxyService) SetConfig(cfg config.Config) {
	s.connectionMu.Lock()
	oldCfg := s.config
	s.config = cfg
	s.connectionMu.Unlock()

	if oldCfg.Application.Port != cfg.Application.Port ||
		oldCfg.Application.IP != cfg.Application.IP {
		s.cleanupPool()
	}
}

func (s *ProxyService) GetConfig() config.Config {
	s.connectionMu.RLock()
	defer s.connectionMu.RUnlock()
	return s.config
}

func (s *ProxyService) cleanupPool() {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	for i, conn := range s.pool {
		if conn != nil && conn.conn != nil {
			_ = conn.conn.Close()
			s.pool[i] = nil
		}
	}
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
			s.startProxyMonitoring(s.Context(), conn)
		}

	case event.ConnectionClosed:
		if s.cancelFunc != nil {
			s.cancelFunc()
			s.cancelFunc = nil
		}

		s.cleanupPool()
	}
}

func (s *ProxyService) startProxyMonitoring(ctx context.Context, conn transport.Connection) {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	monitorCtx, cancelFunc := context.WithCancel(ctx)
	s.cancelFunc = cancelFunc

	binaryInput := conn.BinaryInput()
	binaryOutput := conn.BinaryOutput()
	if binaryInput == nil || binaryOutput == nil {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.proxyHandler(monitorCtx, conn)
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.poolMaintenance(monitorCtx)
	}()
}

func (s *ProxyService) poolMaintenance(ctx context.Context) {
	s.cleanIdleConnections()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanIdleConnections()
		}
	}
}

func (s *ProxyService) cleanIdleConnections() {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	now := time.Now()

	for i, pc := range s.pool {
		if pc == nil {
			continue
		}

		if pc.inUse {
			continue
		}

		if pc.conn == nil {
			if now.Sub(pc.lastUsed) > 10*time.Second {
				s.pool[i] = nil
			}
			continue
		}

		// This ensures we don't keep HTTP/1.1 persistent connections around too long
		if now.Sub(pc.lastUsed) > 5*time.Second {
			_ = pc.conn.Close()
			pc.conn = nil
		}
	}
}

func (s *ProxyService) proxyHandler(ctx context.Context, conn transport.Connection) {
	tunnelInput := conn.BinaryInput()
	tunnelOutput := conn.BinaryOutput()
	if tunnelInput == nil || tunnelOutput == nil {
		return
	}

	buffer := make([]byte, bufferSize)
	reqNum := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.tunnelReadMu.Lock()
			reqNum++
			currentReqNum := reqNum

			n, err := tunnelOutput.Read(buffer)
			if err != nil {
				s.tunnelReadMu.Unlock()
				if err != io.EOF {
					s.bus.Publish(event.Event{
						Type: event.ConnectionFailed,
						Data: err,
						Ctx:  ctx,
					})
				}
				return
			}

			if n > 0 {
				// This blocks the next Read until this request is completely processed
				s.handleRequest(ctx, buffer[:n], tunnelInput, currentReqNum)
			}

			// Allow the next read only after this request is fully processed
			s.tunnelReadMu.Unlock()
		}
	}
}

func (s *ProxyService) handleRequest(ctx context.Context, requestData []byte, tunnelInput io.Writer, reqNum int) {
	conn, idx := s.getPoolConnection()

	defer func() {
		if conn != nil {
			s.releasePoolConnection(idx)
		}
	}()

	if conn == nil {
		return
	}

	_, err := conn.Write(requestData)
	if err != nil {
		s.removePoolConnection(idx)
		return
	}

	respBuffer := make([]byte, bufferSize)

	if err := conn.SetReadDeadline(time.Now().Add(initialReadWait)); err != nil {
		s.removePoolConnection(idx)
		return
	}

	var receivedData bool
	var inactivityTimer *time.Timer
	inactivityTimeout := 500 * time.Millisecond

	inactivityTimer = time.AfterFunc(inactivityTimeout, func() {
		if receivedData {
			_ = conn.SetReadDeadline(time.Now().Add(1 * time.Microsecond))
		}
	})
	defer inactivityTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := conn.Read(respBuffer)

			if n > 0 {
				receivedData = true

				_, writeErr := tunnelInput.Write(respBuffer[:n])
				if writeErr != nil {
					return
				}

				// After getting data, extend the read deadline for streaming data
				if err := conn.SetReadDeadline(time.Now().Add(initialReadWait)); err != nil {
					s.removePoolConnection(idx)
					return
				}

				inactivityTimer.Reset(inactivityTimeout)
			}

			if err != nil {
				// If it's EOF, we're done with this request
				if err == io.EOF {
					return
				}

				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// If we got data and then a timeout, it's likely the end of the message
					if receivedData {
						return
					}

					// Initial timeout with no data means server didn't respond
					s.removePoolConnection(idx)
					return
				}

				s.removePoolConnection(idx)
				return
			}
		}
	}
}

func (s *ProxyService) getPoolConnection() (net.Conn, int) {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	for i, pc := range s.pool {
		if pc != nil && pc.conn == nil && !pc.inUse {
			cfg := s.GetConfig()
			conn, err := s.createLocalConnection(cfg)
			if err != nil {
				continue
			}

			pc.conn = conn
			pc.inUse = true
			pc.lastUsed = time.Now()
			return conn, i
		}
	}

	for i, pc := range s.pool {
		if pc != nil && pc.conn != nil && !pc.inUse {
			// We shouldn't hit this since we're closing connections after use
			if isConnAlive(pc.conn) {
				pc.inUse = true
				return pc.conn, i
			}

			_ = pc.conn.Close()
			pc.conn = nil
		}
	}

	for i, pc := range s.pool {
		if pc == nil {
			cfg := s.GetConfig()
			conn, err := s.createLocalConnection(cfg)
			if err != nil {
				return nil, -1
			}

			s.pool[i] = &pooledConnection{
				conn:     conn,
				inUse:    true,
				lastUsed: time.Now(),
			}
			return conn, i
		}
	}

	// Pool is full, create a one-off connection
	cfg := s.GetConfig()
	conn, err := s.createLocalConnection(cfg)
	if err != nil {
		return nil, -1
	}

	return conn, -1
}

func (s *ProxyService) releasePoolConnection(idx int) {
	if idx < 0 {
		return
	}

	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	if idx >= len(s.pool) || s.pool[idx] == nil {
		return
	}

	// Ensuring we don't reuse connections that might have buffered responses
	if s.pool[idx].conn != nil {
		_ = s.pool[idx].conn.Close()
		s.pool[idx].conn = nil
	}

	s.pool[idx].inUse = false
	s.pool[idx].lastUsed = time.Now()
}

func (s *ProxyService) removePoolConnection(idx int) {
	if idx < 0 {
		return
	}

	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	if idx >= len(s.pool) || s.pool[idx] == nil {
		return
	}

	if s.pool[idx].conn != nil {
		_ = s.pool[idx].conn.Close()
	}
	s.pool[idx] = nil
}

func isConnAlive(conn net.Conn) bool {
	if conn == nil {
		return false
	}

	// Try to set a very short deadline
	err := conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
	if err != nil {
		return false
	}

	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	_, err = conn.Write([]byte{})
	return err == nil
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
