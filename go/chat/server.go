// Package chat is the Aion 1.9 chat server, a port of Aion Lightning's
// AL-CServer: the game server registers each player with a token, the client
// connects with it, joins channels by name and sends messages to them.
//
// Unlike AL-CServer, a message reaches only the players in its channel:
// AL-CServer sent it to everyone in any channel of the same kind, so every
// region's and both races' trade chat reached all traders.
package chat

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net"
	"sync"

	"aionlightning/wire"
)

// Server holds the registered players and the channels.
type Server struct {
	password    string
	chatAddress [4]byte
	chatPort    uint16
	log         *slog.Logger

	mu       sync.Mutex
	players  map[int32]*player
	channels []*channel
}

// player is a character the game server registered, and its chat connection once it connects.
type player struct {
	id         int32
	token      []byte
	identifier []byte
	conn       *clientConn
	channels   map[channelKind]*channel
	nextIndex  uint16
}

// NewServer takes the password game servers register with, and the address
// and port it tells them clients connect to.
func NewServer(password string, chatAddress [4]byte, chatPort uint16, log *slog.Logger) *Server {
	return &Server{
		password:    password,
		chatAddress: chatAddress,
		chatPort:    chatPort,
		log:         log,
		players:     map[int32]*player{},
		channels:    newChannels(1),
	}
}

// ServeClients accepts game clients on l until it closes.
func (s *Server) ServeClients(l net.Listener) error { return serve(l, s.handleClient) }

// ServeGameServers accepts game servers on l until it closes.
func (s *Server) ServeGameServers(l net.Listener) error { return serve(l, s.handleGameServer) }

func serve(l net.Listener, handle func(net.Conn)) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}
		go handle(conn)
	}
}

// link is a connection whose packets are framed and not encrypted.
type link struct {
	conn    net.Conn
	writeMu sync.Mutex
}

func (l *link) send(w *wire.Writer) {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	_, _ = l.conn.Write(wire.Frame(w.Data))
}

// handleGameServer is the game server's side: it registers, then registers and logs out players.
func (s *Server) handleGameServer(conn net.Conn) {
	defer conn.Close()
	l := &link{conn: conn}
	authed := false
	for {
		payload, err := wire.ReadFrame(conn)
		if err != nil || len(payload) == 0 {
			return
		}
		r := wire.NewReader(payload[1:])
		switch opcode := payload[0]; {
		case !authed && opcode == 0x00:
			id := r.C()
			r.B(int(r.C()))
			ok := r.S() == s.password
			response := byte(1) // not authed
			if ok {
				response = 0
			}
			w := wire.Packet(0x00)
			w.C(response)
			w.B(s.chatAddress[:])
			w.H(s.chatPort)
			l.send(w)
			if ok {
				authed = true
				s.mu.Lock()
				s.channels = newChannels(id)
				s.mu.Unlock()
			}
			s.log.Info("game server registration", "id", id, "authed", ok)
		case authed && opcode == 0x01:
			id, login := r.D(), r.S()
			p := s.registerPlayer(id, login)
			w := wire.Packet(0x01)
			w.D(id)
			w.C(byte(len(p.token)))
			w.B(p.token)
			l.send(w)
		case authed && opcode == 0x02:
			s.logout(r.D())
		default:
			s.log.Warn("unknown game server packet", "opcode", opcode, "authed", authed)
		}
	}
}

// registerPlayer gives the player a token: 16 random bytes, then the SHA-256 of its account name.
func (s *Server) registerPlayer(id int32, login string) *player {
	account := sha256.Sum256([]byte(login))
	token := make([]byte, 16, 48)
	_, _ = rand.Read(token)
	token = append(token, account[:]...)
	p := &player{id: id, token: token, channels: map[channelKind]*channel{}, nextIndex: 1}
	s.mu.Lock()
	s.players[id] = p
	s.mu.Unlock()
	return p
}

func (s *Server) logout(id int32) {
	s.mu.Lock()
	p := s.players[id]
	delete(s.players, id)
	s.mu.Unlock()
	if p != nil && p.conn != nil {
		_ = p.conn.conn.Close()
	}
	s.log.Info("player logged out", "player", id)
}

