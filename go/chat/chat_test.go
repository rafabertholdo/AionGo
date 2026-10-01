package chat

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"aionlightning/wire"
)

type testLink struct {
	t    *testing.T
	conn net.Conn
}

func connect(t *testing.T, address string) *testLink {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &testLink{t: t, conn: conn}
}

func (l *testLink) send(w *wire.Writer) {
	if _, err := l.conn.Write(wire.Frame(w.Data)); err != nil {
		l.t.Fatal(err)
	}
}

func (l *testLink) receive() (byte, *wire.Reader) {
	l.t.Helper()
	_ = l.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	data, err := wire.ReadFrame(l.conn)
	if err != nil {
		l.t.Fatal(err)
	}
	return data[0], wire.NewReader(data[1:])
}

// expectNothing checks that no packet arrives for a moment.
func (l *testLink) expectNothing() {
	l.t.Helper()
	_ = l.conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if data, err := wire.ReadFrame(l.conn); err == nil {
		l.t.Fatalf("unexpected packet %#x", data[0])
	}
}

func start(t *testing.T) (clients, gameServers string) {
	t.Helper()
	server := NewServer("aion", [4]byte{127, 0, 0, 1}, 10241, slog.New(slog.DiscardHandler))
	c, _ := net.Listen("tcp", "127.0.0.1:0")
	g, _ := net.Listen("tcp", "127.0.0.1:0")
	t.Cleanup(func() { c.Close(); g.Close() })
	go server.ServeClients(c)
	go server.ServeGameServers(g)
	return c.Addr().String(), g.Addr().String()
}

func registerGameServer(t *testing.T, address, password string) (*testLink, byte, *wire.Reader) {
	g := connect(t, address)
	w := wire.Packet(0x00)
	w.C(1)
	w.C(4)
	w.B([]byte{127, 0, 0, 1})
	w.S(password)
	g.send(w)
	_, r := g.receive()
	return g, r.C(), r
}

// joinAs registers player id through the game server, connects it and joins the channel called name.
func joinAs(t *testing.T, g *testLink, clients string, id int32, name string) *testLink {
	t.Helper()
	w := wire.Packet(0x01)
	w.D(id)
	w.S("account")
	g.send(w)
	opcode, r := g.receive()
	if opcode != 0x01 || r.D() != id {
		t.Fatal("no token for the player")
	}
	token := r.B(int(r.C()))

	c := connect(t, clients)
	identifier := utf16LE("Player")
	account := utf16LE("account")
	w = wire.Packet(0x05)
	w.C(0x40)
	w.H(0)
	w.H(1)
	w.H(4)
	w.S("AION")
	w.D(id)
	w.D(0)
	w.D(0)
	w.H(uint16(len(identifier) / 2))
	w.B(identifier)
	w.H(uint16(len(account) / 2))
	w.B(account)
	w.H(uint16(len(token)))
	w.B(token)
	c.send(w)
	if opcode, _ := c.receive(); opcode != 0x02 {
		t.Fatalf("auth answered %#x", opcode)
	}

	channel := utf16LE(name)
	w = wire.Packet(0x10)
	w.C(0x40)
	w.H(0)
	w.H(1)
	w.H(uint16(len(channel) / 2))
	w.B(channel)
	c.send(w)
	if opcode, _ := c.receive(); opcode != 0x11 {
		t.Fatalf("join answered %#x", opcode)
	}
	return c
}

func TestMessagesReachOnlyTheirChannel(t *testing.T) {
	clients, gameServers := start(t)
	g, response, r := registerGameServer(t, gameServers, "aion")
	if response != 0 || r.B(4)[0] != 127 || r.H() != 10241 {
		t.Fatal("game server not registered with the chat address")
	}
	poeta := "@\x01trade_lf1\x011.0.AION.KOR"
	a := joinAs(t, g, clients, 100, poeta)
	b := joinAs(t, g, clients, 101, poeta)
	other := joinAs(t, g, clients, 102, "@\x01trade_DC1\x011.1.AION.KOR")

	w := wire.Packet(0x18)
	w.H(0)
	w.C(0)
	w.D(0)
	w.D(0)
	w.D(3) // trade_lf1 for Elyos: after the two group channels
	text := utf16LE("wts sword")
	w.H(uint16(len(text) / 2))
	w.B(text)
	a.send(w)

	for _, c := range []*testLink{a, b} {
		opcode, r := c.receive()
		if opcode != 0x1a {
			t.Fatalf("got %#x", opcode)
		}
		r.C()
		if channel, sender := r.D(), r.D(); channel != 3 || sender != 100 {
			t.Fatalf("channel %d from %d", channel, sender)
		}
	}
	other.expectNothing()
}

func TestWrongPasswordIsRefused(t *testing.T) {
	_, gameServers := start(t)
	if _, response, _ := registerGameServer(t, gameServers, "wrong"); response != 1 {
		t.Fatal("registered with a wrong password")
	}
}

func TestWrongTokenIsIgnored(t *testing.T) {
	clients, _ := start(t)
	c := connect(t, clients)
	w := wire.Packet(0x05)
	w.C(0x40)
	w.H(0)
	w.H(1)
	w.H(4)
	w.S("AION")
	w.D(7)
	w.D(0)
	w.D(0)
	w.H(0)
	w.H(0)
	w.H(2)
	w.B([]byte{1, 2})
	c.send(w)
	c.expectNothing()
}

func TestChannelNamesFollowTheGameServerID(t *testing.T) {
	channels := newChannels(1)
	if len(channels) != 70 || channels[2].id != 3 || decodeUTF16LE(channels[2].name) != "@\x01trade_lf1\x011.0.AION.KOR" {
		t.Fatalf("%d channels, third is %q", len(channels), decodeUTF16LE(channels[2].name))
	}
}
