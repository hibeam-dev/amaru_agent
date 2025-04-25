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
	"erlang-solutions.com/cortex_agent/internal/transport"
	"erlang-solutions.com/cortex_agent/internal/util"
)

const ResponseTimeout = 10 * time.Second

func HandleJSONMode(ctx context.Context, conn transport.Connection, in io.Reader, out io.Writer) error {
	connEncoder := json.NewEncoder(conn.Stdin())

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

	return processRequests(ctx, cancelRead, decoder, connEncoder, outEncoder, responseCh, errorCh)
}

func daemonModeWait(ctx context.Context, errorCh <-chan error) error {
	for {
		select {
		case <-ctx.Done():
			log.Println(i18n.T("json_daemon_shutdown", nil))
			return ctx.Err()
		case err := <-errorCh:
			if util.IsExpectedError(err) {
				log.Println(i18n.T("connection_closed", nil))
				return nil
			}
			log.Printf("%s", i18n.T("connection_error_log", map[string]interface{}{"Error": err}))
			return err
		}
	}
}

func readResponses(ctx context.Context, conn transport.Connection, responseCh chan<- json.RawMessage, errorCh chan<- error) {
	defer log.Println(i18n.T("reader_exiting", nil))

	connDecoder := json.NewDecoder(conn.Stdout())
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var response json.RawMessage
			err := connDecoder.Decode(&response)
			if err != nil {
				if util.IsExpectedError(err) {
					return
				}
				log.Printf("%s", i18n.T("decode_error", map[string]interface{}{"Error": err}))
				select {
				case errorCh <- fmt.Errorf("failed to read response: %w", err):
				case <-ctx.Done():
				}
				return
			}

			log.Printf("%s", i18n.T("server_response", map[string]interface{}{"Response": string(response)}))
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
	connEncoder *json.Encoder,
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
				if util.IsExpectedError(err) {
					if errors.Is(err, io.EOF) {
						log.Println(i18n.T("input_closed", nil))
						return nil
					}
					log.Println(i18n.T("context_cancelled_read", nil))
					return ctx.Err()
				}
				return fmt.Errorf("failed to decode input: %w", err)
			}

			if err := connEncoder.Encode(request); err != nil {
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
					log.Printf("%s", i18n.T("response_no_writer", map[string]interface{}{"Response": string(response)}))
				}
			case <-responseTimer.C:
				log.Println(i18n.T("response_timeout", nil))
				continue
			}
		}
	}
}
