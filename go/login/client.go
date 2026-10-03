package login

import (
	"bytes"
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

// The client's login steps, which decide the packets it may send.
type clientState int

const (
	connected clientState = iota
	authedGG
	authedLogin
)

// Login results the client shows (AionAuthResponse).
const (
	authed          int32 = 0
	systemError     int32 = 1
	invalidPassword int32 = 2
	noGSRegistered  int32 = 6
	alreadyLoggedIn int32 = 7
	serverDown      int32 = 8
	timeExpired     int32 = 18
	banIP           int32 = 22
	serverFull      int32 = 15
)

// protocolRevision is what SM_INIT tells the 1.9 client.
const protocolRevision = 0x0000c621

type sessionKey struct {
	accountID, loginOK, playOK1, playOK2 int32
}

// client is one game client's connection.
type client struct {
	s         *Server
	conn      net.Conn
	ip        string
	engine    *crypt.Engine
	keys      *crypt.KeyPair
	sessionID int32
	state     clientState

	// Guarded by s.mu: the account and session also reach it from game server connections.
	account  *Account
	session  sessionKey
	joinedGS bool

	writeMu sync.Mutex
}

func (s *Server) handleClient(conn net.Conn) {
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	blowfishKey := crypt.NewBlowfishKey()
	c := &client{
		s:         s,
		conn:      conn,
		ip:        ip,
		engine:    crypt.NewEngine(blowfishKey),
		keys:      s.randomKeyPair(),
		sessionID: crypt.RandomInt32(),
	}
	s.log.Info("client connected", "ip", ip)
	defer c.disconnected()

	c.send(c.initPacket(blowfishKey))
	for {
		payload, err := wire.ReadFrame(conn)
		if err != nil {
			return
		}
		if !c.engine.Decrypt(payload) {
			s.log.Warn("wrong checksum from client", "ip", ip)
			return
		}
		if !c.handle(payload) {
			return
		}
	}
}

// handle runs one decrypted packet and reports whether the connection stays open.
func (c *client) handle(payload []byte) bool {
	r := wire.NewReader(payload[1:])
	opcode := payload[0]
	switch {
	case c.state == connected && opcode == 0x07:
		sessionID := r.D()
		if sessionID != c.sessionID {
			return c.fail(systemError)
		}
		c.state = authedGG
		w := packet(0x0b)
		w.D(sessionID)
		w.B(make([]byte, 16))
		c.send(w)
	case c.state == connected && opcode == 0x08:
		accountID, loginOK, key := r.D(), r.D(), r.D()
		return c.reconnect(accountID, loginOK, key)
	case c.state == authedGG && opcode == 0x0b:
		r.D()
		if r.Remaining() < 128 {
			return true
		}
		return c.login(c.keys.Decrypt(r.B(128)))
	case c.state == authedLogin && opcode == 0x05:
		accountID, loginOK := r.D(), r.D()
		return c.serverList(accountID, loginOK)
	case c.state == authedLogin && opcode == 0x02:
		accountID, loginOK, serverID := r.D(), r.D(), r.C()
		return c.play(accountID, loginOK, serverID)
	default:
		c.s.log.Warn("unknown client packet", "opcode", opcode, "state", c.state)
	}
	return true
}

// login checks the credentials in the RSA block: the name at 64 and the password at 96, 32 bytes each.
func (c *client) login(block []byte) bool {
	name := strings.ToLower(field(block[64:96]))
	password := field(block[96:128])
	response := c.s.login(c, name, password)
	if response != authed {
		return c.fail(response)
	}
	c.state = authedLogin
	c.s.mu.Lock()
	session := c.session
	c.s.mu.Unlock()
	w := packet(0x03)
	w.D(session.accountID)
	w.D(session.loginOK)
	w.D(0)
	w.D(0)
	w.D(0x3ea)
	w.D(0)
	w.D(0)
	w.D(0)
	w.B(make([]byte, 16))
	c.send(w)
	return true
}

// field is a fixed-size text field: up to the first zero byte, without surrounding spaces, as Java's trim leaves it.
func field(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

// login is AccountController.login.
func (s *Server) login(c *client, name, password string) int32 {
	account, err := s.store.Account(name)
	if err != nil {
		s.log.Error("loading account", "account", name, "err", err)
		return systemError
	}
	if account == nil && s.autoCreate {
		if account, err = s.store.CreateAccount(name, HashPassword(password)); err != nil {
			s.log.Error("creating account", "account", name, "err", err)
			return systemError
		}
		s.log.Info("account created", "account", name)
	}
	now := time.Now()
	switch {
	case account == nil, account.PasswordHash != HashPassword(password), account.Activated != 1:
		return invalidPassword
	case account.Time.expired(now):
		return timeExpired
	case account.Time.penaltyActive(now):
		return banIP
	case account.IPForce != "" && !ipMatches(account.IPForce, c.ip):
		return banIP
	}

	s.mu.Lock()
	if s.isBanned(c.ip) {
		s.mu.Unlock()
		return banIP
	}
	if g := s.accountOnGameServer(account.ID); g != nil {
		if g.conn != nil {
			g.conn.send(requestKickAccount(account.ID))
		}
		s.mu.Unlock()
		return alreadyLoggedIn
	}
	if other, ok := s.onLogin[account.ID]; ok {
		delete(s.onLogin, account.ID)
		s.mu.Unlock()
		other.close()
		return alreadyLoggedIn
	}
	c.account = account
	c.session = newSession(account.ID)
	s.onLogin[account.ID] = c
	s.mu.Unlock()

	s.updateOnLogin(account)
	if err := s.store.UpdateLastIP(account.ID, c.ip); err != nil {
		s.log.Error("saving last IP", "account", name, "err", err)
	}
	s.log.Info("logged in", "account", name, "ip", c.ip)
	return authed
}

func newSession(accountID int32) sessionKey {
	return sessionKey{accountID, crypt.RandomInt32(), crypt.RandomInt32(), crypt.RandomInt32()}
}

func (c *client) checkLogin(accountID, loginOK int32) bool {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	return c.session.accountID == accountID && c.session.loginOK == loginOK
}

func (c *client) serverList(accountID, loginOK int32) bool {
	if !c.checkLogin(accountID, loginOK) {
		return c.fail(systemError)
	}
	c.s.mu.Lock()
	servers := slices.SortedFunc(maps.Values(c.s.gameServers), func(a, b *gameServer) int { return int(a.row.ID) - int(b.row.ID) })
	servers = slices.DeleteFunc(servers, func(g *gameServer) bool { return c.s.hidden[g.row.ID] })
	if len(servers) == 0 {
		c.s.mu.Unlock()
		return c.fail(noGSRegistered)
	}
	w := packet(0x04)
	w.C(byte(len(servers)))
	w.C(c.account.LastServer)
	for _, g := range servers {
		w.C(g.row.ID)
		w.B(g.addressFor(c.ip))
		w.D(int32(g.port))
		w.C(0) // age limit
		w.C(1) // pvp
		w.H(uint16(len(g.accounts)))
		w.H(uint16(g.maxPlayers))
		w.Bool(g.online())
		w.D(1)
		w.C(0)
	}
	c.s.mu.Unlock()
	c.send(w)
	return true
}

func (c *client) play(accountID, loginOK int32, serverID byte) bool {
	if !c.checkLogin(accountID, loginOK) {
		return c.fail(systemError)
	}
	c.s.mu.Lock()
	g := c.s.gameServers[serverID]
	var response int32 = authed
	switch {
	case g == nil || !g.online():
		response = serverDown
	case int32(len(g.accounts)) >= g.maxPlayers:
		response = serverFull
	default:
		c.joinedGS = true
	}
	session := c.session
	c.s.mu.Unlock()
	if response != authed {
		w := packet(0x06)
		w.D(response)
		c.send(w)
		return true
	}
	w := packet(0x07)
	w.D(session.playOK1)
	w.D(session.playOK2)
	c.send(w)
	return true
}

// reconnect lets a player back to the server list after leaving a game server, with the key it handed out.
func (c *client) reconnect(accountID, loginOK, key int32) bool {
	c.s.mu.Lock()
	r, ok := c.s.reconnecting[accountID]
	delete(c.s.reconnecting, accountID)
	if !ok || r.key != key {
		c.s.mu.Unlock()
		return false
	}
	c.account = r.account
	c.session = newSession(r.account.ID)
	c.s.onLogin[r.account.ID] = c
	session := c.session
	c.s.mu.Unlock()
	c.state = authedLogin
	w := packet(0x0c)
	w.D(session.accountID)
	w.D(session.loginOK)
	w.C(0)
	c.send(w)
	return true
}

// initPacket is SM_INIT: the session id, the scrambled RSA key and the connection's Blowfish key.
func (c *client) initPacket(blowfishKey []byte) *wire.Writer {
	w := packet(0x00)
	w.D(c.sessionID)
	w.D(protocolRevision)
	w.B(c.keys.ScrambledModulus)
	w.B(make([]byte, 16))
	w.B(blowfishKey)
	w.D(197635)
	w.D(2097152)
	return w
}

func packet(opcode byte) *wire.Writer {
	return wire.Packet(opcode)
}

// send encrypts and writes one packet.
func (c *client) send(w *wire.Writer) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	n := len(w.Data)
	if n > wire.MaxPayloadSize || c.engine.EncryptedSize(n) > wire.MaxPayloadSize {
		c.close()
		return
	}
	payload := make([]byte, c.engine.EncryptedSize(n))
	copy(payload, w.Data)
	size := c.engine.Encrypt(payload, n)
	if err := wire.WriteFrame(c.conn, payload[:size]); err != nil {
		c.close()
	}
}

// fail sends SM_LOGIN_FAIL and closes the connection; it returns false to stop reading.
func (c *client) fail(response int32) bool {
	w := packet(0x01)
	w.D(response)
	c.send(w)
	c.close()
	return false
}

func (c *client) close() {
	_ = c.conn.Close()
}

// disconnected forgets the account if the player didn't go on to a game server.
func (c *client) disconnected() {
	c.close()
	c.s.mu.Lock()
	account, joined := c.account, c.joinedGS
	if account != nil && !joined && c.s.onLogin[account.ID] == c {
		delete(c.s.onLogin, account.ID)
	}
	c.s.mu.Unlock()
	if account != nil && !joined {
		c.s.updateOnLogout(account)
	}
}
