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
	initialReadWait = 5 * time.Second
	bufferSize      = 32 * 1024 // Buffer size for data transfer (32KB)
)

type ProxyService struct {
	Service
	connectionMu sync.RWMutex
	config       config.Config
	cancelFunc   context.CancelFunc

	tunnelReadMu sync.Mutex

	connPool *ConnectionPool
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
			s.startProxyMonitoring(s.Context(), conn)
		}

	case event.ConnectionClosed:
		if s.cancelFunc != nil {
			s.cancelFunc()
			s.cancelFunc = nil
		}

		s.connPool.CleanupPool()
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
	conn, idx := s.connPool.GetConnection(s.GetConfig())

	defer func() {
		if conn != nil && idx >= 0 {
			s.connPool.ReleaseConnection(idx)
		}
	}()

	if conn == nil {
		return
	}

	_, err := conn.Write(requestData)
	if err != nil {
		s.connPool.RemoveConnection(idx)
		return
	}

	respBuffer := make([]byte, bufferSize)

	if err := conn.SetReadDeadline(time.Now().Add(initialReadWait)); err != nil {
		s.connPool.RemoveConnection(idx)
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
					s.connPool.RemoveConnection(idx)
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
					s.connPool.RemoveConnection(idx)
					return
				}

				s.connPool.RemoveConnection(idx)
				return
			}
		}
	}
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
