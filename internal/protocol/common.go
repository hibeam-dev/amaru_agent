package protocol

import (
	"context"
	"errors"
	"io"
	"time"

	"erlang-solutions.com/cortex_agent/internal/ssh"
	"golang.org/x/sync/errgroup"
)

func readStderr(ctx context.Context, conn ssh.Connection) {
	buffer := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return

		default:
			n, err := conn.Stderr().Read(buffer)
			if err != nil {
				if isExpectedError(err) {
					return
				}
				return
			}

			_ = n
		}
	}
}

func isExpectedError(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.ErrClosedPipe)
}

func waitWithTimeout(g *errgroup.Group, timeout time.Duration) {
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		_ = g.Wait()
	}()

	select {
	case <-waitDone:
	case <-time.After(timeout):
	}
}

func RunHeartbeat(ctx context.Context, conn ssh.Connection, interval time.Duration) error {
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
