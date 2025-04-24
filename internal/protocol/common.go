package protocol

import (
	"context"
	"errors"
	"io"
	"log"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func readStderr(ctx context.Context, conn ssh.Connection) {
	defer log.Println(i18n.T("stderr_reader_exiting", nil))

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
				log.Printf(i18n.Tf("stderr_read_error", nil), i18n.T("stderr_read_error", map[string]interface{}{"Error": err}))
				return
			}
			if n > 0 {
				log.Printf(i18n.Tf("server_stderr", nil), i18n.T("server_stderr", map[string]interface{}{"Output": string(buffer[:n])}))
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
