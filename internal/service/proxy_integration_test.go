package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
)

type mockTunnelPair struct {
	clientReader io.ReadCloser
	clientWriter io.WriteCloser
	tunnelReader io.ReadCloser
	tunnelWriter io.WriteCloser
}

func createMockPipe() (io.ReadCloser, io.WriteCloser) {
	r, w := io.Pipe()
	return r, w
}

func createMockTunnelPair() mockTunnelPair {
	clientToTunnel_Reader, clientToTunnel_Writer := createMockPipe()
	tunnelToClient_Reader, tunnelToClient_Writer := createMockPipe()

	return mockTunnelPair{
		clientReader: tunnelToClient_Reader, // Client reads from tunnel
		clientWriter: clientToTunnel_Writer, // Client writes to tunnel
		tunnelReader: clientToTunnel_Reader, // Tunnel reads from client
		tunnelWriter: tunnelToClient_Writer, // Tunnel writes to client
	}
}

type mockTunnelConnection struct {
	pipes  mockTunnelPair
	mu     sync.Mutex
	closed bool
}

func newMockTunnelConnection() *mockTunnelConnection {
	return &mockTunnelConnection{
		pipes: createMockTunnelPair(),
	}
}

func (m *mockTunnelConnection) Stdin() io.WriteCloser {
	return nil
}

func (m *mockTunnelConnection) Stdout() io.Reader {
	return nil
}

func (m *mockTunnelConnection) Stderr() io.Reader {
	return nil
}

func (m *mockTunnelConnection) SendPayload(payload any) error {
	return nil
}

func (m *mockTunnelConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	_ = m.pipes.tunnelReader.Close()
	_ = m.pipes.tunnelWriter.Close()
	_ = m.pipes.clientReader.Close()
	_ = m.pipes.clientWriter.Close()

	return nil
}

func (m *mockTunnelConnection) CheckHealth(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("connection closed")
	}
	return nil
}

func (m *mockTunnelConnection) BinaryInput() io.WriteCloser {
	return m.pipes.tunnelWriter
}

func (m *mockTunnelConnection) BinaryOutput() io.Reader {
	return m.pipes.tunnelReader
}

func simulateHttpRequest(t *testing.T, pipes mockTunnelPair, request string, timeout time.Duration) string {
	responseCh := make(chan string, 1)
	errorCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		var responseBuffer strings.Builder
		buffer := make([]byte, 1024)

		for {
			if deadline, ok := ctx.Deadline(); ok {
				if reader, canSetDeadline := pipes.clientReader.(interface{ SetReadDeadline(time.Time) error }); canSetDeadline {
					_ = reader.SetReadDeadline(deadline)
				}
			}

			n, err := pipes.clientReader.Read(buffer)
			if n > 0 {
				responseBuffer.Write(buffer[:n])

				if strings.Contains(responseBuffer.String(), "HTTP/1.1 200 OK") &&
					(strings.Contains(responseBuffer.String(), "\r\n\r\n") ||
						responseBuffer.Len() > 256) {
					responseCh <- responseBuffer.String()
					return
				}
			}

			if err != nil {
				if (err == io.EOF || isTimeout(err)) && responseBuffer.Len() > 0 {
					responseCh <- responseBuffer.String()
					return
				}

				errorCh <- err
				return
			}
		}
	}()

	_, err := pipes.clientWriter.Write([]byte(request))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	select {
	case response := <-responseCh:
		return response
	case err := <-errorCh:
		t.Fatalf("Error reading response: %v", err)
		return ""
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for response")
		return ""
	}
}

func isTimeout(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}

func TestProxySequentialRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		seqID := r.Header.Get("X-Sequence-ID")
		_, err := fmt.Fprintf(w, "Response for sequence %s", seqID)
		if err != nil {
			t.Logf("Error writing response: %v", err)
		}
	}))
	defer testServer.Close()

	_, portStr, _ := net.SplitHostPort(testServer.Listener.Addr().String())
	serverPort := 0
	var err error
	_, err = fmt.Sscanf(portStr, "%d", &serverPort)
	if err != nil {
		t.Fatalf("Failed to parse port number: %v", err)
	}

	bus := event.NewBus()

	proxyService := NewProxyService(bus)
	err = proxyService.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start proxy service: %v", err)
	}
	defer func() {
		if err := proxyService.Stop(ctx); err != nil {
			t.Logf("Error stopping proxy service: %v", err)
		}
	}()

	var cfg config.Config
	cfg.Application.IP = "127.0.0.1"
	cfg.Application.Port = serverPort
	proxyService.SetConfig(cfg)

	mockTunnel := newMockTunnelConnection()
	defer func() {
		if err := mockTunnel.Close(); err != nil {
			t.Logf("Error closing mock tunnel connection: %v", err)
		}
	}()

	bus.Publish(event.Event{
		Type: event.ConnectionEstablished,
		Data: mockTunnel,
		Ctx:  ctx,
	})

	time.Sleep(100 * time.Millisecond)

	for i := 1; i <= 10; i++ {
		httpRequest := fmt.Sprintf("GET / HTTP/1.1\r\nHost: example.com\r\nX-Sequence-ID: %d\r\n\r\n", i)
		expectedResponse := fmt.Sprintf("Response for sequence %d", i)

		response := simulateHttpRequest(t, mockTunnel.pipes, httpRequest, 2*time.Second)

		if !strings.Contains(response, expectedResponse) {
			t.Errorf("Request %d failed: expected '%s' in response, got: %s",
				i, expectedResponse, response)
		}

		time.Sleep(20 * time.Millisecond)
	}
}
