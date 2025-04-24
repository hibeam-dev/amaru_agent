package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/service"
)

type App struct {
	configService     *service.ConfigService
	connectionService *service.ConnectionService
	signalService     *service.SignalService
	eventBus          *service.EventBus
}

func NewApp(configFile string, jsonMode bool) *App {
	eventBus := service.NewEventBus()

	return &App{
		eventBus:          eventBus,
		configService:     service.NewConfigService(configFile, eventBus),
		connectionService: service.NewConnectionService(jsonMode, eventBus),
		signalService:     service.NewDefaultSignalService(eventBus),
	}
}

func (a *App) Run(ctx context.Context) error {
	mainCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := a.startServices(mainCtx); err != nil {
		return err
	}

	cfg, err := a.configService.LoadConfig()
	if err != nil {
		return err
	}

	a.connectionService.SetConfig(cfg)
	termCh := a.eventBus.Subscribe(service.TerminationSignal, 1)

	if a.connectionService.IsJSONMode() {
		return a.runOnce(mainCtx, cfg)
	}
	return a.runWithReconnect(mainCtx, termCh)
}

func (a *App) startServices(ctx context.Context) error {
	services := []struct {
		name    string
		service service.Service
	}{
		{"signal", a.signalService},
		{"config", a.configService},
		{"connection", a.connectionService},
	}

	for _, s := range services {
		if err := s.service.Start(ctx); err != nil {
			return fmt.Errorf("failed to start %s service: %w", s.name, err)
		}
	}

	return nil
}

func (a *App) stopServices(ctx context.Context) {
	// Stop in reverse order of startup
	services := []struct {
		name string
		svc  service.Service
	}{
		{"connection", a.connectionService},
		{"config", a.configService},
		{"signal", a.signalService},
	}

	for _, s := range services {
		if err := s.svc.Stop(ctx); err != nil {
			// Just log errors
			log.Printf("%s", i18n.T("service_stop_error", map[string]interface{}{
				"Service": s.name,
				"Error":   err,
			}))
		}
	}
}

func (a *App) runOnce(ctx context.Context, cfg config.Config) error {
	if err := a.connectionService.Connect(ctx, cfg); err != nil {
		return err
	}

	<-ctx.Done()
	a.stopServices(context.Background())
	return nil
}

func (a *App) runWithReconnect(ctx context.Context, termCh <-chan service.Event) error {
	connEvents := a.eventBus.SubscribeMultiple([]service.EventType{
		service.ConnectionEstablished,
		service.ConnectionClosed,
		service.ConnectionFailed,
		service.ReconnectRequested,
	}, 4)

	const (
		shortDelay = 100 * time.Millisecond
		longDelay  = 5 * time.Second
	)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.stopServices(context.Background())
			return nil

		case <-termCh:
			a.stopServices(context.Background())
			return nil

		case event := <-connEvents:
			switch event.Type {
			case service.ConnectionEstablished:
				// Connection established, nothing to do
			case service.ConnectionClosed:
				time.Sleep(shortDelay)
				a.triggerReconnect()
			case service.ConnectionFailed:
				time.Sleep(longDelay)
				a.triggerReconnect()
			case service.ReconnectRequested:
				a.triggerReconnect()
			}

		case <-ticker.C:
			if !a.connectionService.HasConnection() {
				cfg := a.connectionService.GetConfig()
				if err := a.connectionService.Connect(ctx, cfg); err != nil {
					time.Sleep(longDelay)
				}
			}
		}
	}
}

func (a *App) triggerReconnect() {
	a.eventBus.Publish(service.Event{Type: service.ReconnectRequested})
}