// clientConn is a game client's chat connection.
type clientConn struct {
	link
	player *player
}

func (s *Server) handleClient(conn net.Conn) {
	c := &clientConn{link: link{conn: conn}}
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		if c.player != nil && c.player.conn == c {
			c.player.conn = nil
		}
		s.mu.Unlock()
	}()
	for {
		payload, err := wire.ReadFrame(conn)
		if err != nil || len(payload) == 0 {
			return
		}
		r := wire.NewReader(payload[1:])
		switch opcode := payload[0]; {
		case c.player == nil && opcode == 0x05:
			s.authenticate(c, r)
		case c.player != nil && opcode == 0x10:
			s.join(c, r)
		case c.player != nil && opcode == 0x18:
			s.message(c, r)
		}
	}
}

// authenticate is CM_PLAYER_AUTH: the player's id, name and the token the game server passed on.
func (s *Server) authenticate(c *clientConn, r *wire.Reader) {
	r.C()
	r.H()
	r.H()
	r.H()
	r.S() // "AION"
	id := r.D()
	r.D()
	r.D()
	identifier := r.B(int(r.H()) * 2)
	r.B(int(r.H()) * 2) // account name
	token := r.B(int(r.H()))
	if r.Err != nil {
		return
	}
	s.mu.Lock()
	p := s.players[id]
	if p == nil || !bytes.Equal(p.token, token) {
		s.mu.Unlock()
		return
	}
	p.identifier = identifier
	p.conn = c
	c.player = p
	s.mu.Unlock()
	s.log.Info("player connected", "player", id, "name", decodeUTF16LE(identifier))
	w := wire.Packet(0x02)
	w.C(0x40)
	w.H(0x01)
	w.D(0x0bdd0000)
	c.send(w)
}

// join is CM_CHANNEL_REQUEST: joining a channel by its name.
func (s *Server) join(c *clientConn, r *wire.Reader) {
	r.C()
	r.H()
	r.H() // the client's index for it
	name := r.B(int(r.H()) * 2)
	s.mu.Lock()
	var joined *channel
	for _, ch := range s.channels {
		if bytes.Equal(ch.name, name) {
			joined = ch
			break
		}
	}
	if joined == nil {
		s.mu.Unlock()
		s.log.Warn("unknown channel", "player", c.player.id, "name", decodeUTF16LE(name))
		return
	}
	c.player.channels[joined.kind] = joined
	c.player.nextIndex++
	index := c.player.nextIndex
	s.mu.Unlock()
	s.log.Info("joined channel", "player", c.player.id, "channel", joined.id, "name", decodeUTF16LE(name))
	w := wire.Packet(0x11)
	w.C(0x40)
	w.H(index)
	w.H(0)
	w.D(joined.id)
	c.send(w)
}

// message is CM_CHANNEL_MESSAGE: text for a channel the player is in, sent to everyone in it.
func (s *Server) message(c *clientConn, r *wire.Reader) {
	r.H()
	r.C()
	r.D()
	r.D()
	channelID := r.D()
	text := r.B(int(r.H()) * 2)
	if r.Err != nil {
		return
	}
	s.mu.Lock()
	sender := c.player
	var recipients []*clientConn
	for _, p := range s.players {
		if ch := p.channels[kindOf(s.channels, channelID)]; ch != nil && ch.id == channelID && p.conn != nil {
			recipients = append(recipients, p.conn)
		}
	}
	s.mu.Unlock()
	w := wire.Packet(0x1a)
	w.C(0)
	w.D(channelID)
	w.D(sender.id)
	w.D(0)
	w.H(uint16(len(sender.identifier) / 2))
	w.B(sender.identifier)
	w.H(uint16(len(text) / 2))
	w.B(text)
	for _, conn := range recipients {
		conn.send(w)
	}
}

// kindOf is the kind of the channel with id, or -1 when there's none.
func kindOf(channels []*channel, id int32) channelKind {
	for _, ch := range channels {
		if ch.id == id {
			return ch.kind
		}
	}
	return -1
}
