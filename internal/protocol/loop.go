package protocol

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/transport"
	"erlang-solutions.com/cortex_agent/internal/util"
)

func RunMainLoop(ctx context.Context, conn transport.Connection, reconnectCh <-chan struct{}) error {
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
					return fmt.Errorf("%s", i18n.T("heartbeat_error", map[string]interface{}{"Error": err}))
				}
			}
		}
	})

	// Wait for any signal to stop
	var result error
	select {
	case <-ctx.Done():
		cancelRead()
		result = ctx.Err()
	case err := <-dataErrCh:
		cancelRead()
		if !util.IsExpectedError(err) {
			result = err
		}
	case <-reconnectCh:
		cancelRead()
	}

	util.WaitGroupWithTimeout(g, 500*time.Millisecond)

	return result
}

func readData(ctx context.Context, conn transport.Connection, errorCh chan<- error) error {
	buffer := make([]byte, 8192)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			n, err := conn.Stdout().Read(buffer)
			if err != nil {
				if util.IsExpectedError(err) {
					return nil
				}

				select {
				case errorCh <- fmt.Errorf("%s", i18n.T("transport_read_error", map[string]interface{}{"Error": err})):
				case <-ctx.Done():
				}
				return err
			}

			_ = n
		}
	}
}
