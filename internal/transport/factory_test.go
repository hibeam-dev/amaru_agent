package transport

import (
	"context"
	"errors"
	"io"
	"testing"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/util"
	"maps"
)

type mockConnCreator struct {
	createFunc func(ctx context.Context, cfg config.Config, options ConnectionOptions) (Connection, error)
}

func (m *mockConnCreator) CreateConnection(ctx context.Context, cfg config.Config, options ConnectionOptions) (Connection, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, cfg, options)
	}
	return nil, nil
}

type mockConnection struct {
	closeFunc func() error
}

func (m *mockConnection) Stdin() io.WriteCloser {
	return nil
}

func (m *mockConnection) Stdout() io.Reader {
	return nil
}

func (m *mockConnection) Stderr() io.Reader {
	return nil
}

func (m *mockConnection) SendPayload(payload any) error {
	return nil
}

func (m *mockConnection) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockConnection) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockConnection) BinaryInput() io.WriteCloser {
	return nil
}

func (m *mockConnection) BinaryOutput() io.Reader {
	return nil
}

func TestRegisterTransport(t *testing.T) {
	originalRegistry := make(map[string]ConnCreator)
	maps.Copy(originalRegistry, transportRegistry)

	defer func() {
		transportRegistry = make(map[string]ConnCreator)
		maps.Copy(transportRegistry, originalRegistry)
	}()

	transportRegistry = make(map[string]ConnCreator)

	creator := &mockConnCreator{}
	RegisterTransport("mock", creator)

	registeredCreator, ok := transportRegistry["mock"]
	if !ok {
		t.Errorf("Expected 'mock' transport to be registered")
	}

	if registeredCreator != creator {
		t.Errorf("Expected registered creator to be the same as the one we registered")
	}
}

func TestConnect(t *testing.T) {
	originalRegistry := make(map[string]ConnCreator)
	maps.Copy(originalRegistry, transportRegistry)

	defer func() {
		transportRegistry = make(map[string]ConnCreator)
		maps.Copy(transportRegistry, originalRegistry)
	}()

	transportRegistry = make(map[string]ConnCreator)

	t.Run("UnregisteredProtocol", func(t *testing.T) {
		ctx := context.Background()
		cfg := config.Config{}

		conn, err := Connect(ctx, cfg, WithProtocol("nonexistent"))

		if conn != nil {
			t.Errorf("Expected connection to be nil for unregistered protocol")
		}

		if err == nil {
			t.Errorf("Expected error for unregistered protocol")
		}

		var appErr *util.AppError
		if !errors.As(err, &appErr) {
			t.Errorf("Expected app error, got: %v", err)
		}
	})

	t.Run("RegisteredProtocol", func(t *testing.T) {
		mockConn := &mockConnection{}
		creator := &mockConnCreator{
			createFunc: func(ctx context.Context, cfg config.Config, options ConnectionOptions) (Connection, error) {
				if options.Protocol != "mock" {
					t.Errorf("Expected protocol to be 'mock', got %s", options.Protocol)
				}

				if options.Host != "testhost" {
					t.Errorf("Expected host to be 'testhost', got %s", options.Host)
				}

				if options.Port != 1234 {
					t.Errorf("Expected port to be 1234, got %d", options.Port)
				}

				return mockConn, nil
			},
		}

		RegisterTransport("mock", creator)

		ctx := context.Background()
		cfg := config.Config{}

		conn, err := Connect(ctx, cfg, WithProtocol("mock"), WithHost("testhost"), WithPort(1234))

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if conn == nil {
			t.Errorf("Expected connection to be non-nil")
		}
	})

	t.Run("ConnectionError", func(t *testing.T) {
		expectedErr := errors.New("connection error")
		creator := &mockConnCreator{
			createFunc: func(ctx context.Context, cfg config.Config, options ConnectionOptions) (Connection, error) {
				return nil, expectedErr
			},
		}

		RegisterTransport("mock", creator)

		ctx := context.Background()
		cfg := config.Config{}

		conn, err := Connect(ctx, cfg, WithProtocol("mock"))

		if conn != nil {
			t.Errorf("Expected connection to be nil on error")
		}

		if err != expectedErr {
			t.Errorf("Expected error to be %v, got: %v", expectedErr, err)
		}
	})
}
