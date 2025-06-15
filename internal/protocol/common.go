package protocol

import (
	"context"
	"time"

	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/transport"
	"erlang-solutions.com/amaru_agent/internal/util"
)

func readStderr(ctx context.Context, conn transport.Connection) {
	buffer := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return

		default:
			n, err := conn.Stderr().Read(buffer)
			if err != nil {
				return
			}

			if n > 0 {
				util.Debug("stderr output", "message", string(buffer[:n]))
			}
		}
	}
}

func RunHeartbeat(ctx context.Context, conn transport.Connection, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := conn.SendPayload(map[string]string{"type": "heartbeat"}); err != nil {
				return util.NewError(util.ErrTypeConnection, i18n.T("heartbeat_error", map[string]any{"Error": err}), err)
			}
		}
	}
}
