package transport

import (
	"context"
	"io"
)

type Connection interface {
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	SendPayload(payload any) error
	Close() error
	CheckHealth(ctx context.Context) error

	// Provides direct access to a binary data channel for tunneling raw data
	BinaryInput() io.WriteCloser
	BinaryOutput() io.Reader
}
