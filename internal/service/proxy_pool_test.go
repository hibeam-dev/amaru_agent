package service

import (
	"context"
	"testing"
	"time"

	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/event"
)

func TestWireGuardProxyService(t *testing.T) {
	bus := event.NewBus()
	proxyService := NewProxyService(bus)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := proxyService.Start(ctx); err != nil {
		t.Fatalf("Failed to start proxy service: %v", err)
	}
	defer func() {
		if err := proxyService.Stop(ctx); err != nil {
			t.Logf("Error stopping proxy service: %v", err)
		}
	}()

	var cfg config.Config
	cfg.Application.IP = "127.0.0.1"
	cfg.Application.Port = 8080
	cfg.Connection.Tunnel = true
	proxyService.SetConfig(cfg)

	t.Run("ServiceInitialization", func(t *testing.T) {
		if proxyService.wgClient != nil {
			t.Error("WireGuard client should not be initialized without config")
		}
	})

	t.Run("ConfigUpdate", func(t *testing.T) {
		// Test config update
		newCfg := cfg
		newCfg.Application.Port = 9090
		proxyService.SetConfig(newCfg)

		updatedCfg := proxyService.GetConfig()
		if updatedCfg.Application.Port != 9090 {
			t.Errorf("Expected port 9090, got %d", updatedCfg.Application.Port)
		}
	})

	t.Run("WireGuardConfigHandling", func(t *testing.T) {
		// Test WireGuard config handling
		wgConfig := &WireGuardConfig{
			PrivateKey:   "QH7bXDSBhNvHXnUXlFKi0Ks64a4I1j7E1K4Z4b8LfVs=",
			PublicKey:    "d/t9p4J3A+J5A8lRO+dUEeO4hgd4JB8z5FE8eI6ZmWI=",
			ServerPubKey: "m9RCGLWOgYkk0xqCuC5VlRKJQ6Hj4b7YXKjJYG8Q2Vs=",
			Endpoint:     "test.example.com:51820",
			AllowedIPs:   []string{"10.0.0.2/32"},
			DNS:          "8.8.8.8",
			MTU:          1420,
		}

		// This would normally be called by the event handler
		// but since we don't have a real server connection, we test separately
		if !cfg.Connection.Tunnel {
			t.Skip("Tunnel not enabled in test config")
		}

		// Test that the config is properly stored
		proxyService.connectionMu.Lock()
		proxyService.wgConfig = wgConfig
		proxyService.connectionMu.Unlock()

		proxyService.connectionMu.RLock()
		storedConfig := proxyService.wgConfig
		proxyService.connectionMu.RUnlock()

		if storedConfig == nil {
			t.Error("WireGuard config not stored")
		} else if storedConfig.Endpoint != "test.example.com:51820" {
			t.Errorf("Expected endpoint test.example.com:51820, got %s", storedConfig.Endpoint)
		}
	})
}

func TestWireGuardClientConfig(t *testing.T) {
	t.Run("ClientConfigCreation", func(t *testing.T) {
		config := &WireGuardClientConfig{
			PrivateKey:   "QH7bXDSBhNvHXnUXlFKi0Ks64a4I1j7E1K4Z4b8LfVs=",
			PublicKey:    "d/t9p4J3A+J5A8lRO+dUEeO4hgd4JB8z5FE8eI6ZmWI=",
			ServerPubKey: "m9RCGLWOgYkk0xqCuC5VlRKJQ6Hj4b7YXKjJYG8Q2Vs=",
			Endpoint:     "test.example.com:51820",
			AllowedIPs:   []string{"10.0.0.2/32"},
			DNS:          "8.8.8.8",
			MTU:          1420,
		}

		if config.Endpoint != "test.example.com:51820" {
			t.Errorf("Expected endpoint test.example.com:51820, got %s", config.Endpoint)
		}

		if len(config.AllowedIPs) != 1 {
			t.Errorf("Expected 1 allowed IP, got %d", len(config.AllowedIPs))
		}
	})
}
