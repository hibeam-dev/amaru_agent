package service

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	SIGHUPReceived EventType = "sighup_received"
)

const DefaultGracefulTimeout = 5 * time.Second

type SignalService struct {
	BaseService
	gracefulTimeout time.Duration
}

func NewSignalService(gracefulTimeout time.Duration, bus *EventBus) *SignalService {
	return &SignalService{
		BaseService:     NewBaseService("signal", bus),
		gracefulTimeout: gracefulTimeout,
	}
}

func NewDefaultSignalService(bus *EventBus) *SignalService {
	return NewSignalService(DefaultGracefulTimeout, bus)
}

func (s *SignalService) Start(ctx context.Context) error {
	if err := s.BaseService.Start(ctx); err != nil {
		return err
	}

	s.Go(func() {
		s.handleSignals()
	})
	return nil
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
		case <-s.Context().Done():
			return

		case sig := <-termCh:
			drainSignals(termCh)
			s.bus.Publish(Event{Type: TerminationSignal, Data: sig})

			// Set up forced exit after timeout
			time.AfterFunc(s.gracefulTimeout, func() {
				os.Exit(1)
			})

		case <-sighupCh:
			drainSignals(sighupCh)
			s.bus.Publish(Event{Type: SIGHUPReceived})
		}
	}
}
