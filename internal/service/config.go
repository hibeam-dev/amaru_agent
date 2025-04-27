package service

import (
	"context"
	"sync"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/util"
)

type ConfigService struct {
	Service
	filePath string

	configMu sync.RWMutex
	config   config.Config
	loaded   bool
}

func NewConfigService(filePath string, bus *event.Bus) *ConfigService {
	s := &ConfigService{
		Service:  NewService("config", bus),
		filePath: filePath,
	}
	return s
}

func (s *ConfigService) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	unsub := s.bus.Subscribe(event.SIGHUPReceived, s.handleSIGHUP)
	s.AddSubscription(unsub)

	return nil
}

func (s *ConfigService) Stop(ctx context.Context) error {
	return s.Service.Stop(ctx)
}

func (s *ConfigService) GetConfig() config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config
}

func (s *ConfigService) SetConfig(cfg config.Config) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	s.config = cfg
	s.loaded = true
}

func (s *ConfigService) LoadConfig() (config.Config, error) {
	cfg, err := config.Load(s.filePath)
	if err != nil {
		return config.Config{}, util.WrapError(i18n.T("config_load_error", map[string]any{"Error": err}), err)
	}

	s.SetConfig(cfg)
	return cfg, nil
}

func (s *ConfigService) handleSIGHUP(evt event.Event) {
	if evt.Type != event.SIGHUPReceived {
		return
	}
	s.reloadAndPublishConfig()
}

func (s *ConfigService) reloadAndPublishConfig() {
	newConfig, err := config.Load(s.filePath)
	if err != nil {
		return
	}

	s.SetConfig(newConfig)
	s.bus.Publish(event.Event{Type: event.ConfigUpdated, Data: newConfig, Ctx: s.Context()})
}
