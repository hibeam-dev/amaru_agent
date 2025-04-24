package protocol

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"time"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func RunMainLoop(ctx context.Context, conn ssh.Connection, reconnectCh <-chan struct{}) error {
	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()

	g, gCtx := errgroup.WithContext(readCtx)

	dataErrCh := make(chan error, 1)

	g.Go(func() error {
		return readData(gCtx, conn, dataErrCh)
	})

	g.Go(func() error {
		readStderr(gCtx, conn)
		return nil
	})

	g.Go(func() error {
		err := RunHeartbeat(gCtx, conn, 30*time.Second)
		if err != nil {
			return fmt.Errorf("%s", i18n.T("heartbeat_error", map[string]interface{}{"Error": err}))
		}
		return nil
	})

	// Wait for any signal to stop
	var result error
	select {
	case <-ctx.Done():
		cancelRead()
		result = ctx.Err()
	case err := <-dataErrCh:
		cancelRead()
		if !isExpectedError(err) {
			result = err
		}
	case <-reconnectCh:
		cancelRead()
	}

	waitWithTimeout(g, 500*time.Millisecond)

	return result
}

func readData(ctx context.Context, conn ssh.Connection, errorCh chan<- error) error {
	buffer := make([]byte, 8192)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			n, err := conn.Stdout().Read(buffer)
			if err != nil {
				if isExpectedError(err) {
					return nil
				}

				select {
				case errorCh <- fmt.Errorf("%s", i18n.T("ssh_read_error", map[string]interface{}{"Error": err})):
				case <-ctx.Done():
				}
				return err
			}

			_ = n
		}
	}
}
