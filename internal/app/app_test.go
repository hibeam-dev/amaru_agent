package app

import (
	"testing"
)

func TestNewApp(t *testing.T) {
	app := NewApp("test-config.toml")

	if app == nil {
		t.Fatal("Expected app to be non-nil")
	}

	components := map[string]any{
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
