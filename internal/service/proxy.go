package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/hibeam-dev/amaru_agent/internal/config"
	"github.com/hibeam-dev/amaru_agent/internal/event"
	"github.com/hibeam-dev/amaru_agent/internal/i18n"
	"github.com/hibeam-dev/amaru_agent/internal/transport"
	"github.com/hibeam-dev/amaru_agent/internal/util"
)

type ProxyService struct {
	Service
	connectionMu sync.RWMutex
	config       config.Config
	cancelFunc   context.CancelFunc
	wgClient     *WireGuardClient
	wgConfig     *WireGuardConfig
	sshConn      transport.Connection
}

func NewProxyService(bus *event.Bus) *ProxyService {
	svc := &ProxyService{}
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
		[]string{event.ConnectionEstablished, event.ConnectionClosed, event.WireGuardConfigReceived},
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

	s.connectionMu.Lock()
	if s.wgClient != nil {
		_ = s.wgClient.Stop()
		s.wgClient = nil
	}
	s.connectionMu.Unlock()

	return s.Service.Stop(ctx)
}

func (s *ProxyService) SetConfig(cfg config.Config) {
	s.connectionMu.Lock()
	s.config = cfg
	s.connectionMu.Unlock()
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
			s.connectionMu.Lock()
			s.sshConn = conn
			s.connectionMu.Unlock()
		}

	case event.ConnectionClosed:
		if s.cancelFunc != nil {
			s.cancelFunc()
			s.cancelFunc = nil
		}

		s.connectionMu.Lock()
		if s.wgClient != nil {
			_ = s.wgClient.Stop()
			s.wgClient = nil
		}
		s.sshConn = nil
		s.connectionMu.Unlock()

		s.bus.Publish(event.Event{Type: event.WireGuardDisconnected, Ctx: s.Context()})

	case event.WireGuardConfigReceived:
		if wgConfig, ok := evt.Data.(*WireGuardConfig); ok {
			s.handleWireGuardConfig(s.Context(), wgConfig)
		}
	}
}

func (s *ProxyService) handleWireGuardConfig(ctx context.Context, wgConfig *WireGuardConfig) {
	util.Debug(i18n.T("wireguard_config_received", map[string]any{
		"type":      "proxy",
		"Endpoint":  wgConfig.Endpoint,
		"PublicKey": wgConfig.PublicKey[:16] + "...",
	}), map[string]any{"component": "proxy"})

	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()

	if s.wgClient != nil {
		util.Debug(i18n.T("wireguard_client_stopping_existing", map[string]any{
			"type": "proxy",
		}), map[string]any{"component": "proxy"})
		_ = s.wgClient.Stop()
	}

	clientConfig := &WireGuardConfig{
		PrivateKey:   wgConfig.PrivateKey,
		PublicKey:    wgConfig.PublicKey,
		Endpoint:     wgConfig.Endpoint,
		AllowedIPs:   wgConfig.AllowedIPs,
		PresharedKey: wgConfig.PresharedKey,
		PersistentKA: wgConfig.PersistentKA,
		ListenPort:   wgConfig.ListenPort,
		ServerPubKey: wgConfig.ServerPubKey,
		DNS:          wgConfig.DNS,
		MTU:          wgConfig.MTU,
		TunnelIP:     wgConfig.TunnelIP,
	}

	util.Debug(i18n.T("wireguard_client_creating", map[string]any{
		"type":       "proxy",
		"ListenPort": clientConfig.ListenPort,
		"MTU":        clientConfig.MTU,
		"TunnelIP":   clientConfig.TunnelIP,
	}), map[string]any{"component": "proxy"})

	wgClient, err := NewWireGuardClient(clientConfig)
	if err != nil {
		util.LogError(i18n.T("wireguard_client_creation_failed", map[string]any{
			"type":  "proxy",
			"Error": err,
		}), err, map[string]any{"component": "proxy"})
		s.bus.Publish(event.Event{Type: event.ProxyFailed, Data: err, Ctx: ctx})
		return
	}

	util.Debug(i18n.T("wireguard_client_starting", map[string]any{
		"type": "proxy",
	}), map[string]any{"component": "proxy"})

	if err := wgClient.Start(); err != nil {
		util.LogError(i18n.T("wireguard_client_start_failed", map[string]any{
			"type":  "proxy",
			"Error": err,
		}), err, map[string]any{"component": "proxy"})
		s.bus.Publish(event.Event{Type: event.ProxyFailed, Data: err, Ctx: ctx})
		return
	}

	s.wgClient = wgClient
	s.wgConfig = wgConfig

	util.Info(i18n.T("wireguard_client_established", map[string]any{
		"type": "proxy",
	}), map[string]any{"component": "proxy"})

	s.bus.Publish(event.Event{Type: event.WireGuardConnected, Ctx: ctx})

	util.Debug(i18n.T("wireguard_registration_sending", map[string]any{
		"type": "proxy",
	}), map[string]any{"component": "proxy"})

	go func() {
		for range 5 {
			s.connectionMu.RLock()
			currentSSHConn := s.sshConn
			s.connectionMu.RUnlock()

			if currentSSHConn != nil {
				if err := s.sendRegistrationDetailsWithConnections(currentSSHConn, s.wgClient); err != nil {
					util.LogError(i18n.T("wireguard_registration_failed", map[string]any{
						"type":  "proxy",
						"Error": err,
					}), err, map[string]any{"component": "proxy"})
				}
				return
			}

			time.Sleep(100 * time.Millisecond)
		}

		util.LogError(i18n.T("wireguard_registration_timeout", map[string]any{
			"type": "proxy",
		}), nil, map[string]any{"component": "proxy"})
	}()

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	proxyCtx, cancelFunc := context.WithCancel(ctx)
	s.cancelFunc = cancelFunc

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.handleTunnelRequests(proxyCtx)
	}()
}

