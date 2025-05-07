package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"syscall"
)

type ErrorType string

const (
	ErrTypeConfig     ErrorType = "config"
	ErrTypeConnection ErrorType = "connection"
	ErrTypeSession    ErrorType = "session"
	ErrTypeSubsystem  ErrorType = "subsystem"
	ErrTypeService    ErrorType = "service"
)

type AppError struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("error_type", string(e.Type)),
		slog.String("error_message", e.Message),
	}

	if e.Cause != nil {
		attrs = append(attrs, slog.String("cause", e.Cause.Error()))

		// If cause is also an AppError, get its type
		var appErr *AppError
		if errors.As(e.Cause, &appErr) {
			attrs = append(attrs, slog.String("cause_type", string(appErr.Type)))
		}
	}

	return slog.GroupValue(attrs...)
}

var (
	ErrConfigLoad       = NewError(ErrTypeConfig, "config load failed", nil)
	ErrConnectionFailed = NewError(ErrTypeConnection, "connection failed", nil)
	ErrSessionFailed    = NewError(ErrTypeSession, "session failed", nil)
	ErrSubsystemFailed  = NewError(ErrTypeSubsystem, "subsystem failed", nil)
)

func NewError(errType ErrorType, message string, cause error) error {
	return &AppError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

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
	var appErr *AppError
	if errors.As(base, &appErr) {
		return NewError(appErr.Type, msg, err)
	}
	return fmt.Errorf("%w: %s: %v", base, msg, err)
}

func WrapError(msg string, err error) error {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return &AppError{
			Type:    appErr.Type,
			Message: msg,
			Cause:   appErr,
		}
	}

	return fmt.Errorf("%s: %w", msg, err)
}
