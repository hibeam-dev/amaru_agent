package transport

import (
	"io"
)

type Connection interface {
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	SendPayload(payload interface{}) error
	Close() error
}
