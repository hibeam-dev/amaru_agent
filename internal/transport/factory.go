package transport

import (
	"context"
	"fmt"

	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/util"
)

type ConnCreator interface {
	CreateConnection(ctx context.Context, cfg config.Config, options ConnectionOptions) (Connection, error)
}

var transportRegistry = make(map[string]ConnCreator)

func RegisterTransport(protocol string, creator ConnCreator) {
	transportRegistry[protocol] = creator
}

const DefaultProtocol = "ssh"

func Connect(ctx context.Context, cfg config.Config, opts ...Option) (Connection, error) {
	options := ApplyOptions(opts...)

	creator, ok := transportRegistry[options.Protocol]
	if !ok {
		return nil, util.NewError(util.ErrTypeConnection,
			fmt.Sprintf("transport protocol '%s' not available - ensure required package is included in the build", options.Protocol), nil)
	}

	return creator.CreateConnection(ctx, cfg, options)
}
