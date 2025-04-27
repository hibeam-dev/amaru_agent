package service

import (
	"context"
	"sync"
	"time"

	"erlang-solutions.com/cortex_agent/internal/event"
)

type Service struct {
	name         string
	bus          *event.Bus
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	unsubscribes []func()
}

func NewService(name string, bus *event.Bus) Service {
	return Service{
		name:         name,
		bus:          bus,
		unsubscribes: make([]func(), 0),
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	unsubscribes := s.unsubscribes
	s.unsubscribes = nil
	s.mu.Unlock()

	for _, unsub := range unsubscribes {
		if unsub != nil {
			unsub()
		}
	}

	if s.cancel != nil {
		s.cancel()
	}

	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

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
	}
}

func (s *Service) AddSubscription(unsub func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unsubscribes = append(s.unsubscribes, unsub)
}

func (s *Service) Context() context.Context {
	return s.ctx
}
