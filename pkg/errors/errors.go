package errors

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrConfigLoad   = errors.New("config load failed")
	ErrSSHConnect   = errors.New("SSH connection failed")
	ErrSSHSession   = errors.New("SSH session failed")
	ErrSSHSubsystem = errors.New("SSH subsystem failed")
)

func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}

func WrapWithBase(base error, msg string, err error) error {
	return fmt.Errorf("%w: %s: %v", base, msg, err)
}

func New(text string) error {
	return errors.New(text)
}

// Returns an error that wraps the given errors
func Join(errs ...error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}

	var sb strings.Builder
	for i, err := range filtered {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(err.Error())
	}
	return errors.New(sb.String())
}
