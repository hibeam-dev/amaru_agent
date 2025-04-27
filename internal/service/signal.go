package service

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erlang-solutions.com/cortex_agent/internal/event"
)

const DefaultGracefulTimeout = 5 * time.Second

type SignalService struct {
	Service
	gracefulTimeout time.Duration
}

func NewSignalService(gracefulTimeout time.Duration, bus *event.Bus) *SignalService {
	return &SignalService{
		Service:         NewService("signal", bus),
		gracefulTimeout: gracefulTimeout,
	}
}

func NewDefaultSignalService(bus *event.Bus) *SignalService {
	return NewSignalService(DefaultGracefulTimeout, bus)
}

func (s *SignalService) Start(ctx context.Context) error {
	if err := s.Service.Start(ctx); err != nil {
		return err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.handleSignals()
	}()
	return nil
}

func (s *SignalService) Stop(ctx context.Context) error {
	return s.Service.Stop(ctx)
}

func (s *SignalService) handleSignals() {
	termCh := make(chan os.Signal, 1)
	sighupCh := make(chan os.Signal, 1)

	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(sighupCh, syscall.SIGHUP)

	defer signal.Stop(termCh)
	defer signal.Stop(sighupCh)

	for {
		select {
		case <-s.ctx.Done():
			return

		case sig := <-termCh:
		drainLoop:
			for {
				select {
				case <-termCh:
					// Continue draining
				default:
					break drainLoop
				}
			}
			s.bus.Publish(event.Event{Type: event.TerminationSignal, Data: sig, Ctx: s.Context()})

			time.AfterFunc(s.gracefulTimeout, func() {
				os.Exit(1)
			})

		case <-sighupCh:
		drainSighup:
			for {
				select {
				case <-sighupCh:
					// Continue draining
				default:
					break drainSighup
				}
			}
			s.bus.Publish(event.Event{Type: event.SIGHUPReceived, Ctx: s.Context()})
		}
	}
}
