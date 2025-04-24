package protocol

import (
	"context"
	"errors"
	"io"

	"erlang-solutions.com/cortex_agent/internal/ssh"
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
