package game

import (
	"net"
	"sync"
	"time"

	"aionlightning/wire"
)

// retryDelay is how long a link waits before connecting again, as AL-Game does.
const retryDelay = 5 * time.Second

// link is a connection to the login or chat server: framed, not encrypted.
type link struct {
	mu   sync.Mutex
	conn net.Conn
	up   bool // registered
}

func (l *link) send(w *wire.Writer) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil || !l.up {
		return false
	}
	_, err := l.conn.Write(wire.Frame(w.Data))
	return err == nil
}

// sendAlways writes even before registering, for the registration itself.
func (l *link) sendAlways(w *wire.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		_, _ = l.conn.Write(wire.Frame(w.Data))
	}
}

// loginLink is the connection to the login server, which checks players' session keys.
type loginLink struct {
	link
	s        *Server
	requests map[int32]*conn // account auth requests in flight
}

func newLoginLink(s *Server) *loginLink {
	return &loginLink{s: s, requests: map[int32]*conn{}}
}

// run keeps the link up: connect, register, read, and connect again when it drops.
func (l *loginLink) run() {
	for {
		nc, err := net.Dial("tcp", l.s.config.LoginAddress)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}
		l.s.log.Info("connected to the login server", "address", l.s.config.LoginAddress)
		l.mu.Lock()
		l.conn, l.up = nc, false
		l.mu.Unlock()
		l.sendAlways(l.registration())
		for {
			payload, err := wire.ReadFrame(nc)
			if err != nil || len(payload) == 0 {
				break
			}
			l.handle(payload[0], wire.NewReader(payload[1:]))
		}
		l.down()
		time.Sleep(retryDelay)
	}
}

// registration is SM_GS_AUTH: the id, the address players connect to, the port, capacity and password.
func (l *loginLink) registration() *wire.Writer {
	config := l.s.config
	w := wire.Packet(0x00)
	w.C(config.ID)
	w.C(4)
	w.B(config.HostAddress[:])
	w.D(0) // no IP ranges
	w.H(config.Port)
	w.D(config.MaxPlayers)
	w.S(config.LoginPassword)
	return w
}

func (l *loginLink) handle(opcode byte, r *wire.Reader) {
	switch opcode {
	case 0x00: // CM_GS_AUTH_RESPONSE
		switch r.C() {
		case 0:
			l.mu.Lock()
			l.up = true
			l.mu.Unlock()
			l.s.log.Info("registered with the login server")
			l.send(l.s.accountList())
		case 1:
			l.s.log.Error("the login server refused this game server's id or password")
		case 2:
			time.AfterFunc(10*time.Second, func() { l.sendAlways(l.registration()) })
		}
	case 0x01: // CM_ACOUNT_AUTH_RESPONSE
		accountID, ok := r.D(), r.C() == 1
		var name string
		var accessLevel, membership byte
		if ok {
			name = r.S()
			r.Q() // accumulated online time
			r.Q() // accumulated rest time
			accessLevel, membership = r.C(), r.C()
		}
		l.mu.Lock()
		c := l.requests[accountID]
		delete(l.requests, accountID)
		l.mu.Unlock()
		if c != nil {
			// Loading the account's characters mustn't hold up the link.
			go l.s.accountAuthenticated(c, accountID, ok, name, accessLevel, membership)
		}
	case 0x02: // CM_REQUEST_KICK_ACCOUNT
		l.s.kickAccount(r.D())
	case 0x03: // CM_ACCOUNT_RECONNECT_KEY
		accountID, key := r.D(), r.D()
		l.mu.Lock()
		c := l.requests[accountID]
		delete(l.requests, accountID)
		l.mu.Unlock()
		if c != nil {
			w := wire.Packet(smReconnectKey)
			w.C(0)
			w.D(key)
			c.close(w)
		}
	}
}

// authenticate asks the login server whether a client's session keys are the ones it handed out.
func (l *loginLink) authenticate(c *conn, accountID, loginOK, playOK1, playOK2 int32) {
	l.mu.Lock()
	if _, pending := l.requests[accountID]; pending || !l.up {
		up := l.up
		l.mu.Unlock()
		if !up {
			c.close(nil)
		}
		return
	}
	l.requests[accountID] = c
	l.mu.Unlock()
	w := wire.Packet(0x01)
	w.D(accountID)
	w.D(loginOK)
	w.D(playOK1)
	w.D(playOK2)
	l.send(w)
}

