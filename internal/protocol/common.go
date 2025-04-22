package protocol

import (
	"context"
	"errors"
	"io"
	"log"

	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func readStderr(ctx context.Context, conn ssh.Connection) {
	defer log.Println("Stderr reader goroutine exiting")

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
				log.Printf("Error reading stderr: %v", err)
				return
			}
			if n > 0 {
				log.Printf("Server stderr: %s", string(buffer[:n]))
			}
		}
	}
}

func isExpectedError(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.ErrClosedPipe)
}
