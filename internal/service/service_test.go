package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/event"
)

type MockService struct {
	name     string
	bus      *event.Bus
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	executed chan struct{}
}

func NewMockService(name string, bus *event.Bus) *MockService {
	return &MockService{
		name:     name,
		bus:      bus,
		executed: make(chan struct{}),
	}
}

func (s *MockService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		close(s.executed)
	}()

	return nil
}

func (s *MockService) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func TestServiceInterface(t *testing.T) {
	bus := event.NewBus()
	svc := NewMockService("test", bus)

	var _ Service = svc

	ctx := context.Background()
	err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	select {
	case <-svc.executed:
		// Success path
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Function was not executed within timeout")
	}

	err = svc.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}
