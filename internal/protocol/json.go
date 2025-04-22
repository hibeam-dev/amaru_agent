package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

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
			log.Println("Context cancelled, shutting down daemon JSON mode...")
			return ctx.Err()
		case err := <-errorCh:
			if isExpectedError(err) {
				log.Println("Connection closed normally")
				return nil
			}
			log.Printf("Connection error: %v", err)
			return err
		}
	}
}

func readResponses(ctx context.Context, conn ssh.Connection, responseCh chan<- json.RawMessage, errorCh chan<- error) {
	defer log.Println("Response reader goroutine exiting")

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
				log.Printf("Error decoding server response: %v", err)
				select {
				case errorCh <- fmt.Errorf("failed to read response: %w", err):
				case <-ctx.Done():
				}
				return
			}

			log.Printf("Received response from server: %s", string(response))
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
			log.Println("Context cancelled, shutting down JSON mode...")
			return ctx.Err()
		case err := <-errorCh:
			return err
		default:
			var request json.RawMessage
			if err := decoder.Decode(&request); err != nil {
				if errors.Is(err, io.EOF) {
					log.Println("Input stream closed")
					return nil
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Println("Context cancelled during read")
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
					// In daemon mode, just log the response
					log.Printf("Received response (no output writer): %s", string(response))
				}
			case <-responseTimer.C:
				log.Println("Timeout waiting for response")
				continue
			}
		}
	}
}
