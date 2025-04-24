package service

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erlang-solutions.com/cortex_agent/internal/i18n"
)

type SignalService struct {
	gracefulTimeout time.Duration
}

func NewSignalService(gracefulTimeout time.Duration) *SignalService {
	return &SignalService{
		gracefulTimeout: gracefulTimeout,
	}
}

const DefaultGracefulTimeout = 5 * time.Second

func NewDefaultSignalService() *SignalService {
	return NewSignalService(DefaultGracefulTimeout)
}

func (s *SignalService) SetupTerminationHandler(parentCtx context.Context) context.Context {
	ctx, cancel := context.WithCancel(parentCtx)

	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-termCh
		log.Println(i18n.T("termination_signal", nil))
		cancel()

		// Set a timeout for graceful shutdown
		go func() {
			time.Sleep(s.gracefulTimeout)
			log.Println(i18n.T("forced_exit", nil))
			os.Exit(1)
		}()
	}()

	return ctx
}
