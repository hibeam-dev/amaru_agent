package event

import (
	"context"
	"fmt"
	"sync"

	"github.com/maniartech/signals"
)

type Handler func(Event)

type Bus struct {
	mu      sync.RWMutex
	signals map[string]signals.Signal[interface{}]
}

func NewBus() *Bus {
	return &Bus{
		signals: make(map[string]signals.Signal[interface{}]),
	}
}

func (b *Bus) Subscribe(eventType string, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.signals[eventType]; !exists {
		b.signals[eventType] = signals.New[interface{}]()
	}

	key := fmt.Sprintf("%p", handler) // Use the handler's pointer address as a unique key

	b.signals[eventType].AddListener(func(ctx context.Context, data interface{}) {
		handler(Event{
			Type: eventType,
			Data: data,
		})
	}, key)

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if signal, exists := b.signals[eventType]; exists {
			signal.RemoveListener(key)
		}
	}
}

func (b *Bus) SubscribeMultiple(eventTypes []string, handler Handler) []func() {
	unsubs := make([]func(), 0, len(eventTypes))

	for _, eventType := range eventTypes {
		unsub := b.Subscribe(eventType, handler)
		unsubs = append(unsubs, unsub)
	}

	return unsubs
}

func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	signal, exists := b.signals[event.Type]
	b.mu.RUnlock()

	if exists {
		signal.Emit(context.Background(), event.Data)
	}
}

func (b *Bus) Unsubscribe(unsubscribe func()) {
	if unsubscribe != nil {
		unsubscribe()
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, signal := range b.signals {
		signal.Reset()
	}
	b.signals = make(map[string]signals.Signal[interface{}])
}
