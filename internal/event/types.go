package event

import (
	"context"
)

const (
	ConfigUpdated           = "config_updated"
	ConnectionEstablished   = "connection_established"
	ConnectionClosed        = "connection_closed"
	ConnectionFailed        = "connection_failed"
	ReconnectRequested      = "reconnect_requested"
	TerminationSignal       = "termination_signal"
	SIGHUPReceived          = "sighup_received"
	ProxyEstablished        = "proxy_established"
	ProxyFailed             = "proxy_failed"
	WireGuardConfigReceived = "wireguard_config_received"
	WireGuardConnected      = "wireguard_connected"
	WireGuardDisconnected   = "wireguard_disconnected"
)

type Event struct {
	Type string
	Data any
	Ctx  context.Context
}
