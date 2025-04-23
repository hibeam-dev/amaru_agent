package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/ssh"
)

const ResponseTimeout = 10 * time.Second

func HandleJSONMode(ctx context.Context, conn ssh.Connection, in io.Reader, out io.Writer) error {
	sshEncoder := json.NewEncoder(conn.Stdin())

	responseCh := make(chan json.RawMessage)
	errorCh := make(chan error, 1)

	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()

	go readResponses(readCtx, conn, responseCh, errorCh)
	go readStderr(readCtx, conn)

	if in == nil {
		return daemonModeWait(ctx, errorCh)
	}

	decoder := json.NewDecoder(in)
	var outEncoder *json.Encoder
	if out != nil {
		outEncoder = json.NewEncoder(out)
	}

	return processRequests(ctx, cancelRead, decoder, sshEncoder, outEncoder, responseCh, errorCh)
}

func daemonModeWait(ctx context.Context, errorCh <-chan error) error {
	for {
		select {
		case <-ctx.Done():
			log.Println(i18n.T("json_daemon_shutdown", nil))
			return ctx.Err()
		case err := <-errorCh:
			if isExpectedError(err) {
				log.Println(i18n.T("connection_closed", nil))
				return nil
			}
			log.Printf(i18n.Tf("connection_error_log", nil), i18n.T("connection_error_log", map[string]interface{}{"Error": err}))
			return err
		}
	}
}

func readResponses(ctx context.Context, conn ssh.Connection, responseCh chan<- json.RawMessage, errorCh chan<- error) {
	defer log.Println(i18n.T("reader_exiting", nil))

	sshDecoder := json.NewDecoder(conn.Stdout())
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var response json.RawMessage
			err := sshDecoder.Decode(&response)
			if err != nil {
				if isExpectedError(err) {
					return
				}
				log.Printf(i18n.Tf("decode_error", nil), i18n.T("decode_error", map[string]interface{}{"Error": err}))
				select {
				case errorCh <- fmt.Errorf("failed to read response: %w", err):
				case <-ctx.Done():
				}
				return
			}

			log.Printf(i18n.Tf("server_response", nil), i18n.T("server_response", map[string]interface{}{"Response": string(response)}))
			select {
			case <-ctx.Done():
				return
			case responseCh <- response:
				// Response sent successfully
			}
		}
	}
}

func processRequests(
	ctx context.Context,
	cancelRead context.CancelFunc,
	decoder *json.Decoder,
	sshEncoder *json.Encoder,
	outEncoder *json.Encoder,
	responseCh <-chan json.RawMessage,
	errorCh <-chan error,
) error {
	for {
		select {
		case <-ctx.Done():
			log.Println(i18n.T("json_shutdown", nil))
			return ctx.Err()
		case err := <-errorCh:
			return err
		default:
			var request json.RawMessage
			if err := decoder.Decode(&request); err != nil {
				if errors.Is(err, io.EOF) {
					log.Println(i18n.T("input_closed", nil))
					return nil
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Println(i18n.T("context_cancelled_read", nil))
					return ctx.Err()
				}
				return fmt.Errorf("failed to decode input: %w", err)
			}

			if err := sshEncoder.Encode(request); err != nil {
				return fmt.Errorf("failed to send request: %w", err)
			}

			responseTimer := time.NewTimer(ResponseTimeout)
			select {
			case <-ctx.Done():
				responseTimer.Stop()
				return ctx.Err()
			case err := <-errorCh:
				responseTimer.Stop()
				return err
			case response := <-responseCh:
				responseTimer.Stop()
				if outEncoder != nil {
					if err := outEncoder.Encode(response); err != nil {
						return fmt.Errorf("failed to encode response: %w", err)
					}
				} else {
					// In daemon mode, just log the response for now
					log.Printf(i18n.Tf("response_no_writer", nil), i18n.T("response_no_writer", map[string]interface{}{"Response": string(response)}))
				}
			case <-responseTimer.C:
				log.Println(i18n.T("response_timeout", nil))
				continue
			}
		}
	}
}
