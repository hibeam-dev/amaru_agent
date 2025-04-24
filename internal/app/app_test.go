package app

import (
	"testing"
)

func TestNewApp(t *testing.T) {
	app := NewApp("test-config.toml", true)

	if app == nil {
		t.Fatal("Expected app to be non-nil")
	}

	components := map[string]interface{}{
		"configService":     app.configService,
		"connectionService": app.connectionService,
		"signalService":     app.signalService,
		"eventBus":          app.eventBus,
	}

	for name, component := range components {
		if component == nil {
			t.Errorf("Expected %s to be non-nil", name)
		}
	}
}

func TestServices(t *testing.T) {
	t.Run("StartServices", func(t *testing.T) {
		t.Skip("Need to implement mock services")
	})

	t.Run("StopServices", func(t *testing.T) {
		t.Skip("Need to implement mock services")
	})

	t.Run("RunOnce", func(t *testing.T) {
		t.Skip("Need to implement mock ConnectionService")
	})

	t.Run("RunWithReconnect", func(t *testing.T) {
		t.Skip("Need to implement mock services")
	})

	t.Run("AppRunCancel", func(t *testing.T) {
		t.Skip("Need to implement mock services")
	})

	t.Run("TriggerReconnect", func(t *testing.T) {
		t.Skip("Need to implement mock eventBus")
	})
}
