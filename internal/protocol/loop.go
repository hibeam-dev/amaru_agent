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

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	g.Go(func() error {
		for {
			select {
			case <-gCtx.Done():
				return nil
			case <-heartbeatTicker.C:
				if err := conn.SendPayload(map[string]string{"type": "heartbeat"}); err != nil {
					return fmt.Errorf("heartbeat failed: %w", err)
				}
			}
		}
	})

	select {
	case <-ctx.Done():
		cancelRead()
		_ = g.Wait()
		return ctx.Err()

	case err := <-dataErrCh:
		cancelRead()
		_ = g.Wait()
		if isExpectedError(err) {
			return nil
		}
		return err

	case <-reconnectCh:
		cancelRead()
		_ = g.Wait()
		return nil
	}
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
