package service

import (
	"context"
	"testing"
	"time"
)

func TestBaseService(t *testing.T) {
	bus := NewEventBus()
	svc := NewBaseService("test", bus)

	ctx := context.Background()
	err := svc.Start(ctx)
	if err != nil {
		t.Errorf("Start returned error: %v", err)
	}

	if svc.Context() == nil {
		t.Error("Context is nil after Start")
	}

	executed := make(chan struct{})
	svc.Go(func() {
		close(executed)
	})

	select {
	case <-executed:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Go function was not executed")
	}

	err = svc.Stop(ctx)
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
}

func TestEventBusPublishAndSubscribe(t *testing.T) {
	bus := NewEventBus()

	ch1 := bus.Subscribe("test_event", 1)
	ch2 := bus.Subscribe("test_event", 1)
	ch3 := bus.Subscribe("other_event", 1)

	bus.Publish(Event{
		Type: "test_event",
		Data: "test data",
	})

	expectEvent(t, ch1, "test_event", "test data")
	expectEvent(t, ch2, "test_event", "test data")
	expectNoEvent(t, ch3)

	bus.Publish(Event{
		Type: "other_event",
		Data: 123,
	})

	expectEvent(t, ch3, "other_event", 123)

	bus.Close()

	if _, ok := <-ch1; ok {
		t.Error("Expected ch1 to be closed")
	}
	if _, ok := <-ch2; ok {
		t.Error("Expected ch2 to be closed")
	}
	if _, ok := <-ch3; ok {
		t.Error("Expected ch3 to be closed")
	}
}

func TestEventBusSubscribeMultiple(t *testing.T) {
	bus := NewEventBus()

	events := []EventType{"event1", "event2", "event3"}
	ch := bus.SubscribeMultiple(events, 3)

	for _, eventType := range events {
		bus.Publish(Event{
			Type: eventType,
			Data: string(eventType) + "_data",
		})
		expectEvent(t, ch, eventType, string(eventType)+"_data")
	}

	// Bus closed in TestEventBusPublishAndSubscribe
}

func expectEvent[T comparable](t *testing.T, ch <-chan Event, expectedType EventType, expectedData T) {
	t.Helper()
	select {
	case event := <-ch:
		if event.Type != expectedType {
			t.Errorf("Expected event type %q, got %q", expectedType, event.Type)
		}
		if data, ok := event.Data.(T); !ok || data != expectedData {
			t.Errorf("Expected event data %v, got %v", expectedData, event.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Timeout waiting for event of type %q", expectedType)
	}
}

func expectNoEvent(t *testing.T, ch <-chan Event) {
	t.Helper()
	select {
	case event := <-ch:
		t.Errorf("Expected no event, but received: %v", event)
	case <-time.After(100 * time.Millisecond):
		// This is expected - no event should be received
	}
}
