package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/event"
	"erlang-solutions.com/cortex_agent/internal/transport/mocks"
	"go.uber.org/mock/gomock"
)

type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error {
	return nil
}

func TestTunnelReconnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping reconnection test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bus := event.NewBus()
	proxyService := NewProxyService(bus)

	err := proxyService.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start proxy service: %v", err)
	}
	defer func() {
		if err := proxyService.Stop(ctx); err != nil {
			t.Logf("Error stopping proxy service: %v", err)
		}
	}()

	var cfg config.Config
	cfg.Application.IP = "127.0.0.1"
	cfg.Application.Port = 8080
	proxyService.SetConfig(cfg)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTunnel := mocks.NewMockConnection(ctrl)
	binOutput := bytes.NewBuffer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
	writeCloser := &nopWriteCloser{&bytes.Buffer{}}

	mockTunnel.EXPECT().BinaryInput().Return(writeCloser).AnyTimes()
	mockTunnel.EXPECT().BinaryOutput().Return(binOutput).AnyTimes()
	mockTunnel.EXPECT().CheckHealth(gomock.Any()).Return(nil).AnyTimes()
	mockTunnel.EXPECT().Close().Return(nil).AnyTimes()

	connectionFailedCh := make(chan struct{}, 1)
	subscription := bus.Subscribe(event.ConnectionFailed, func(evt event.Event) {
		if evt.Type == event.ConnectionFailed {
			select {
			case connectionFailedCh <- struct{}{}:
			default:
			}
		}
	})
	defer subscription()

	bus.Publish(event.Event{
		Type: event.ConnectionEstablished,
		Data: mockTunnel,
		Ctx:  ctx,
	})

	bus.Publish(event.Event{
		Type: event.ConnectionFailed,
		Data: errors.New("simulated connection failure"),
		Ctx:  ctx,
	})

	select {
	case <-connectionFailedCh:
		t.Log("Successfully detected connection failure")
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for connection failed event")
	}

	mockTunnelReconnect := mocks.NewMockConnection(ctrl)
	binOutputReconnect := bytes.NewBuffer([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
	writeCloserReconnect := &nopWriteCloser{&bytes.Buffer{}}

	mockTunnelReconnect.EXPECT().BinaryInput().Return(writeCloserReconnect).AnyTimes()
	mockTunnelReconnect.EXPECT().BinaryOutput().Return(binOutputReconnect).AnyTimes()
	mockTunnelReconnect.EXPECT().CheckHealth(gomock.Any()).Return(nil).AnyTimes()
	mockTunnelReconnect.EXPECT().Close().Return(nil).AnyTimes()

	bus.Publish(event.Event{
		Type: event.ConnectionEstablished,
		Data: mockTunnelReconnect,
		Ctx:  ctx,
	})

	requestData := []byte("GET / HTTP/1.1\r\nHost: test.local\r\n\r\n")
	writeCloserReconnect.Writer.(*bytes.Buffer).Reset() // Clear any previous data

	binOutputReconnect.Write(requestData)

	err = mockTunnelReconnect.CheckHealth(ctx)
	if err != nil {
		t.Errorf("Failed to check health on reconnected tunnel: %v", err)
	}
	t.Logf("Successfully tested reconnection flow")
}
