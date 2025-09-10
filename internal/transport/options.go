package transport

import (
	"time"
)

type ConnectionOptions struct {
	Protocol string
	Host     string
	Port     int
	KeyFile  string
	Timeout  time.Duration
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

func DefaultOptions() ConnectionOptions {
	return ConnectionOptions{
		Protocol: DefaultProtocol,
		Timeout:  30 * time.Second,
	}
}

func ApplyOptions(opts ...Option) ConnectionOptions {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}
	return options
}
