package transport

import (
	"io"
)

type Connection interface {
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	SendPayload(payload any) error
	Close() error
}
