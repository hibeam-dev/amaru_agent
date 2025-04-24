package errors

import (
	"errors"
	"testing"
)

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := Wrap(originalErr, "wrapped message")

	if wrappedErr == nil {
		t.Fatal("Expected non-nil error, got nil")
	}

	if wrappedErr.Error() != "wrapped message: original error" {
		t.Errorf("Expected 'wrapped message: original error', got '%s'", wrappedErr.Error())
	}

	unwrappedErr := errors.Unwrap(wrappedErr)
	if unwrappedErr != originalErr {
		t.Errorf("Expected unwrapped error to be original error")
	}
}

func TestWrapWithBase(t *testing.T) {
	baseErr := errors.New("base error")
	causeErr := errors.New("cause error")
	result := WrapWithBase(baseErr, "context message", causeErr)

	if result == nil {
		t.Fatal("Expected non-nil error, got nil")
	}

	expectedMsg := "base error: context message: cause error"
	if result.Error() != expectedMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, result.Error())
	}

	if !errors.Is(result, baseErr) {
		t.Error("Expected result to contain baseErr")
	}
}

func TestPredefinedErrors(t *testing.T) {
	testCases := []struct {
		err  error
		want string
	}{
		{ErrConfigLoad, "config load failed"},
		{ErrConnectionFailed, "connection failed"},
		{ErrSessionFailed, "session failed"},
		{ErrSubsystemFailed, "subsystem failed"},
	}

	for _, tc := range testCases {
		if tc.err.Error() != tc.want {
			t.Errorf("Error message for %v: got %q, want %q", tc.err, tc.err.Error(), tc.want)
		}
	}
}
