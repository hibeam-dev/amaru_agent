package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
)

var (
	ErrConfigLoad       = errors.New("config load failed")
	ErrConnectionFailed = errors.New("connection failed")
	ErrSessionFailed    = errors.New("session failed")
	ErrSubsystemFailed  = errors.New("subsystem failed")
)

func IsExpectedError(err error) bool {
	return err == nil ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed)
}

func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE)
}

func WrapWithBase(base error, msg string, err error) error {
	return fmt.Errorf("%w: %s: %w", base, msg, err)
}

func WrapError(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, err)
}
