package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
)

type mockService struct {
	startFunc func(context.Context) error
	stopFunc  func(context.Context) error
}

func (m *mockService) Start(ctx context.Context) error {
	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	return nil
}

func (m *mockService) Stop(ctx context.Context) error {
	if m.stopFunc != nil {
		return m.stopFunc(ctx)
	}
	return nil
}

type mockConfigService struct {
	mockService
	loadConfigFunc func() (config.Config, error)
}

func (m *mockConfigService) LoadConfig() (config.Config, error) {
	if m.loadConfigFunc != nil {
		return m.loadConfigFunc()
	}
	return config.Config{}, nil
}

type mockConnectionService struct {
	mockService
	connectFunc    func(context.Context, config.Config) error
	setConfigFunc  func(config.Config)
	hasConnFunc    func() bool
	getConfigFunc  func() config.Config
	contextFunc    func() context.Context
	cfg            config.Config
	hasConnection  bool
	serviceContext context.Context
}

func (m *mockConnectionService) Connect(ctx context.Context, cfg config.Config) error {
	if m.connectFunc != nil {
		return m.connectFunc(ctx, cfg)
	}
	return nil
}

func (m *mockConnectionService) SetConfig(cfg config.Config) {
	if m.setConfigFunc != nil {
		m.setConfigFunc(cfg)
	}
	m.cfg = cfg
}

func (m *mockConnectionService) HasConnection() bool {
	if m.hasConnFunc != nil {
		return m.hasConnFunc()
	}
	return m.hasConnection
}

func (m *mockConnectionService) GetConfig() config.Config {
	if m.getConfigFunc != nil {
		return m.getConfigFunc()
	}
	return m.cfg
}

func (m *mockConnectionService) Context() context.Context {
	if m.contextFunc != nil {
		return m.contextFunc()
	}
	return m.serviceContext
}

type mockSignalService struct {
	mockService
}

type mockProxyService struct {
	mockService
	setConfigFunc func(config.Config)
}

func (m *mockProxyService) SetConfig(cfg config.Config) {
	if m.setConfigFunc != nil {
		m.setConfigFunc(cfg)
	}
}

type mockEventBus struct {
	mu            sync.Mutex
	handlers      map[string][]event.Handler
	events        []event.Event
	subscriptions []func()
	subCounter    int
}

func newMockEventBus() *mockEventBus {
	return &mockEventBus{
		handlers:      make(map[string][]event.Handler),
		subscriptions: make([]func(), 0),
	}
}

func (m *mockEventBus) Publish(evt event.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, evt)

	for eventType, handlers := range m.handlers {
		if eventType == evt.Type {
			for _, handler := range handlers {
				go handler(evt)
			}
		}
	}
}

func (m *mockEventBus) Subscribe(eventType string, handler event.Handler) func() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.handlers[eventType]; !exists {
		m.handlers[eventType] = make([]event.Handler, 0)
	}

	m.handlers[eventType] = append(m.handlers[eventType], handler)
	m.subCounter++

	unsubFunc := func() {}

	m.subscriptions = append(m.subscriptions, unsubFunc)
	return unsubFunc
}

func (m *mockEventBus) SubscribeMultiple(eventTypes []string, handler event.Handler) []func() {
	m.mu.Lock()
	defer m.mu.Unlock()

	unsubs := make([]func(), 0, len(eventTypes))

	for _, eventType := range eventTypes {
		if _, exists := m.handlers[eventType]; !exists {
			m.handlers[eventType] = make([]event.Handler, 0)
		}

		m.handlers[eventType] = append(m.handlers[eventType], handler)
		m.subCounter++

		unsubFunc := func() {}

		m.subscriptions = append(m.subscriptions, unsubFunc)
		unsubs = append(unsubs, unsubFunc)
	}

	return unsubs
}

func (m *mockEventBus) Unsubscribe(unsub func()) {
	// For testing we can just track that it was called
}

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

func TestStartServices(t *testing.T) {
	bus := newMockEventBus()
	app := &App{
		eventBus:          bus,
		configService:     &mockConfigService{},
		connectionService: &mockConnectionService{},
		signalService:     &mockSignalService{},
		proxyService:      &mockProxyService{},
	}

	ctx := context.Background()
	err := app.startServices(ctx)
	if err != nil {
		t.Errorf("Expected startServices to succeed, got error: %v", err)
	}

	failApp := &App{
		eventBus: bus,
		configService: &mockConfigService{
			mockService: mockService{
				startFunc: func(ctx context.Context) error {
					return errors.New("start failure")
				},
			},
		},
		connectionService: &mockConnectionService{},
		signalService:     &mockSignalService{},
		proxyService:      &mockProxyService{},
	}

	err = failApp.startServices(ctx)
	if err == nil {
		t.Errorf("Expected startServices to fail when a service fails to start")
	}
}

func TestStopServices(t *testing.T) {
	var stoppedServices []string

	bus := newMockEventBus()
	app := &App{
		eventBus: bus,
		configService: &mockConfigService{
			mockService: mockService{
				stopFunc: func(ctx context.Context) error {
					stoppedServices = append(stoppedServices, "config")
					return nil
				},
			},
		},
		connectionService: &mockConnectionService{
			mockService: mockService{
				stopFunc: func(ctx context.Context) error {
					stoppedServices = append(stoppedServices, "connection")
					return nil
				},
			},
		},
		signalService: &mockSignalService{
			mockService: mockService{
				stopFunc: func(ctx context.Context) error {
					stoppedServices = append(stoppedServices, "signal")
					return nil
				},
			},
		},
		proxyService: &mockProxyService{
			mockService: mockService{
				stopFunc: func(ctx context.Context) error {
					stoppedServices = append(stoppedServices, "proxy")
					return nil
				},
			},
		},
	}

	ctx := context.Background()
	app.stopServices(ctx)

	expectedOrder := []string{"proxy", "connection", "config", "signal"}
	if len(stoppedServices) != len(expectedOrder) {
		t.Errorf("Expected %d services to be stopped, got %d", len(expectedOrder), len(stoppedServices))
	}

	for i, service := range stoppedServices {
		if i >= len(expectedOrder) || service != expectedOrder[i] {
			t.Errorf("Expected service %s to be stopped at position %d, got %s", expectedOrder[i], i, service)
		}
	}
}

