package transport

import (
	"time"
)

type ConnectionOptions struct {
	Protocol string
	Host     string
	Port     int
	User     string
	KeyFile  string
	Timeout  time.Duration
	Tunnel   bool
}

type Option func(*ConnectionOptions)

func WithProtocol(protocol string) Option {
	return func(o *ConnectionOptions) {
		o.Protocol = protocol
	}
}

func WithHost(host string) Option {
	return func(o *ConnectionOptions) {
		o.Host = host
	}
}

func WithPort(port int) Option {
	return func(o *ConnectionOptions) {
		o.Port = port
	}
}

func WithUser(user string) Option {
	return func(o *ConnectionOptions) {
		o.User = user
	}
}

func WithKeyFile(keyFile string) Option {
	return func(o *ConnectionOptions) {
		o.KeyFile = keyFile
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(o *ConnectionOptions) {
		o.Timeout = timeout
	}
}

func WithTunnel(enabled bool) Option {
	return func(o *ConnectionOptions) {
		o.Tunnel = enabled
	}
}

func DefaultOptions() ConnectionOptions {
	return ConnectionOptions{
		Protocol: DefaultProtocol,
		Timeout:  30 * time.Second,
		Tunnel:   false,
	}
}

func ApplyOptions(opts ...Option) ConnectionOptions {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}
