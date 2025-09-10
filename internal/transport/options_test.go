package transport

import (
	"testing"
	"time"
)

func TestWithProtocol(t *testing.T) {
	opt := WithProtocol("custom")
	opts := DefaultOptions()
	opt(&opts)

	if opts.Protocol != "custom" {
		t.Errorf("Expected protocol to be 'custom', got '%s'", opts.Protocol)
	}
}

func TestWithHost(t *testing.T) {
	opt := WithHost("example.com")
	opts := DefaultOptions()
	opt(&opts)

	if opts.Host != "example.com" {
		t.Errorf("Expected host to be 'example.com', got '%s'", opts.Host)
	}
}

func TestWithPort(t *testing.T) {
	opt := WithPort(8080)
	opts := DefaultOptions()
	opt(&opts)

	if opts.Port != 8080 {
		t.Errorf("Expected port to be 8080, got %d", opts.Port)
	}
}
func TestWithKeyFile(t *testing.T) {
	opt := WithKeyFile("/path/to/key.pem")
	opts := DefaultOptions()
	opt(&opts)

	if opts.KeyFile != "/path/to/key.pem" {
		t.Errorf("Expected key file to be '/path/to/key.pem', got '%s'", opts.KeyFile)
	}
}

func TestWithTimeout(t *testing.T) {
	timeout := 10 * time.Second
	opt := WithTimeout(timeout)
	opts := DefaultOptions()
	opt(&opts)

	if opts.Timeout != timeout {
		t.Errorf("Expected timeout to be %v, got %v", timeout, opts.Timeout)
	}
}


func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Protocol != DefaultProtocol {
		t.Errorf("Expected protocol to be '%s', got '%s'", DefaultProtocol, opts.Protocol)
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", opts.Timeout)
	}
}

func TestApplyOptions(t *testing.T) {
	opts := ApplyOptions(
		WithProtocol("custom"),
		WithHost("example.com"),
		WithPort(8080),
		WithKeyFile("/path/to/key.pem"),
		WithTimeout(10*time.Second),
	)

	if opts.Protocol != "custom" {
		t.Errorf("Expected protocol to be 'custom', got '%s'", opts.Protocol)
	}

	if opts.Host != "example.com" {
		t.Errorf("Expected host to be 'example.com', got '%s'", opts.Host)
	}

	if opts.Port != 8080 {
		t.Errorf("Expected port to be 8080, got %d", opts.Port)
	}

	if opts.KeyFile != "/path/to/key.pem" {
		t.Errorf("Expected key file to be '/path/to/key.pem', got '%s'", opts.KeyFile)
	}

	if opts.Timeout != 10*time.Second {
		t.Errorf("Expected timeout to be 10s, got %v", opts.Timeout)
	}
}

func TestApplyOptionsOrder(t *testing.T) {
	opts := ApplyOptions(
		WithProtocol("first"),
		WithProtocol("second"),
		WithProtocol("third"),
	)

	if opts.Protocol != "third" {
		t.Errorf("Expected protocol to be 'third', got '%s'", opts.Protocol)
	}
}
