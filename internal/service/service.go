package service

import (
	"context"
	"sync"
)

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type BaseService struct {
	name   string
	bus    *EventBus
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewBaseService(name string, bus *EventBus) BaseService {
	return BaseService{
		name: name,
		bus:  bus,
	}
}

func (s *BaseService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return nil
}

func (s *BaseService) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *BaseService) Context() context.Context {
	return s.ctx
}

func (s *BaseService) Go(f func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		f()
	}()
}

type EventType string

const (
	ConfigUpdated         EventType = "config_updated"
	ConnectionEstablished EventType = "connection_established"
	ConnectionClosed      EventType = "connection_closed"
	ConnectionFailed      EventType = "connection_failed"
	ReconnectRequested    EventType = "reconnect_requested"
	TerminationSignal     EventType = "termination_signal"
)

type Event struct {
	Type EventType
	Data interface{}
}

type EventBus struct {
	listeners map[EventType][]chan Event
	mu        sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[EventType][]chan Event),
	}
}

func (b *EventBus) Subscribe(eventType EventType, bufferSize int) <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, bufferSize)
	b.listeners[eventType] = append(b.listeners[eventType], ch)
	return ch
}

func (b *EventBus) SubscribeMultiple(eventTypes []EventType, bufferSize int) <-chan Event {
	ch := make(chan Event, bufferSize)

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, eventType := range eventTypes {
		b.listeners[eventType] = append(b.listeners[eventType], ch)
	}

	return ch
}

func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	listeners, ok := b.listeners[event.Type]
	b.mu.RUnlock()

	if !ok {
		return
	}

	for _, ch := range listeners {
		select {
		case ch <- event:
		default:
			// Channel full, skip this
		}
	}
}

func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, channels := range b.listeners {
		for _, ch := range channels {
			close(ch)
		}
	}
	b.listeners = make(map[EventType][]chan Event)
}
