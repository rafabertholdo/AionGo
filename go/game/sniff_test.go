package game

import (
	"bufio"
	"log/slog"
	"net"
	"strings"
	"testing"
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
