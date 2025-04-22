package errors

import (
	"errors"
	"fmt"
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
