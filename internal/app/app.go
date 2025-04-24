package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/service"
	"erlang-solutions.com/cortex_agent/pkg/result"
)

type App struct {
	configService     *service.ConfigService
	connectionService *service.ConnectionService
	signalService     *service.SignalService
}

func NewApp(configFile string, jsonMode bool) *App {
	return &App{
		configService:     service.NewConfigService(configFile),
		connectionService: service.NewConnectionService(jsonMode),
		signalService:     service.NewDefaultSignalService(),
	}
}

func (a *App) Run(ctx context.Context) error {
	ctx = a.signalService.SetupTerminationHandler(ctx)

	configResult := result.FlatMap(
		loadConfig(a.configService),
		func(cfg config.Config) result.Result[config.Config] {
			configUpdateCh := a.configService.SetupReloader(ctx)

			if a.connectionService.IsJSONMode() {
				// In JSON mode, run once and exit
				return runOnce(ctx, a.connectionService, cfg)
			} else {
				// In standard mode, keep reconnecting
				return runWithReconnect(ctx, a.connectionService, cfg, configUpdateCh)
			}
		},
	)

	return configResult.Error()
}

func loadConfig(configService *service.ConfigService) result.Result[config.Config] {
	cfg, err := configService.LoadConfig()
	if err != nil {
		msg := i18n.T("config_load_error", map[string]interface{}{"Error": err})
		return result.Err[config.Config](fmt.Errorf("%s", msg))
	}
	return result.Ok(cfg)
}

func runOnce(
	ctx context.Context,
	connService *service.ConnectionService,
	cfg config.Config,
) result.Result[config.Config] {
	err := executeConnection(ctx, connService, cfg, nil)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println(i18n.T("app_context_terminated", nil))
			return result.Ok(cfg)
		}
		return result.Err[config.Config](err)
	}
	return result.Ok(cfg)
}

func runWithReconnect(
	ctx context.Context,
	connService *service.ConnectionService,
	initialCfg config.Config,
	configUpdateCh <-chan config.Config,
) result.Result[config.Config] {
	currentCfg := initialCfg
	reconnectCh := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			return result.Err[config.Config](ctx.Err())
		case newCfg := <-configUpdateCh:
			currentCfg = newCfg
			select {
			case reconnectCh <- struct{}{}:
			default:
				// Channel already has a pending reconnect signal
			}
		default:
			log.Println(i18n.T("connection_establishing", nil))

			connErr := executeConnection(ctx, connService, currentCfg, reconnectCh)

			select {
			case <-ctx.Done():
				log.Println(i18n.T("app_terminating", nil))
				return result.Err[config.Config](ctx.Err())
			case <-reconnectCh:
				log.Println(i18n.T("reconnecting", nil))
				continue
			default:
				if connErr != nil {
					msg := i18n.T("connection_error_terminating", map[string]interface{}{"Error": connErr})
					log.Printf("%s", msg)
					return result.Err[config.Config](connErr)
				}
				log.Println(i18n.T("connection_normal_termination", nil))
				return result.Ok(currentCfg)
			}
		}
	}
}

func executeConnection(
	ctx context.Context,
	connService *service.ConnectionService,
	cfg config.Config,
	reconnectCh <-chan struct{},
) error {
	conn, err := connService.Connect(ctx, cfg)
	if err != nil {
		return err
	}

	defer func() {
		log.Println(i18n.T("connection_closing", nil))
		if closeErr := conn.Close(); closeErr != nil {
			msg := i18n.T("connection_close_error", map[string]interface{}{"Error": closeErr})
			log.Printf("%s", msg)
		}
	}()

	if err := connService.SendInitialConfig(conn, cfg); err != nil {
		return err
	}

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = connService.Run(connCtx, conn, reconnectCh)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
