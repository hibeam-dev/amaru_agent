package service

import (
	"context"
	"testing"
	"time"

	"erlang-solutions.com/amaru_agent/internal/event"
)

type MockServiceImpl struct {
	Service
	executed chan struct{}
}

func NewMockService(name string, bus *event.Bus) *MockServiceImpl {
	return &MockServiceImpl{
		Service:  NewService(name, bus),
		executed: make(chan struct{}),
	}
}

func (s *MockServiceImpl) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		close(s.executed)
	}()

	return nil
}

func TestServiceImpl(t *testing.T) {
	bus := event.NewBus()
	svc := NewMockService("test", bus)

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
