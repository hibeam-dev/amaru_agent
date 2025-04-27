package transport

import (
	"context"
	"fmt"

	"erlang-solutions.com/cortex_agent/internal/config"
)

type ConnCreator interface {
	CreateConnection(ctx context.Context, cfg config.Config, opts map[string]any) (Connection, error)
}

var transportRegistry = make(map[string]ConnCreator)

func RegisterTransport(protocol string, creator ConnCreator) {
	transportRegistry[protocol] = creator
}

const DefaultProtocol = "ssh"

func NewConnection(ctx context.Context, cfg config.Config, opts ...map[string]any) (Connection, error) {
	protocol := DefaultProtocol
	connectionOpts := make(map[string]any)

	if len(opts) > 0 && opts[0] != nil {
		connectionOpts = opts[0]
	}

	creator, ok := transportRegistry[protocol]
	if !ok {
		return nil, fmt.Errorf("transport protocol '%s' not available - ensure required package is included in the build",
			protocol)
	}

	return creator.CreateConnection(ctx, cfg, connectionOpts)
}
