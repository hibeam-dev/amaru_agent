package protocol

import (
	"context"
	"encoding/json"
	"errors"
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

	responseCh := make(chan json.RawMessage, 5)
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
			log.Printf("%s", i18n.T("connection_error_log", map[string]any{"Error": err}))
			return err
		}
	}
}

func readResponses(ctx context.Context, conn transport.Connection, responseCh chan<- json.RawMessage, errorCh chan<- error) {
	defer log.Println(i18n.T("reader_exiting", nil))

	connDecoder := json.NewDecoder(conn.Stdout())
	for {
		if ctx.Err() != nil {
			return
		}

		if setter, ok := conn.Stdout().(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = setter.SetReadDeadline(time.Now().Add(5 * time.Second))
		}

		var response json.RawMessage
		err := connDecoder.Decode(&response)

		if ctx.Err() != nil {
			return
		}

		if err != nil {
			if util.IsExpectedError(err) {
				return
			}
			log.Printf("%s", i18n.T("decode_error", map[string]any{"Error": err}))
			select {
			case errorCh <- util.NewError(util.ErrTypeConnection, "failed to read response", err):
			case <-ctx.Done():
			}
			return
		}

		log.Printf("%s", i18n.T("server_response", map[string]any{"Response": string(response)}))
		select {
		case <-ctx.Done():
			return
		case responseCh <- response:
			// Response sent successfully
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
				return util.NewError(util.ErrTypeConnection, "failed to decode input", err)
			}

			if err := connEncoder.Encode(request); err != nil {
				return util.NewError(util.ErrTypeConnection, "failed to send request", err)
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
						return util.NewError(util.ErrTypeConnection, "failed to encode response", err)
					}
				} else {
					// In daemon mode, just log the response for now
					log.Printf("%s", i18n.T("response_no_writer", map[string]any{"Response": string(response)}))
				}
			case <-responseTimer.C:
				log.Println(i18n.T("response_timeout", nil))
				continue
			}
		}
	}
}
