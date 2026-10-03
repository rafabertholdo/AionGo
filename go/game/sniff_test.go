package game

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"aionlightning/wire"
)

func TestSnifferControlRetargetsAndDropsClients(t *testing.T) {
	s := NewSniffer("old:7777", slog.New(slog.DiscardHandler))
	client, peer := net.Pipe()
	s.clients[peer] = true
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	t.Cleanup(func() { l.Close() })
	go s.Control(l)
	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	replies := bufio.NewReader(conn)
	ask := func(line string) string {
		conn.Write([]byte(line + "\n"))
		reply, _ := replies.ReadString('\n')
		return strings.TrimSpace(reply)
	}
	if got := ask("target new:7777"); got != "ok target new:7777" {
		t.Fatal(got)
	}
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("the open connection was not dropped")
	}
	if got := ask("mark run1"); got != "ok mark run1" {
		t.Fatal(got)
	}
	if got := ask("status"); !strings.HasPrefix(got, "ok target new:7777") {
		t.Fatal(got)
	}
}

func TestSnifferReturnsWhenKeyPacketIsMissing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"empty", nil},
		{"short_header", []byte{1, 2}},
		{"short_key", []byte{1, 2, 3}},
		{"wrong_opcode", make([]byte, 7)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			serverDone := make(chan error, 1)
			go func() {
				peer, err := listener.Accept()
				if err != nil {
					serverDone <- err
					return
				}
				defer peer.Close()
				serverDone <- wire.WriteFrame(peer, tc.payload)
			}()
			client, relay := net.Pipe()
			defer client.Close()
			defer relay.Close()
			if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				sniffConnection(relay, listener.Addr().String(), slog.New(slog.DiscardHandler))
			}()
			if _, err := wire.ReadFrame(client); !errors.Is(err, io.EOF) {
				t.Fatalf("invalid key packet reached the client: %v", err)
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("sniffer stayed blocked waiting for a key")
			}
			if err := <-serverDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}
