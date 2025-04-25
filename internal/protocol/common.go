package protocol

import (
	"context"
	"time"

	"erlang-solutions.com/cortex_agent/internal/transport"
	"erlang-solutions.com/cortex_agent/internal/util"
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
				if util.IsExpectedError(err) {
					return
				}
				return
			}

			_ = n
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
				return err
			}
		}
	}
}
