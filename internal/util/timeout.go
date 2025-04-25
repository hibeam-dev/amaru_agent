package util

import (
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

func WaitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) {
	WaitDone := make(chan struct{})
	go func() {
		defer close(WaitDone)
		wg.Wait()
	}()

	select {
	case <-WaitDone:
	case <-time.After(timeout):
	}
}

func WaitGroupWithTimeout(g *errgroup.Group, timeout time.Duration) {
	WaitDone := make(chan struct{})
	go func() {
		defer close(WaitDone)
		_ = g.Wait()
	}()

	select {
	case <-WaitDone:
	case <-time.After(timeout):
	}
}
