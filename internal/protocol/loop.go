package protocol

import (
	"context"
	"fmt"
	"log"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

func RunMainLoop(ctx context.Context, conn ssh.Connection, reconnectCh <-chan struct{}) error {
	errorCh := make(chan error, 1)

	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()

	go readData(readCtx, conn, errorCh)
	go readStderr(readCtx, conn)

	for {
		select {
		case <-ctx.Done():
			log.Println(i18n.T("json_shutdown", nil))
			return ctx.Err()
		case err := <-errorCh:
			if isExpectedError(err) {
				log.Println(i18n.T("connection_closed", nil))
				return nil
			}
			log.Printf(i18n.Tf("connection_error_log", nil), i18n.T("connection_error_log", map[string]interface{}{"Error": err}))
			return err
		case <-reconnectCh:
			log.Println(i18n.T("reconnection_requested", nil))
			return nil
		}
	}
}

func readData(ctx context.Context, conn ssh.Connection, errorCh chan<- error) {
	defer log.Println(i18n.T("main_loop_exiting", nil))

	buffer := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := conn.Stdout().Read(buffer)
			if err != nil {
				if isExpectedError(err) {
					return
				}
				select {
				case errorCh <- fmt.Errorf("failed to read from SSH connection: %w", err):
				case <-ctx.Done():
				}
				return
			}

			if n > 0 {
				log.Printf(i18n.Tf("bytes_received", nil), i18n.T("bytes_received", map[string]interface{}{"Count": n}))

				// Process data received from server
				// This is where you could add more sophisticated handling of server messages
				// For now, we just log the receipt of data
			}
		}
	}
}
