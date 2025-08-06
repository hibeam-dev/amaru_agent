package service

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/util"
)

type WireGuardClient struct {
	device     *device.Device
	tun        *netstack.Net
	tunDevice  any
	privateKey wgtypes.Key
	publicKey  wgtypes.Key
	serverKey  wgtypes.Key
	endpoint   string
	allowedIPs []string
	listenPort int
	mtu        int
	dns        string
	tunnelIP   string
	mu         sync.RWMutex
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

type WireGuardClientConfig struct {
	PrivateKey   string   `json:"private_key"`
	PublicKey    string   `json:"public_key"`
	Endpoint     string   `json:"endpoint"`
	AllowedIPs   []string `json:"allowed_ips"`
	PresharedKey string   `json:"preshared_key,omitempty"`
	PersistentKA int      `json:"persistent_keepalive,omitempty"`
	ListenPort   int      `json:"listen_port,omitempty"`
	ServerPubKey string   `json:"server_public_key"`
	DNS          string   `json:"dns,omitempty"`
	MTU          int      `json:"mtu,omitempty"`
	TunnelIP     string   `json:"tunnel_ip"`
}

func NewWireGuardClient(config *WireGuardClientConfig) (*WireGuardClient, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := &WireGuardClient{
		endpoint:   config.Endpoint,
		allowedIPs: config.AllowedIPs,
		listenPort: config.ListenPort,
		mtu:        config.MTU,
		dns:        config.DNS,
		tunnelIP:   config.TunnelIP,
		ctx:        ctx,
		cancel:     cancel,
	}

	privateKey, err := parseKey(config.PrivateKey)
	if err != nil {
		cancel()
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_private_key_error", map[string]any{"Error": err}), err)
	}
	client.privateKey = privateKey
	client.publicKey = privateKey.PublicKey()

	serverKey, err := parseKey(config.ServerPubKey)
	if err != nil {
		cancel()
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_server_key_error", map[string]any{"Error": err}), err)
	}
	client.serverKey = serverKey

	if client.mtu == 0 {
		client.mtu = 1420
	}
	if client.dns == "" {
		client.dns = "8.8.8.8"
	}

	return client, nil
}

func (c *WireGuardClient) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	tunnelIP := c.tunnelIP
	util.Debug(i18n.T("wireguard_client_tunnel_ip_debug", map[string]any{
		"type":     "wireguard",
		"TunnelIP": tunnelIP,
	}))
	if tunnelIP == "" {
		var err error
		tunnelIP, err = c.getClientIP()
		if err != nil {
			return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_client_ip_error", map[string]any{"Error": err}), err)
		}
		util.Debug(i18n.T("wireguard_client_fallback_ip_debug", map[string]any{
			"type":       "wireguard",
			"FallbackIP": tunnelIP,
		}))
	}

	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr(tunnelIP)},
		[]netip.Addr{netip.MustParseAddr(c.dns)},
		c.mtu,
	)
	if err != nil {
		return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_tun_error", map[string]any{"Error": err}), err)
	}

	c.tunDevice = tun
	c.tun = tnet

	// Create WireGuard device logger
	logger := device.NewLogger(
		device.LogLevelVerbose,
		"wireguard-client: ",
	)

	c.device = device.NewDevice(tun, conn.NewDefaultBind(), logger)

	// Configure WireGuard device
	config := fmt.Sprintf("private_key=%s\n", keyToHex(c.privateKey))
	if c.listenPort > 0 {
		config += fmt.Sprintf("listen_port=%d\n", c.listenPort)
	}

	// Add server peer
	config += fmt.Sprintf("public_key=%s\n", keyToHex(c.serverKey))
	config += fmt.Sprintf("endpoint=%s\n", c.endpoint)
	config += fmt.Sprintf("persistent_keepalive_interval=%d\n", 25)

	for _, ip := range c.allowedIPs {
		config += fmt.Sprintf("allowed_ip=%s\n", ip)
	}

	util.Debug(i18n.T("wireguard_config_debug", map[string]any{
		"type":     "wireguard",
		"Config":   config,
		"Endpoint": c.endpoint,
	}))

	err = c.device.IpcSet(config)
	if err != nil {
		c.device.Close()
		return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_config_error", map[string]any{"Error": err}), err)
	}

	err = c.device.Up()
	if err != nil {
		c.device.Close()
		return util.NewError(util.ErrTypeConnection, i18n.T("wireguard_start_error", map[string]any{"Error": err}), err)
	}

	c.running = true

	util.Info(i18n.T("wireguard_client_started", map[string]any{
		"type":     "wireguard",
		"ClientIP": tunnelIP,
		"Endpoint": c.endpoint,
	}))

	return nil
}

func (c *WireGuardClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.cancel()
	c.running = false

	if c.device != nil {
		c.device.Close()
		c.device = nil
	}

	c.tunDevice = nil
	c.tun = nil

	util.Info(i18n.T("wireguard_client_stopped", map[string]any{
		"type": "wireguard",
	}))

	return nil
}

func (c *WireGuardClient) Dial(network, address string) (net.Conn, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.running || c.tun == nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_not_running", map[string]any{}), nil)
	}

	conn, err := c.tun.Dial(network, address)
	if err != nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_dial_error", map[string]any{
			"Address": address,
			"Error":   err,
		}), err)
	}

	return conn, nil
}

func (c *WireGuardClient) Listen(network, address string) (net.Listener, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.running || c.tun == nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_not_running", map[string]any{}), nil)
	}

	tcpAddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_listen_error", map[string]any{
			"Address": address,
			"Error":   err,
		}), err)
	}

	listener, err := c.tun.ListenTCP(tcpAddr)
	if err != nil {
		return nil, util.NewError(util.ErrTypeConnection, i18n.T("wireguard_listen_error", map[string]any{
			"Address": address,
			"Error":   err,
		}), err)
	}

	return listener, nil
}

func (c *WireGuardClient) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *WireGuardClient) GetTunnelIP() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tunnelIP
}

func (c *WireGuardClient) getClientIP() (string, error) {
	for _, ip := range c.allowedIPs {
		if addr, err := netip.ParsePrefix(ip); err == nil {
			return addr.Addr().String(), nil
		}
	}
	return "", fmt.Errorf("no valid client IP found in allowedIPs")
}

func parseKey(keyStr string) (wgtypes.Key, error) {
	return wgtypes.ParseKey(keyStr)
}

func keyToHex(key wgtypes.Key) string {
	decoded, err := base64.StdEncoding.DecodeString(key.String())
	if err != nil {
		log.Printf("Error decoding base64 key: %v", err)
		return ""
	}
	return hex.EncodeToString(decoded)
}