func (s *ProxyService) sendRegistrationDetailsWithConnections(sshConn transport.Connection, wgClient *WireGuardClient) error {
	if sshConn == nil || wgClient == nil {
		util.Debug(i18n.T("wireguard_registration_missing_connections", map[string]any{
			"type":      "proxy",
			"HasSSH":    sshConn != nil,
			"HasClient": wgClient != nil,
		}), map[string]any{"component": "proxy"})
		return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_registration_no_connection", map[string]any{}), nil)
	}

	s.connectionMu.RLock()
	publicKey := ""
	if s.wgConfig != nil {
		publicKey = s.wgConfig.PublicKey
	}
	s.connectionMu.RUnlock()

	registrationPayload := struct {
		Type      string `json:"type"`
		Status    string `json:"status"`
		ClientIP  string `json:"client_ip,omitempty"`
		PublicKey string `json:"public_key,omitempty"`
	}{
		Type:      "wireguard_registration",
		Status:    "connected",
		PublicKey: publicKey,
	}

	// Use configured tunnel IP
	if tunnelIP := wgClient.GetTunnelIP(); tunnelIP != "" {
		registrationPayload.ClientIP = tunnelIP
	} else {
		// Fallback to extracting IP from allowedIPs
		if clientIP, err := wgClient.getClientIP(); err == nil {
			registrationPayload.ClientIP = clientIP
			util.Debug(i18n.T("wireguard_client_ip_fallback", map[string]any{
				"type":     "proxy",
				"ClientIP": clientIP,
			}), map[string]any{"component": "proxy"})
		} else {
			util.Debug(i18n.T("wireguard_client_ip_failed", map[string]any{
				"type":  "proxy",
				"Error": err,
			}), map[string]any{"component": "proxy"})
		}
	}

	util.Debug(i18n.T("wireguard_registration_payload_sending", map[string]any{
		"type":      "proxy",
		"Status":    registrationPayload.Status,
		"ClientIP":  registrationPayload.ClientIP,
		"PublicKey": registrationPayload.PublicKey,
	}), map[string]any{"component": "proxy"})

	if err := sshConn.SendPayload(registrationPayload); err != nil {
		util.Debug(i18n.T("wireguard_registration_ssh_failed", map[string]any{
			"type":  "proxy",
			"Error": err,
		}), map[string]any{"component": "proxy"})
		return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_registration_send_failed", map[string]any{"Error": err}), err)
	}

	util.Info(i18n.T("wireguard_registration_sent", map[string]any{
		"type": "proxy",
	}), map[string]any{"component": "proxy"})

	return nil
}

