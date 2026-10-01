package login

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

// Game server registration results (GsAuthResponse).
const (
	gsAuthed    byte = 0
	gsNotAuthed byte = 1
)

// gameServerConn is a game server's connection. Its packets are framed like
// the client's but not encrypted.
type gameServerConn struct {
	s       *Server
	conn    net.Conn
	ip      string
	server  *gameServer
	writeMu sync.Mutex
}

func (s *Server) handleGameServer(conn net.Conn) {
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	g := &gameServerConn{s: s, conn: conn, ip: ip}
	s.log.Info("game server connected", "ip", ip)
	defer g.disconnected()
	for {
		payload, err := wire.ReadFrame(conn)
		if err != nil || len(payload) == 0 {
			return
		}
		if !g.handle(payload[0], wire.NewReader(payload[1:])) {
			return
		}
	}
}

func (g *gameServerConn) handle(opcode byte, r *wire.Reader) bool {
	if g.server == nil {
		if opcode != 0x00 {
			g.s.log.Warn("unknown game server packet before registering", "opcode", opcode)
			return true
		}
		return g.register(r)
	}
	switch opcode {
	case 0x01:
		key := sessionKey{r.D(), r.D(), r.D(), r.D()}
		g.accountAuth(key)
	case 0x02:
		g.reconnectKey(r.D())
	case 0x03:
		g.accountDisconnected(r.D())
	case 0x04:
		names := make([]string, r.D())
		for i := range names {
			names[i] = r.S()
		}
		g.accountList(names)
	case 0x05:
		kind, admin, account, player, param := r.C(), r.S(), r.S(), r.S(), r.C()
		g.control(kind, admin, account, player, param)
	case 0x06:
		kind, accountID, ip, minutes, adminID := r.C(), r.D(), r.S(), r.D(), r.D()
		g.ban(kind, accountID, ip, minutes, adminID)
	default:
		g.s.log.Warn("unknown game server packet", "opcode", opcode)
	}
	return true
}

// register is CM_GS_AUTH: the game server's id, its addresses for players, port, capacity and password.
func (g *gameServerConn) register(r *wire.Reader) bool {
	id := r.C()
	defaultAddress := r.B(int(r.C()))
	ranges := make([]ipRange, r.D())
	for i := range ranges {
		low, high, address := r.B(int(r.C())), r.B(int(r.C())), r.B(int(r.C()))
		ranges[i] = ipRange{min: addressValue(low), max: addressValue(high), address: address}
	}
	port, maxPlayers, password := r.H(), r.D(), r.S()
	if r.Err != nil {
		return false
	}

	s := g.s
	s.mu.Lock()
	response := gsAuthed
	server := s.gameServers[id]
	var stale *gameServerConn
	switch {
	case server == nil:
		response = gsNotAuthed
	case server.row.Password != password || !ipMatches(server.row.Mask, g.ip):
		response = gsNotAuthed
	default:
		// AL-Login refuses a server already registered, but that is how a
		// restarted one finds its old connection when the old never closed
		// (a killed container sends no FIN): the new one replaces it.
		if server.conn != nil {
			stale = server.conn
			clear(server.accounts)
		}
		server.defaultAddress, server.ranges, server.port, server.maxPlayers = defaultAddress, ranges, port, maxPlayers
		server.conn = g
		g.server = server
	}
	s.mu.Unlock()
	if stale != nil {
		s.log.Warn("game server registered again: dropping its old connection", "id", id, "old", stale.ip, "new", g.ip)
		_ = stale.conn.Close()
	}

	w := packet(0x00)
	w.C(response)
	g.send(w)
	s.log.Info("game server registration", "id", id, "ip", g.ip, "response", response)
	return response == gsAuthed
}

// accountAuth is CM_ACCOUNT_AUTH: a player arrived at the game server with the session keys the login server gave it.
func (g *gameServerConn) accountAuth(key sessionKey) {
	s := g.s
	s.mu.Lock()
	c, ok := s.onLogin[key.accountID]
	if !ok || c.session != key {
		s.mu.Unlock()
		w := packet(0x01)
		w.D(key.accountID)
		w.Bool(false)
		g.send(w)
		return
	}
	delete(s.onLogin, key.accountID)
	account := c.account
	g.server.accounts[account.ID] = account
	hidden := s.hidden[g.server.row.ID]
	if !hidden { // a hidden server is not on the list, so it cannot be the last one played
		account.LastServer = g.server.row.ID
	}
	s.mu.Unlock()

	if !hidden {
		if err := s.store.UpdateLastServer(account.ID, account.LastServer); err != nil {
			s.log.Error("saving last server", "account", account.Name, "err", err)
		}
	}
	var online, rest int64
	if account.Time != nil {
		online, rest = account.Time.AccumulatedOnline, account.Time.AccumulatedRest
	}
	w := packet(0x01)
	w.D(account.ID)
	w.Bool(true)
	w.S(account.Name)
	w.Q(online)
	w.Q(rest)
	w.C(account.AccessLevel)
	w.C(account.Membership)
	g.send(w)
}

