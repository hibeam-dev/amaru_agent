package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/service"

	// Import SSH package to register it
	_ "erlang-solutions.com/cortex_agent/internal/ssh"
)

type App struct {
	configService     *service.ConfigService
	connectionService *service.ConnectionService
	signalService     *service.SignalService
	eventBus          *event.Bus
}

func NewApp(configFile string, jsonMode bool) *App {
	eventBus := event.NewBus()

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

	var terminationReceived bool
	terminationSub := a.eventBus.Subscribe(event.TerminationSignal, func(evt event.Event) {
		terminationReceived = true
	})
	defer a.eventBus.Unsubscribe(terminationSub)

	if a.connectionService.IsJSONMode() {
		return a.runOnce(mainCtx, cfg)
	}
	return a.runWithReconnect(mainCtx, &terminationReceived)
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

	stopCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	for _, s := range services {
		if err := s.svc.Stop(stopCtx); err != nil {
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

func (a *App) runWithReconnect(ctx context.Context, terminated *bool) error {
	const (
		shortDelay = 100 * time.Millisecond
		longDelay  = 5 * time.Second
	)

	connSubs := a.eventBus.SubscribeMultiple(
		[]string{
			event.ConnectionEstablished,
			event.ConnectionClosed,
			event.ConnectionFailed,
			event.ReconnectRequested,
		},
		func(evt event.Event) {
			switch evt.Type {
			case event.ConnectionClosed:
				time.Sleep(shortDelay)
				a.triggerReconnect()
			case event.ConnectionFailed:
				time.Sleep(longDelay)
				a.triggerReconnect()
			case event.ReconnectRequested:
				a.triggerReconnect()
			}
		},
	)

	// Clean up all subscriptions when done
	defer func() {
		for _, unsub := range connSubs {
			unsub()
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.stopServices(context.Background())
			return nil

		case <-ticker.C:
			if *terminated {
				a.stopServices(context.Background())
				return nil
			}

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
	a.eventBus.Publish(event.Event{Type: event.ReconnectRequested})
}
