package service

import (
	"context"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/event"
)

func TestSignalService(t *testing.T) {
	t.Run("Constructor", func(t *testing.T) {
		bus := event.NewBus()
		customTimeout := 10 * time.Second

		svc1 := NewSignalService(customTimeout, bus)
		if svc1.gracefulTimeout != customTimeout {
			t.Errorf("Expected gracefulTimeout to be %v, got %v", customTimeout, svc1.gracefulTimeout)
		}

		svc2 := NewDefaultSignalService(bus)
		if svc2.gracefulTimeout != DefaultGracefulTimeout {
			t.Errorf("Expected default gracefulTimeout to be %v, got %v", DefaultGracefulTimeout, svc2.gracefulTimeout)
		}
	})

	t.Run("ServiceLifecycle", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewDefaultSignalService(bus)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Expected no error from Start(), got: %v", err)
		}

		if err := svc.Stop(ctx); err != nil {
			t.Fatalf("Expected no error from Stop(), got: %v", err)
		}
	})
}
