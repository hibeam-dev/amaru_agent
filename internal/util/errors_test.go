package util

import (
	"errors"
	"testing"
)

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
