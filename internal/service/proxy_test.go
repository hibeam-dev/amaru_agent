package service

import (
	"context"
	"testing"

	"github.com/hibeam-dev/amaru_agent/internal/config"
	"github.com/hibeam-dev/amaru_agent/internal/event"
)

func TestProxyService(t *testing.T) {
	t.Run("BasicServiceFunctionality", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewProxyService(bus)

		ctx := context.Background()

		if err := svc.Start(ctx); err != nil {
			t.Errorf("Start should not return error: %v", err)
		}

		if err := svc.Stop(ctx); err != nil {
			t.Errorf("Stop should not return error: %v", err)
		}
	})

	t.Run("ConfigManagement", func(t *testing.T) {
		bus := event.NewBus()
		svc := NewProxyService(bus)

		var cfg config.Config
		cfg.Application.Port = 8080
		cfg.Application.IP = "127.0.0.1"
		cfg.Application.Security = map[string]bool{"tls": true}

		svc.SetConfig(cfg)

		retrievedCfg := svc.GetConfig()
		if retrievedCfg.Application.Port != 8080 {
			t.Errorf("Port should be 8080, got %d", retrievedCfg.Application.Port)
		}

		if retrievedCfg.Application.IP != "127.0.0.1" {
			t.Errorf("IP should be 127.0.0.1, got %s", retrievedCfg.Application.IP)
		}

		if !retrievedCfg.Application.Security["tls"] {
			t.Error("TLS flag should be true in the config")
		}
	})
}
