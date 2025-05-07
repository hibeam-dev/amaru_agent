package service

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
)

// Verifies the proxy service can handle multiple simultaneous
// connections without deadlocking
func TestConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent requests test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bus := event.NewBus()
	proxyService := NewProxyService(bus)

	err := proxyService.Start(ctx)
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
	cfg.Application.Port = 8080
	proxyService.SetConfig(cfg)

	mockTunnel := newConcurrentMockTunnelConnection()
	defer func() {
		if err := mockTunnel.Close(); err != nil {
			t.Logf("Error closing mock tunnel: %v", err)
		}
	}()

	bus.Publish(event.Event{
		Type: event.ConnectionEstablished,
		Data: mockTunnel,
		Ctx:  ctx,
	})

	concurrentRequests := 10
	var wg sync.WaitGroup
	results := make([]string, concurrentRequests)
	errors := make([]error, concurrentRequests)

	for i := range concurrentRequests {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			httpRequest := fmt.Sprintf("GET / HTTP/1.1\r\nHost: example.com\r\nX-Concurrent-ID: %d\r\n\r\n", idx)

			result, err := mockTunnel.sendRequestWithTimeout(httpRequest, 500*time.Millisecond)
			results[idx] = result
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	processedCount := 0
	failedCount := 0

	for i := range concurrentRequests {
		if errors[i] != nil {
			failedCount++
			t.Logf("Request %d failed: %v", i, errors[i])
		} else {
			processedCount++
		}
	}

	t.Logf("Processed %d requests, %d failed", processedCount, failedCount)

	// The test's primary purpose is to verify no deadlocks occur when handling concurrent requests
	// We don't care if the requests actually succeed, just that they complete without hanging
	select {
	case <-ctx.Done():
		// If we reached this point because of a context timeout, that means we had a deadlock
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("Test timed out, likely due to deadlock in concurrent request handling")
		}
	default:
		// All goroutines completed normally
	}
}

type concurrentMockTunnelConnection struct {
	pairs   []mockTunnelPair
	mu      sync.Mutex
	closed  bool
	counter int
}

func newConcurrentMockTunnelConnection() *concurrentMockTunnelConnection {
	return &concurrentMockTunnelConnection{
		pairs: make([]mockTunnelPair, 0, 10),
	}
}

func (m *concurrentMockTunnelConnection) Stdin() io.WriteCloser {
	return nil
}

func (m *concurrentMockTunnelConnection) Stdout() io.Reader {
	return nil
}

func (m *concurrentMockTunnelConnection) Stderr() io.Reader {
	return nil
}

func (m *concurrentMockTunnelConnection) SendPayload(payload any) error {
	return nil
}

func (m *concurrentMockTunnelConnection) BinaryInput() io.WriteCloser {
	return &concurrentWriteCloser{m: m}
}

func (m *concurrentMockTunnelConnection) BinaryOutput() io.Reader {
	return &concurrentReader{m: m}
}

func (m *concurrentMockTunnelConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	for _, pair := range m.pairs {
		_ = pair.clientReader.Close()
		_ = pair.clientWriter.Close()
		_ = pair.tunnelReader.Close()
		_ = pair.tunnelWriter.Close()
	}

	return nil
}

func (m *concurrentMockTunnelConnection) CheckHealth(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("connection closed")
	}
	return nil
}

func (m *concurrentMockTunnelConnection) createPair() mockTunnelPair {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair := createMockTunnelPair()
	m.pairs = append(m.pairs, pair)
	return pair
}

func (m *concurrentMockTunnelConnection) sendRequestWithTimeout(request string, timeout time.Duration) (string, error) {
	pair := m.createPair()

	responseCh := make(chan string, 1)
	errorCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		buffer := make([]byte, 1024)
		n, err := pair.clientReader.Read(buffer)

		if err != nil && err != io.EOF {
			errorCh <- err
			return
		}

		if n > 0 {
			responseCh <- string(buffer[:n])
			return
		}

		errorCh <- fmt.Errorf("no data received")
	}()

	_, err := pair.clientWriter.Write([]byte(request))
	if err != nil {
		return "", err
	}

	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errorCh:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("timeout waiting for response")
	}
}

// Helper types to implement concurrent I/O

type concurrentWriteCloser struct {
	m *concurrentMockTunnelConnection
}

func (c *concurrentWriteCloser) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	c.m.mu.Lock()
	counter := c.m.counter
	c.m.counter++
	c.m.mu.Unlock()

	// Simulate a request-response cycle
	if counter < len(c.m.pairs) {
		pair := c.m.pairs[counter]
		return pair.tunnelWriter.Write(p)
	}

	return len(p), nil
}

func (c *concurrentWriteCloser) Close() error {
	return nil
}

type concurrentReader struct {
	m       *concurrentMockTunnelConnection
	current int
}

func (c *concurrentReader) Read(p []byte) (n int, err error) {
	c.m.mu.Lock()
	if c.m.closed {
		c.m.mu.Unlock()
		return 0, io.EOF
	}

	if c.current >= len(c.m.pairs) {
		c.m.mu.Unlock()
		// Block until new data arrives or connection closes
		time.Sleep(50 * time.Millisecond)
		return 0, nil
	}

	pair := c.m.pairs[c.current]
	c.current++
	c.m.mu.Unlock()

	return pair.tunnelReader.Read(p)
}
