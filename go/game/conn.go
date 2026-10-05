package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"aionlightning/crypt"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// account is what the game server knows of a logged-in account.
type account struct {
	id          int32
	name        string
	accessLevel byte
	membership  byte
	characters  []*character
}

// character is a character on the select screen.
type character struct {
	*store.Character
	appearance *store.Appearance
	equipment  []*store.Item
	legionID   int32
}

func (c *character) deletionSeconds() int32 {
	if c.Deletion == nil {
		return 0
	}
	return int32(c.Deletion.Unix())
}

// conn is one game client's connection.
type conn struct {
	s       *Server
	netConn net.Conn
	ip      string
	crypt   *gameCrypt
	key     int32
	state   stateSet
	account *account
	player  *player // in the world

	// worldMu serializes the client's packets with the timers entering the world starts.
	worldMu sync.Mutex

	writeMu sync.Mutex
	closed  bool

	// tap sees every packet sent, for tests.
	tap func(*wire.Writer)

	// dialogReplies counts the packets sent that a quest handler could have sent in answer to a dialog (everything
	// but the periodic ones and a movie: Java's handlers play one and return false, so the client's select is then
	// answered with the window of the same number). questDialog compares it before and after a handler to know whether Java's
	// QuestEngine.onDialog would have returned true; dialogPassed makes a handler that replied say it returned false.
	dialogReplies atomic.Int32
	dialogPassed  atomic.Bool
}

func (s *Server) newClient(nc net.Conn) *conn {
	ip, _, _ := net.SplitHostPort(nc.RemoteAddr().String())
	var key [4]byte
	_, _ = rand.Read(key[:])
	crypt, sent := newGameCrypt(binary.LittleEndian.Uint32(key[:]))
	return &conn{s: s, netConn: nc, ip: ip, crypt: crypt, key: sent, state: inConnected}
}

func (s *Server) handle(c *conn) {
	nc, ip := c.netConn, c.ip
	s.log.Info("connection", "ip", ip)
	defer c.disconnected()

	w := wire.Packet(smKey)
	w.D(c.key)
	c.send(w)
	verified, strays := false, 0
	for {
		payload, err := wire.ReadFrame(nc)
		if err != nil {
			return
		}
		// After a logout to the server list the 1.9 client opens the game connection with a stray frame (once a
		// two-byte one, once a CM_QUIT-sized one, encrypted with the previous connection's key) before it has read
		// SM_KEY. AL-Game drops the connection and the client hangs. Until a packet has decrypted, skip such frames
		// and keep the cipher where it was.
		raw := fmt.Sprintf("%x", payload)
		saved := *c.crypt.in
		if len(payload) < 3 || !c.crypt.decrypt(payload) {
			if verified || strays >= 3 {
				s.log.Warn("decrypt failed", "ip", ip, "hex", raw)
				return
			}
			strays++
			*c.crypt.in = saved
			s.log.Warn("stray frame skipped", "ip", ip, "hex", raw)
			continue
		}
		verified = true
		opcode := payload[0]
		s.log.Debug("packet", "opcode", fmt.Sprintf("%#02x", opcode), "size", len(payload))
		handler, known := handlers[opcode]
		if !known || clientPacketStates[opcode]&c.state == 0 {
			s.log.Debug("unhandled packet", "opcode", fmt.Sprintf("%#02x", opcode), "state", c.state, "hex", fmt.Sprintf("%x", payload))
			continue
		}
		c.worldMu.Lock()
		handler(c, wire.NewReader(payload[3:]))
		c.worldMu.Unlock()
	}
}

// send writes a packet: its opcode (encoded, with the server code and its
// complement) and payload encrypted, after a 2-byte size.
func (c *conn) send(w *wire.Writer) {
	if w == nil || len(w.Data) == 0 {
		return
	}
	// The game header adds two bytes to the packet builder's opcode and fields.
	if len(w.Data) > wire.MaxPayloadSize-2 {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		c.failWrite(wire.ErrFrameTooLarge)
		return
	}
	switch w.Data[0] {
	case smStatupdateHp, smStatupdateMp, smStatupdateDp, smAttackStatus, smTimeCheck, smMove, smNpcInfo, smEmotion, smLookatobject, smPong, smPlayMovie:
	default:
		c.dialogReplies.Add(1)
	}
	if c.tap != nil {
		c.tap(w)
	}
	if c.netConn == nil {
		return
	}
	op := crypt.EncodeServerOpcode(w.Data[0])
	payload := append([]byte{op, crypt.ServerPacketCode, ^op}, w.Data[1:]...)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return
	}
	c.crypt.encrypt(payload)
	if err := c.netConn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		c.failWrite(err)
		return
	}
	if err := wire.WriteFrame(c.netConn, payload); err != nil {
		c.failWrite(err)
	}
}

