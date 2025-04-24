package service

import (
	"os"
	"sync"
	"time"
)

func drainSignals(ch <-chan os.Signal) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) {
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		wg.Wait()
	}()

	select {
	case <-waitDone:
	case <-time.After(timeout):
	}
}
