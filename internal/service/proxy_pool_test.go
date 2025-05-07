package service

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
)

func TestProxyPoolManagement(t *testing.T) {
	bus := event.NewBus()
	proxyService := NewProxyService(bus)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxyService.Start(ctx); err != nil {
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

	t.Run("PoolInitialization", func(t *testing.T) {
		pool := proxyService.connPool

		size := pool.GetPoolSize()
		if size != poolSize {
			t.Errorf("Expected pool size %d, got %d", poolSize, size)
		}

		total, inUse, empty := pool.GetPoolStatus()
		if total != poolSize {
			t.Errorf("Expected total pool slots %d, got %d", poolSize, total)
		}

		if inUse != 0 {
			t.Errorf("Expected 0 in-use connections, got %d", inUse)
		}

		if empty != poolSize {
			t.Errorf("Expected %d empty slots, got %d", poolSize, empty)
		}
	})

	t.Run("GetPoolConnection", func(t *testing.T) {
		_, idx := proxyService.connPool.GetConnection(proxyService.GetConfig())

		// Should fail because the port is likely not open
		if idx != -1 {
			t.Errorf("Expected no connection (idx=-1), got idx=%d", idx)
		}
	})

	t.Run("CleanupPool", func(t *testing.T) {
		mockConn := &mockNetConn{}

		proxyService.connPool.SetPoolConnection(0, mockConn, false)

		_, _, emptyAfterAdd := proxyService.connPool.GetPoolStatus()
		if emptyAfterAdd == poolSize {
			t.Fatal("Failed to add test connection to pool")
		}

		proxyService.connPool.CleanupPool()

		_, _, emptyAfterCleanup := proxyService.connPool.GetPoolStatus()
		if emptyAfterCleanup != poolSize {
			t.Errorf("Expected all slots to be empty after cleanup, got %d empty out of %d",
				emptyAfterCleanup, poolSize)
		}

		if !mockConn.closed {
			t.Error("Connection not properly closed during cleanup")
		}
	})
}

func TestConnectionPoolConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	bus := event.NewBus()
	proxyService := NewProxyService(bus)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := proxyService.Start(ctx); err != nil {
		t.Fatalf("Failed to start proxy service: %v", err)
	}
	defer func() {
		if err := proxyService.Stop(ctx); err != nil {
			t.Logf("Error stopping proxy service: %v", err)
		}
	}()

	for i := range poolSize {
		proxyService.connPool.SetPoolConnection(i, &mockNetConn{id: i}, false)
	}

	var wg sync.WaitGroup
	concurrentWorkers := 10
	operationsPerWorker := 5

	// Test concurrent access to the connection pool
	for w := range concurrentWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for range operationsPerWorker {
				conn, idx := proxyService.connPool.GetConnection(proxyService.GetConfig())

				if idx >= 0 && conn != nil {
					proxyService.connPool.ReleaseConnection(idx)
				}
			}
		}(w)
	}

	wg.Wait()

	_, inUseAfterTest, _ := proxyService.connPool.GetPoolStatus()

	if inUseAfterTest > 0 {
		t.Errorf("Found %d connections still marked as inUse after test", inUseAfterTest)
	}
}

type mockNetConn struct {
	id     int
	closed bool
	mu     sync.Mutex
}

func (m *mockNetConn) Read(b []byte) (n int, err error)  { return 0, nil }
func (m *mockNetConn) Write(b []byte) (n int, err error) { return len(b), nil }
func (m *mockNetConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
func (m *mockNetConn) LocalAddr() net.Addr                { return nil }
func (m *mockNetConn) RemoteAddr() net.Addr               { return nil }
func (m *mockNetConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockNetConn) SetWriteDeadline(t time.Time) error { return nil }
