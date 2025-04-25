package event

const (
	ConfigUpdated         = "config_updated"
	ConnectionEstablished = "connection_established"
	ConnectionClosed      = "connection_closed"
	ConnectionFailed      = "connection_failed"
	ReconnectRequested    = "reconnect_requested"
	TerminationSignal     = "termination_signal"
	SIGHUPReceived        = "sighup_received"
)

type Event struct {
	Type string
	Data interface{}
}
