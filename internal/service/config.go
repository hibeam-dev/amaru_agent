package service

import (
	"context"
	"fmt"
	"sync"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
)

type ConfigService struct {
	BaseService
	filePath string

	mu     sync.RWMutex
	config config.Config
	loaded bool
}

func NewConfigService(filePath string, bus *EventBus) *ConfigService {
	return &ConfigService{
		BaseService: NewBaseService("config", bus),
		filePath:    filePath,
	}
}

func (s *ConfigService) Start(ctx context.Context) error {
	if err := s.BaseService.Start(ctx); err != nil {
		return err
	}

	sighupCh := s.bus.Subscribe(SIGHUPReceived, 1)
	s.Go(func() {
		s.handleEvents(sighupCh)
	})

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

func (s *ConfigService) handleEvents(sighupCh <-chan Event) {
	for {
		select {
		case <-s.Context().Done():
			return

		case _, ok := <-sighupCh:
			if !ok {
				return
			}
			s.reloadAndPublishConfig()
		}
	}
}

func (s *ConfigService) reloadAndPublishConfig() {
	newConfig, err := config.Load(s.filePath)
	if err != nil {
		return
	}

	s.SetConfig(newConfig)
	s.bus.Publish(Event{Type: ConfigUpdated, Data: newConfig})
}
