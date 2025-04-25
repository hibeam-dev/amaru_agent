package event

import (
	"sync"
	"testing"
	"time"
)

func TestEventBusBasicPublishSubscribe(t *testing.T) {
	bus := NewBus()

	var receivedEvent Event
	eventReceived := false

	unsub := bus.Subscribe("test_event", func(evt Event) {
		receivedEvent = evt
		eventReceived = true
	})
	defer unsub()

	bus.Publish(Event{
		Type: "test_event",
		Data: "test_data",
	})

	time.Sleep(10 * time.Millisecond)

	if !eventReceived {
		t.Fatal("Event was not received")
	}

	if receivedEvent.Type != "test_event" {
		t.Errorf("Expected event type %q, got %q", "test_event", receivedEvent.Type)
	}

	if data, ok := receivedEvent.Data.(string); !ok || data != "test_data" {
		t.Errorf("Expected event data %q, got %v", "test_data", receivedEvent.Data)
	}
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := NewBus()

	var wg sync.WaitGroup
	wg.Add(2)

	var event1, event2 Event

	unsub1 := bus.Subscribe("test_event", func(evt Event) {
		event1 = evt
		wg.Done()
	})
	defer unsub1()

	unsub2 := bus.Subscribe("test_event", func(evt Event) {
		event2 = evt
		wg.Done()
	})
	defer unsub2()

	bus.Publish(Event{
		Type: "test_event",
		Data: "test_data",
	})

	if waitTimeout(&wg, 100*time.Millisecond) {
		t.Fatal("Timed out waiting for event handlers to execute")
	}

	if event1.Type != "test_event" || event2.Type != "test_event" {
		t.Error("Event type mismatch in one or more handlers")
	}

	if data1, ok := event1.Data.(string); !ok || data1 != "test_data" {
		t.Errorf("Handler 1: expected data %q, got %v", "test_data", event1.Data)
	}

	if data2, ok := event2.Data.(string); !ok || data2 != "test_data" {
		t.Errorf("Handler 2: expected data %q, got %v", "test_data", event2.Data)
	}
}

func TestEventBusMultipleEventTypes(t *testing.T) {
	bus := NewBus()
	eventTypes := []string{"event1", "event2", "event3"}

	var mu sync.Mutex
	receivedEvents := make(map[string]interface{})

	unsubs := make([]func(), 0, len(eventTypes))
	defer func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}()

	for _, eventType := range eventTypes {
		et := eventType // Capture for closure
		unsub := bus.Subscribe(et, func(evt Event) {
			mu.Lock()
			receivedEvents[evt.Type] = evt.Data
			mu.Unlock()
		})
		unsubs = append(unsubs, unsub)
	}

	for _, eventType := range eventTypes {
		bus.Publish(Event{
			Type: eventType,
			Data: eventType + "_data",
		})
	}

	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedEvents) != len(eventTypes) {
		t.Errorf("Expected %d events, received %d", len(eventTypes), len(receivedEvents))
	}

	for _, eventType := range eventTypes {
		data, exists := receivedEvents[eventType]
		if !exists {
			t.Errorf("Event %q was not received", eventType)
			continue
		}

		expectedData := eventType + "_data"
		if data != expectedData {
			t.Errorf("Event %q: expected data %q, got %v", eventType, expectedData, data)
		}
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	bus := NewBus()

	received := false
	unsub := bus.Subscribe("test_event", func(evt Event) {
		received = true
	})

	unsub() // Unsubscribe immediately

	bus.Publish(Event{
		Type: "test_event",
		Data: "test_data",
	})

	time.Sleep(10 * time.Millisecond)

	if received {
		t.Error("Event handler was called after unsubscribing")
	}
}

func TestEventBusClose(t *testing.T) {
	bus := NewBus()

	received := false
	unsub := bus.Subscribe("test_event", func(evt Event) {
		received = true
	})
	defer unsub()

	bus.Close()

	bus.Publish(Event{
		Type: "test_event",
		Data: "test_data",
	})

	time.Sleep(10 * time.Millisecond)

	if received {
		t.Error("Event handler was called after bus was closed")
	}
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()

	select {
	case <-c:
		return false
	case <-time.After(timeout):
		return true
	}
}
