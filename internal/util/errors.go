package util

import (
	"context"
	"errors"
	"io"
)

func IsExpectedError(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.ErrClosedPipe)
}