// reconnectKey is CM_ACCOUNT_RECONNECT_KEY: a player is leaving the game server for the server list.
func (g *gameServerConn) reconnectKey(accountID int32) {
	key := crypt.RandomInt32()
	s := g.s
	s.mu.Lock()
	if account, ok := g.server.accounts[accountID]; ok {
		delete(g.server.accounts, accountID)
		s.reconnecting[accountID] = reconnectingAccount{account: account, key: key}
	}
	s.mu.Unlock()
	w := packet(0x03)
	w.D(accountID)
	w.D(key)
	g.send(w)
}

func (g *gameServerConn) accountDisconnected(accountID int32) {
	g.s.mu.Lock()
	account, ok := g.server.accounts[accountID]
	delete(g.server.accounts, accountID)
	g.s.mu.Unlock()
	if ok {
		g.s.updateOnLogout(account)
	}
}

// accountList is CM_ACCOUNT_LIST: the players already on a game server that
// (re)connects; any also playing elsewhere is kicked from it.
func (g *gameServerConn) accountList(names []string) {
	s := g.s
	for _, name := range names {
		account, err := s.store.Account(name)
		if err != nil || account == nil {
			continue
		}
		s.mu.Lock()
		if s.accountOnGameServer(account.ID) != nil {
			s.mu.Unlock()
			g.send(requestKickAccount(account.ID))
			continue
		}
		g.server.accounts[account.ID] = account
		s.mu.Unlock()
	}
}

// control is CM_LS_CONTROL: a GM changing an account's access level (1) or membership (2).
func (g *gameServerConn) control(kind byte, admin, accountName, player string, param byte) {
	account, err := g.s.store.Account(accountName)
	result := false
	if err == nil && account != nil {
		switch kind {
		case 1:
			account.AccessLevel = param
		case 2:
			account.Membership = param
		}
		result = g.s.store.UpdateAccess(account) == nil
	}
	var id int32
	if account != nil {
		id = account.ID
	}
	w := packet(0x04)
	w.C(kind)
	w.Bool(result)
	w.S(admin)
	w.S(player)
	w.C(param)
	w.D(id)
	g.send(w)
}

// ban is CM_BAN: ban an account (1), an IP (2) or both (3) for minutes (0 for
// ever, negative to lift it), then kick the account.
func (g *gameServerConn) ban(kind byte, accountID int32, ip string, minutes, adminID int32) {
	s := g.s
	result := false
	now := time.Now()
	if (kind == 1 || kind == 3) && accountID != 0 {
		var end *time.Time
		if minutes >= 0 {
			t := permanentPenalty
			if minutes > 0 {
				t = now.Add(time.Duration(minutes) * time.Minute)
			}
			end = &t
		}
		s.mu.Lock()
		var account *Account
		if server := s.accountOnGameServer(accountID); server != nil {
			account = server.accounts[accountID]
		}
		s.mu.Unlock()
		if account != nil {
			if account.Time == nil {
				account.Time = &AccountTime{LastActive: now}
			}
			account.Time.PenaltyEnd = end
			result = true
		} else if t, err := s.store.AccountTime(accountID); err == nil {
			if t == nil {
				t = &AccountTime{LastActive: now}
			}
			t.PenaltyEnd = end
			result = s.store.SaveAccountTime(accountID, t) == nil
		}
	}
	if kind == 2 || kind == 3 {
		if accountID != 0 {
			if last, err := s.store.LastIP(accountID); err == nil && last != "" {
				ip = last
			}
		}
		if ip != "" {
			result = s.banIP(ip, minutes, now) || result
		}
	}
	if accountID != 0 {
		s.mu.Lock()
		s.kickAccount(accountID)
		s.mu.Unlock()
	}
	w := packet(0x05)
	w.C(kind)
	w.D(accountID)
	w.S(ip)
	w.D(minutes)
	w.D(adminID)
	w.Bool(result)
	g.send(w)
}

// banIP replaces any ban on ip with one for minutes (0 for ever), or only lifts it for negative minutes.
func (s *Server) banIP(ip string, minutes int32, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := false
	for i, b := range s.bans {
		if b.Mask == ip {
			if s.store.RemoveBan(ip) == nil {
				s.bans = append(s.bans[:i], s.bans[i+1:]...)
				result = true
			}
			break
		}
	}
	if minutes < 0 {
		return result
	}
	var end *time.Time
	if minutes > 0 {
		t := now.Add(time.Duration(minutes) * time.Minute)
		end = &t
	}
	if s.store.AddBan(ip, end) != nil {
		return false
	}
	s.bans = append(s.bans, Ban{Mask: ip, End: end})
	return true
}

func requestKickAccount(accountID int32) *wire.Writer {
	w := packet(0x02)
	w.D(accountID)
	return w
}

// send writes one packet, framed with its size and not encrypted.
func (g *gameServerConn) send(w *wire.Writer) {
	g.writeMu.Lock()
	defer g.writeMu.Unlock()
	frame := binary.LittleEndian.AppendUint16(nil, uint16(len(w.Data)+2))
	_, _ = g.conn.Write(append(frame, w.Data...))
}

// disconnected takes the game server offline and forgets its players.
func (g *gameServerConn) disconnected() {
	_ = g.conn.Close()
	g.s.mu.Lock()
	defer g.s.mu.Unlock()
	if g.server != nil && g.server.conn == g {
		g.server.conn = nil
		g.server.port = 0
		clear(g.server.accounts)
	}
	g.s.log.Info("game server disconnected", "ip", g.ip)
}
