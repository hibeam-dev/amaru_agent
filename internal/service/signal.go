package service

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/util"
)

const DefaultGracefulTimeout = 5 * time.Second

type SignalService struct {
	name            string
	bus             *event.Bus
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.Mutex
	gracefulTimeout time.Duration
	unsubscribes    []func()
}

func NewSignalService(gracefulTimeout time.Duration, bus *event.Bus) *SignalService {
	return &SignalService{
		name:            "signal",
		bus:             bus,
		gracefulTimeout: gracefulTimeout,
		unsubscribes:    make([]func(), 0),
	}
}

func NewDefaultSignalService(bus *event.Bus) *SignalService {
	return NewSignalService(DefaultGracefulTimeout, bus)
}

func (s *SignalService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.handleSignals()
	}()
	return nil
}

func (s *SignalService) Stop(ctx context.Context) error {
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
			s.bus.Publish(event.Event{Type: event.TerminationSignal, Data: sig})

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
			s.bus.Publish(event.Event{Type: event.SIGHUPReceived})
		}
	}
}