// failWrite closes a stream that cannot continue with its current cipher state.
// The caller holds writeMu; the reader's deferred cleanup saves player state.
func (c *conn) failWrite(err error) {
	if c.closed {
		return
	}
	c.closed = true
	if c.netConn != nil {
		_ = c.netConn.Close()
	}
	if c.s != nil && c.s.log != nil {
		c.s.log.Warn("game client write failed", "ip", c.ip, "err", err)
	}
}

// close sends a last packet, if any, and closes the connection.
func (c *conn) close(last *wire.Writer) {
	if last != nil {
		c.send(last)
	}
	c.writeMu.Lock()
	c.closed = true
	c.writeMu.Unlock()
	if c.netConn != nil {
		_ = c.netConn.Close()
	}
}

func (c *conn) disconnected() {
	c.close(nil)
	c.worldMu.Lock()
	if !c.lingerLogout() {
		c.leaveWorld()
	}
	c.worldMu.Unlock()
	if c.account == nil {
		return
	}
	c.s.mu.Lock()
	if c.s.accounts[c.account.id] == c {
		delete(c.s.accounts, c.account.id)
	}
	c.s.mu.Unlock()
	c.s.login.accountDisconnected(c.account.id)
	c.s.log.Info("disconnected", "account", c.account.name)
}

// logoutDelay is how long the player of a dropped client stays in the world (AionConnection.onDisconnect),
// so that closing the client doesn't get it out of a fight.
const logoutDelay = 15 * time.Second

// lingering is PlayerService.playerLoggedOutDelay's pending logout of a dropped client's player.
type lingering struct {
	c     *conn
	id    int32
	once  sync.Once
	timer *time.Timer // set under the server's mu
}

// lingerLogout is PlayerService.playerLoggedOutDelay: the player stops and stays in the world for logoutDelay,
// unless the server is shutting down. The caller holds worldMu.
func (c *conn) lingerLogout() bool {
	p, s := c.player, c.s
	if p == nil || s.shuttingDown.Load() {
		return false
	}
	s.visMu.Lock()
	if p.spawned {
		p.broadcast(movePacket(p.ID, p.X, p.Y, p.Z, byte(p.Heading), moveStop, nil, nil), false)
	}
	s.visMu.Unlock()
	l := &lingering{c: c, id: p.ID}
	s.clientWG.Add(1)
	s.mu.Lock()
	if s.lingering == nil {
		s.lingering = map[int32]*lingering{}
	}
	s.lingering[p.ID] = l
	l.timer = time.AfterFunc(logoutDelay, l.finish)
	s.mu.Unlock()
	return true
}

// finish logs the lingering player out, once.
func (l *lingering) finish() {
	l.once.Do(func() {
		s := l.c.s
		l.c.worldMu.Lock()
		l.c.leaveWorld()
		l.c.worldMu.Unlock()
		s.mu.Lock()
		if s.lingering[l.id] == l {
			delete(s.lingering, l.id)
		}
		s.mu.Unlock()
		s.clientWG.Done()
	})
}

// finishLogouts logs out now the lingering players, of one account or of all (accountID 0):
// before the account reads its characters again, or when the server stops.
// The caller holds neither the server's mu nor visMu.
func (s *Server) finishLogouts() { s.finishAccountLogouts(0) }

func (s *Server) finishAccountLogouts(accountID int32) {
	s.mu.Lock()
	var list []*lingering
	for _, l := range s.lingering {
		if accountID == 0 || l.c.account.id == accountID {
			l.timer.Stop()
			list = append(list, l)
		}
	}
	s.mu.Unlock()
	for _, l := range list {
		l.finish()
	}
}
