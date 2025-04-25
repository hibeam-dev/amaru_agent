package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/util"
)

type ConfigService struct {
	name     string
	bus      *event.Bus
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	filePath string

	mu           sync.RWMutex
	config       config.Config
	loaded       bool
	unsubscribes []func()
}

func NewConfigService(filePath string, bus *event.Bus) *ConfigService {
	return &ConfigService{
		name:         "config",
		bus:          bus,
		filePath:     filePath,
		unsubscribes: make([]func(), 0),
	}
}

func (s *ConfigService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	unsub := s.bus.Subscribe(event.SIGHUPReceived, func(evt event.Event) {
		s.reloadAndPublishConfig()
	})

	s.mu.Lock()
	s.unsubscribes = append(s.unsubscribes, unsub)
	s.mu.Unlock()

	return nil
}

func (s *ConfigService) GetConfig() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *ConfigService) SetConfig(cfg config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
	s.loaded = true
}

func (s *ConfigService) LoadConfig() (config.Config, error) {
	cfg, err := config.Load(s.filePath)
	if err != nil {
		return config.Config{}, fmt.Errorf("%s: %w",
			i18n.T("config_load_error", map[string]interface{}{"Error": err}), err)
	}

	s.SetConfig(cfg)
	return cfg, nil
}

func (s *ConfigService) Stop(ctx context.Context) error {
	s.mu.Lock()
	unsubscribes := s.unsubscribes
	s.unsubscribes = nil
	s.mu.Unlock()

	for _, unsub := range unsubscribes {
		if unsub != nil {
			unsub()
		}
	}

	if s.cancel != nil {
		s.cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		util.WaitWithTimeout(&s.wg, 500*time.Millisecond)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		err := ctx.Err()
		if util.IsExpectedError(err) {
			return nil
		}
		return err
	}
}

func (s *ConfigService) reloadAndPublishConfig() {
	newConfig, err := config.Load(s.filePath)
	if err != nil {
		return
	}

	s.SetConfig(newConfig)
	s.bus.Publish(event.Event{Type: event.ConfigUpdated, Data: newConfig})
}
