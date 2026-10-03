package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

type drainingGame struct {
	accepted chan struct{}
	draining chan struct{}
	saved    chan struct{}
}

func (s *drainingGame) Serve(l net.Listener) error {
	close(s.accepted)
	_, err := l.Accept()
	if !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (s *drainingGame) CloseClients() {
	close(s.draining)
	<-s.saved
}

func TestShutdownStopsAcceptingAndWaitsForSaves(t *testing.T) {
	for _, cause := range []string{"process", "administrative"} {
		t.Run(cause, func(t *testing.T) {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &drainingGame{accepted: make(chan struct{}), draining: make(chan struct{}), saved: make(chan struct{})}
			shutdown := make(chan bool, 1)
			done := make(chan error, 1)
			go func() { done <- serveGame(ctx, s, l, shutdown, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
			<-s.accepted
			if cause == "process" {
				cancel()
			} else {
				shutdown <- true
			}
			select {
			case <-s.draining:
			case <-time.After(time.Second):
				t.Fatal("shutdown never drained clients")
			}
			if c, err := net.DialTimeout("tcp", l.Addr().String(), time.Second); err == nil {
				c.Close()
				t.Error("shutdown still accepts new clients")
			}
			select {
			case <-done:
				t.Fatal("shutdown returned before saves completed")
			default:
			}
			close(s.saved)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("shutdown did not finish after saves completed")
			}
		})
	}
}