// requestReconnect asks for a key that takes the player back to the server list.
func (l *loginLink) requestReconnect(c *conn) {
	l.mu.Lock()
	l.requests[c.account.id] = c
	l.mu.Unlock()
	w := wire.Packet(0x02)
	w.D(c.account.id)
	if !l.send(w) {
		c.close(nil)
	}
}

func (l *loginLink) accountDisconnected(accountID int32) {
	l.mu.Lock()
	delete(l.requests, accountID)
	l.mu.Unlock()
	w := wire.Packet(0x03)
	w.D(accountID)
	l.send(w)
}

// down closes the clients waiting for the login server, which can't answer them now.
func (l *loginLink) down() {
	l.mu.Lock()
	waiting := l.requests
	l.requests = map[int32]*conn{}
	l.conn, l.up = nil, false
	l.mu.Unlock()
	for _, c := range waiting {
		c.close(nil)
	}
	l.s.log.Warn("lost the login server")
}

// accountList is SM_ACCOUNT_LIST: the accounts playing here, sent after registering.
func (s *Server) accountList() *wire.Writer {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := wire.Packet(0x04)
	w.D(int32(len(s.accounts)))
	for _, c := range s.accounts {
		w.S(c.account.name)
	}
	return w
}

// kickAccount closes an account's connection at the login server's request.
func (s *Server) kickAccount(accountID int32) {
	s.mu.Lock()
	c := s.accounts[accountID]
	s.mu.Unlock()
	if c != nil {
		c.close(nil)
	} else {
		s.login.accountDisconnected(accountID)
	}
}

// chatLink is the connection to the chat server, which gives players their chat tokens.
type chatLink struct {
	link
	s       *Server
	address [4]byte
	port    uint16
}

func newChatLink(s *Server) *chatLink {
	return &chatLink{s: s}
}

func (l *chatLink) run() {
	for {
		nc, err := net.Dial("tcp", l.s.config.ChatAddress)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}
		l.mu.Lock()
		l.conn, l.up = nc, false
		l.mu.Unlock()
		w := wire.Packet(0x00)
		w.C(l.s.config.ID)
		w.C(4)
		w.B(l.s.config.HostAddress[:])
		w.S(l.s.config.ChatPassword)
		l.sendAlways(w)
		for {
			payload, err := wire.ReadFrame(nc)
			if err != nil || len(payload) == 0 {
				break
			}
			l.handle(payload[0], wire.NewReader(payload[1:]))
		}
		l.mu.Lock()
		l.conn, l.up = nil, false
		l.mu.Unlock()
		l.s.log.Warn("lost the chat server")
		time.Sleep(retryDelay)
	}
}

func (l *chatLink) handle(opcode byte, r *wire.Reader) {
	switch opcode {
	case 0x00: // CM_CS_AUTH_RESPONSE
		if r.C() != 0 {
			l.s.log.Error("the chat server refused this game server's password")
			return
		}
		address, port := r.B(4), r.H()
		l.mu.Lock()
		copy(l.address[:], address)
		l.port, l.up = port, true
		l.mu.Unlock()
		l.s.log.Info("registered with the chat server", "address", net.IP(address).String(), "port", port)
	case 0x01: // CM_CS_PLAYER_AUTH_RESPONSE: the player's chat token
		playerID := r.D()
		token := r.B(int(r.C()))
		l.s.mu.Lock()
		c := l.s.players[playerID]
		l.s.mu.Unlock()
		if c != nil && r.Err == nil {
			c.send(chatInit(token))
		}
	}
}

// playerLogin is SM_CS_PLAYER_AUTH: asks for a token that lets the player into the chat server.
func (l *chatLink) playerLogin(playerID int32, accountName string) {
	w := wire.Packet(0x01)
	w.D(playerID)
	w.S(accountName)
	l.send(w)
}

// playerLogout is SM_CS_PLAYER_LOGOUT.
func (l *chatLink) playerLogout(playerID int32) {
	w := wire.Packet(0x02)
	w.D(playerID)
	l.send(w)
}

// clientAddress is where clients reach the chat server.
func (l *chatLink) clientAddress() ([4]byte, uint16) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.address, l.port
}
