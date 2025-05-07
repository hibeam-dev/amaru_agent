package protocol

import (
	"context"
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

	g.Go(func() error {
		return RunHeartbeat(gCtx, conn, 30*time.Second)
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

	if err := g.Wait(); err != nil && !util.IsExpectedError(err) {
		util.LogError("Error in main loop", err)
	}

	return result
}

func readData(ctx context.Context, conn transport.Connection, errorCh chan<- error) error {
	buffer := make([]byte, 8192)

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if setter, ok := conn.Stdout().(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = setter.SetReadDeadline(time.Now().Add(5 * time.Second))
		}

		n, err := conn.Stdout().Read(buffer)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			if util.IsExpectedError(err) {
				return nil
			}

			wrappedErr := util.NewError(util.ErrTypeConnection, i18n.T("transport_read_error", map[string]any{"Error": err}), err)
			select {
			case errorCh <- wrappedErr:
			case <-ctx.Done():
			}
			return wrappedErr
		}

		if n > 0 {
			util.Debug("Received bytes from transport", "bytes", n)
		}
	}
}