func (s *ProxyService) handleTunnelRequests(ctx context.Context) {
	util.Debug(i18n.T("wireguard_tunnel_handler_started", map[string]any{
		"type": "proxy",
	}), map[string]any{"component": "proxy"})

	appConfig := s.config.Application
	localAddr := fmt.Sprintf("%s:%d", appConfig.IP, appConfig.Port)

	util.Debug(i18n.T("wireguard_tunnel_forwarding_setup", map[string]any{
		"type":      "proxy",
		"LocalAddr": localAddr,
	}), map[string]any{"component": "proxy"})

	// Set up listener on WireGuard tunnel network
	s.connectionMu.RLock()
	wgClient := s.wgClient
	s.connectionMu.RUnlock()

	if wgClient == nil {
		util.LogError(i18n.T("wireguard_tunnel_no_client", map[string]any{
			"type": "proxy",
		}), nil, map[string]any{"component": "proxy"})
		return
	}

	// Listen for incoming connections on the tunnel network
	listener, err := wgClient.Listen("tcp", ":"+strconv.Itoa(appConfig.Port))
	if err != nil {
		util.LogError(i18n.T("wireguard_tunnel_listen_failed", map[string]any{
			"type":  "proxy",
			"Error": err,
		}), err, map[string]any{"component": "proxy"})
		return
	}
	defer func() { _ = listener.Close() }()

	util.Info(i18n.T("wireguard_tunnel_listener_started", map[string]any{
		"type": "proxy",
		"Port": appConfig.Port,
	}), map[string]any{"component": "proxy"})

	// Accept connections in a goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					util.LogError(i18n.T("wireguard_tunnel_accept_failed", map[string]any{
						"type":  "proxy",
						"Error": err,
					}), err, map[string]any{"component": "proxy"})
					continue
				}
			}

			// Handle each connection in a separate goroutine
			go s.handleTunnelConnection(ctx, conn, localAddr)
		}
	}()

	// Keep the handler alive and monitor connection health
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			util.Debug(i18n.T("wireguard_tunnel_handler_stopped", map[string]any{
				"type": "proxy",
			}), map[string]any{"component": "proxy"})
			return
		case <-ticker.C:
			s.connectionMu.RLock()
			isRunning := s.wgClient != nil && s.wgClient.IsRunning()
			s.connectionMu.RUnlock()

			if !isRunning {
				util.Warn(i18n.T("wireguard_connection_lost", map[string]any{
					"type": "proxy",
				}), map[string]any{"component": "proxy"})
				s.bus.Publish(event.Event{Type: event.WireGuardDisconnected, Ctx: ctx})
				return
			} else {
				util.Debug(i18n.T("wireguard_connection_alive", map[string]any{
					"type": "proxy",
				}), map[string]any{"component": "proxy"})
			}
		}
	}
}

func (s *ProxyService) handleTunnelConnection(ctx context.Context, tunnelConn net.Conn, localAddr string) {
	defer func() { _ = tunnelConn.Close() }()

	util.Debug(i18n.T("wireguard_tunnel_connection_accepted", map[string]any{
		"type":       "proxy",
		"RemoteAddr": tunnelConn.RemoteAddr().String(),
	}), map[string]any{"component": "proxy"})

	// Connect to local application
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		util.LogError(i18n.T("wireguard_tunnel_forward_failed", map[string]any{
			"type":      "proxy",
			"LocalAddr": localAddr,
			"Error":     err,
		}), err, map[string]any{"component": "proxy"})
		return
	}
	defer func() { _ = localConn.Close() }()

	util.Debug(i18n.T("wireguard_tunnel_forward_started", map[string]any{
		"type":      "proxy",
		"LocalAddr": localAddr,
	}), map[string]any{"component": "proxy"})

	// Use errgroup for coordinated bidirectional forwarding
	g, gCtx := errgroup.WithContext(ctx)

	// Forward from tunnel to local application
	g.Go(func() error {
		defer func() { _ = localConn.Close() }()
		_, err := io.Copy(localConn, tunnelConn)
		return err
	})

	// Forward from local application to tunnel
	g.Go(func() error {
		defer func() { _ = tunnelConn.Close() }()
		_, err := io.Copy(tunnelConn, localConn)
		return err
	})

	<-gCtx.Done()
	// Context cancelled, set deadlines to force cleanup
	_ = tunnelConn.SetDeadline(time.Now().Add(time.Second))
	_ = localConn.SetDeadline(time.Now().Add(time.Second))

	// Wait for all goroutines to finish
	if err := g.Wait(); err != nil {
		util.Debug(i18n.T("wireguard_tunnel_proxy_error", map[string]any{
			"type":  "proxy",
			"Error": err,
		}), map[string]any{"component": "proxy"})
	}
}
