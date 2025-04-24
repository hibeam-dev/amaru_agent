package service

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
)

type ConfigService struct {
	filePath string
}

func NewConfigService(filePath string) *ConfigService {
	return &ConfigService{
		filePath: filePath,
	}
}

func (s *ConfigService) LoadConfig() (config.Config, error) {
	return config.Load(s.filePath)
}

func (s *ConfigService) SetupReloader(ctx context.Context) <-chan config.Config {
	configCh := make(chan config.Config, 1)
	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)

	go func() {
		defer close(configCh)
		for {
			select {
			case <-ctx.Done():
				return
			case <-sighupCh:
				log.Println(i18n.T("sighup_received", nil))

				newConfig, err := s.LoadConfig()
				if err != nil {
					log.Printf(i18n.Tf("config_reload_error", nil),
						i18n.T("config_reload_error", map[string]interface{}{"Error": err}))
					continue
				}

				select {
				case configCh <- newConfig:
				default:
					// Channel buffer is full, which may mean there's already a pending update
				}
			}
		}
	}()

	return configCh
}