func TestRunWithReconnect(t *testing.T) {
	t.Run("HandleTermination", func(t *testing.T) {
		bus := newMockEventBus()
		connectionCtx := t.Context()

		app := &App{
			eventBus: bus,
			connectionService: &mockConnectionService{
				serviceContext: connectionCtx,
				hasConnection:  true,
			},
			configService: &mockConfigService{},
			signalService: &mockSignalService{},
			proxyService:  &mockProxyService{},
		}

		ctx := t.Context()

		terminated := true

		done := make(chan struct{})
		go func() {
			_ = app.runWithReconnect(ctx, &terminated)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Errorf("runWithReconnect did not exit after termination flag was set")
		}
	})

	t.Run("HandleContextCancellation", func(t *testing.T) {
		bus := newMockEventBus()
		connectionCtx := t.Context()

		app := &App{
			eventBus: bus,
			connectionService: &mockConnectionService{
				serviceContext: connectionCtx,
				hasConnection:  true,
			},
			configService: &mockConfigService{},
			signalService: &mockSignalService{},
			proxyService:  &mockProxyService{},
		}

		ctx, cancel := context.WithCancel(context.Background())

		terminated := false

		done := make(chan struct{})
		go func() {
			_ = app.runWithReconnect(ctx, &terminated)
			close(done)
		}()

		cancel()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Errorf("runWithReconnect did not exit after context was cancelled")
		}
	})

	t.Run("ReconnectWhenNoConnection", func(t *testing.T) {
		bus := newMockEventBus()
		connectionCtx := t.Context()

		connectCalled := false
		cfg := config.Config{}
		cfg.Application.Port = 8080

		app := &App{
			eventBus: bus,
			connectionService: &mockConnectionService{
				serviceContext: connectionCtx,
				hasConnection:  false,
				cfg:            cfg,
				connectFunc: func(ctx context.Context, cfg config.Config) error {
					connectCalled = true
					return nil
				},
			},
			configService: &mockConfigService{},
			signalService: &mockSignalService{},
			proxyService:  &mockProxyService{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		terminated := false

		_ = app.runWithReconnect(ctx, &terminated)

		if !connectCalled {
			t.Errorf("Expected connectionService.Connect to be called when no connection present")
		}
	})

	t.Run("HandleReconnectRequestedEvent", func(t *testing.T) {
		bus := newMockEventBus()
		connectionCtx := t.Context()

		var events []event.Event

		app := &App{
			eventBus: bus,
			connectionService: &mockConnectionService{
				serviceContext: connectionCtx,
				hasConnection:  true,
			},
			configService: &mockConfigService{},
			signalService: &mockSignalService{},
			proxyService:  &mockProxyService{},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		terminated := false

		go func() {
			_ = app.runWithReconnect(ctx, &terminated)
		}()

		time.Sleep(50 * time.Millisecond)
		app.triggerReconnect()
		time.Sleep(50 * time.Millisecond)

		bus.mu.Lock()
		events = slices.Clone(bus.events)
		bus.mu.Unlock()

		reconnectFound := false
		for _, evt := range events {
			if evt.Type == event.ReconnectRequested {
				reconnectFound = true
				break
			}
		}

		if !reconnectFound {
			t.Errorf("Expected ReconnectRequested event to be published")
		}

		cancel()
	})
}

func TestTriggerReconnect(t *testing.T) {
	bus := newMockEventBus()
	connectionCtx := t.Context()

	app := &App{
		eventBus: bus,
		connectionService: &mockConnectionService{
			serviceContext: connectionCtx,
		},
	}

	app.triggerReconnect()

	bus.mu.Lock()
	events := bus.events
	bus.mu.Unlock()

	if len(events) != 1 {
		t.Errorf("Expected 1 event to be published, got %d", len(events))
	}

	if len(events) > 0 && events[0].Type != event.ReconnectRequested {
		t.Errorf("Expected ReconnectRequested event, got %s", events[0].Type)
	}

	if len(events) > 0 && events[0].Ctx != connectionCtx {
		t.Errorf("Expected event context to be the connection context")
	}
}

func TestRun(t *testing.T) {
	bus := newMockEventBus()
	cfg := config.Config{}
	cfg.Application.Port = 8080

	configCalled := false
	connectionConfigSet := false
	proxyConfigSet := false

	app := &App{
		eventBus: bus,
		configService: &mockConfigService{
			loadConfigFunc: func() (config.Config, error) {
				configCalled = true
				return cfg, nil
			},
		},
		connectionService: &mockConnectionService{
			serviceContext: context.Background(),
			setConfigFunc: func(config config.Config) {
				connectionConfigSet = true
			},
		},
		signalService: &mockSignalService{},
		proxyService: &mockProxyService{
			setConfigFunc: func(config config.Config) {
				proxyConfigSet = true
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := app.Run(ctx)
	if err != nil {
		t.Errorf("Expected Run to succeed, got error: %v", err)
	}

	if !configCalled {
		t.Errorf("Expected LoadConfig to be called")
	}

	if !connectionConfigSet {
		t.Errorf("Expected connection service config to be set")
	}

	if !proxyConfigSet {
		t.Errorf("Expected proxy service config to be set")
	}
}
