package service

import (
	"context"
	"testing"

	"github.com/hibeam-dev/amaru_agent/internal/config"
	"github.com/hibeam-dev/amaru_agent/internal/event"
)

func TestConnectionService(t *testing.T) {
	t.Run("BasicServiceFunctionality", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConnectionService(bus)

		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Errorf("Start should not return error: %v", err)
		}

		if err := svc.Stop(ctx); err != nil {
			t.Errorf("Stop should not return error: %v", err)
		}
	})

	t.Run("BasicConfiguration", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewConnectionService(bus)

		var cfg config.Config
		cfg.Application.Security = map[string]bool{"tls": true}

		svc.SetConfig(cfg)

		retrievedCfg := svc.GetConfig()
		if !retrievedCfg.Application.Security["tls"] {
			t.Error("TLS flag should be true in the config")
		}
	})
}
