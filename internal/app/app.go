package app

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"erlang-solutions.com/amaru_agent/internal/config"
	"erlang-solutions.com/amaru_agent/internal/event"
	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/service"
	"erlang-solutions.com/amaru_agent/internal/util"
)

type ServiceProvider interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type ConfigProvider interface {
	ServiceProvider
	LoadConfig() (config.Config, error)
}

type ConnectionProvider interface {
	ServiceProvider
	Connect(context.Context, config.Config) error
	SetConfig(config.Config)
	HasConnection() bool
	GetConfig() config.Config
	Context() context.Context
}

type SignalProvider interface {
	ServiceProvider
}

type EventEmitter interface {
	Publish(event.Event)
	Subscribe(string, event.Handler) func()
	SubscribeMultiple([]string, event.Handler) []func()
	Unsubscribe(func())
}

type ProxyProvider interface {
	ServiceProvider
	SetConfig(config.Config)
}

type App struct {
	configService     ConfigProvider
	connectionService ConnectionProvider
	signalService     SignalProvider
	proxyService      ProxyProvider
	eventBus          EventEmitter
}

func NewApp(configFile string) *App {
	eventBus := event.NewBus()

	return &App{
		eventBus:          eventBus,
		configService:     service.NewConfigService(configFile, eventBus),
		connectionService: service.NewConnectionService(eventBus),
		signalService:     service.NewDefaultSignalService(eventBus),
		proxyService:      service.NewProxyService(eventBus),
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
	a.proxyService.SetConfig(cfg)

	var terminationReceived bool
	terminationSub := a.eventBus.Subscribe(event.TerminationSignal, func(evt event.Event) {
		terminationReceived = true
	})
	defer a.eventBus.Unsubscribe(terminationSub)

	return a.runWithReconnect(mainCtx, &terminationReceived)
}

func (a *App) GetEventBus() EventEmitter {
	return a.eventBus
}

func (a *App) startServices(ctx context.Context) error {
	services := []struct {
		name    string
		service ServiceProvider
	}{
		{"signal", a.signalService},
		{"config", a.configService},
		{"connection", a.connectionService},
		{"proxy", a.proxyService},
	}

	for _, s := range services {
		if err := s.service.Start(ctx); err != nil {
			return util.NewError(util.ErrTypeConfig,
				fmt.Sprintf("failed to start %s service", s.name), err)
		}
	}

	return nil
}

func (a *App) stopServices(ctx context.Context) {
	// Stop in reverse order of startup
	services := []struct {
		name string
		svc  ServiceProvider
	}{
		{"proxy", a.proxyService},
		{"connection", a.connectionService},
		{"config", a.configService},
		{"signal", a.signalService},
	}

	stopCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	for _, s := range services {
		if err := s.svc.Stop(stopCtx); err != nil {
			util.LogError(i18n.T("service_stop_error", map[string]any{
				"Service": s.name,
			}), err)
		}
	}
}

func (a *App) runWithReconnect(ctx context.Context, terminated *bool) error {
	const (
		checkInterval     = 100 * time.Millisecond
		minReconnectDelay = 100 * time.Millisecond
		maxReconnectDelay = 60 * time.Second
	)

	var (
		currentDelay = minReconnectDelay
		backoffMu    sync.Mutex
		resetBackoff = make(chan struct{}, 1)
	)

	resetBackoffFn := func() {
		backoffMu.Lock()
		currentDelay = minReconnectDelay
		backoffMu.Unlock()

		select {
		case resetBackoff <- struct{}{}:
		default:
		}
	}

	nextBackoffDelay := func() time.Duration {
		backoffMu.Lock()
		defer backoffMu.Unlock()

		// Exponential backoff with jitter (±20%)
		delay := currentDelay
		jitter := float64(delay) * 0.2 * (2*rand.Float64() - 1)
		delayWithJitter := delay + time.Duration(jitter)

		currentDelay *= 2
		if currentDelay > maxReconnectDelay {
			currentDelay = maxReconnectDelay
		}

		return delayWithJitter
	}

	connSubs := a.eventBus.SubscribeMultiple(
		[]string{
			event.ConnectionEstablished,
			event.ConnectionClosed,
			event.ConnectionFailed,
			event.ReconnectRequested,
		},
		func(evt event.Event) {
			switch evt.Type {
			case event.ConnectionEstablished:
				resetBackoffFn()
			case event.ConnectionClosed:
				delay := nextBackoffDelay()
				util.Info(i18n.T("connection_reconnecting", map[string]any{
					"Delay": delay.Truncate(time.Millisecond).String(),
				}))
				time.AfterFunc(delay, a.triggerReconnect)
			case event.ConnectionFailed:
				delay := nextBackoffDelay()
				util.Info(i18n.T("connection_reconnecting", map[string]any{
					"Delay": delay.Truncate(time.Millisecond).String(),
				}))
				time.AfterFunc(delay, a.triggerReconnect)
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

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.stopServices(ctx)
			return nil

		case <-resetBackoff:
			ticker.Reset(checkInterval)

		case <-ticker.C:
			if *terminated {
				a.stopServices(ctx)
				return nil
			}

			if !a.connectionService.HasConnection() {
				cfg := a.connectionService.GetConfig()
				if err := a.connectionService.Connect(ctx, cfg); err != nil {
					delay := nextBackoffDelay()
					ticker.Reset(delay)
				} else {
					resetBackoffFn()
				}
			}
		}
	}
}

func (a *App) triggerReconnect() {
	serviceCtx := a.connectionService.Context()
	a.eventBus.Publish(event.Event{
		Type: event.ReconnectRequested,
		Ctx:  serviceCtx,
	})
}
