// Package login is the Aion 1.9 login server, a port of Aion Lightning's
// AL-Login: players log in on one port, and game servers register and check
// their players' sessions on another. It keeps AL-Login's database, protocol
// and rules, so the Java game server and the 1.9 client work with it unchanged.
package login

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"aionlightning/crypt"
)

// keyPairs is how many RSA keys connections pick from, as AL-Login does:
// generating one per connection would take longer than the login itself.
const keyPairs = 10

// Server holds what AL-Login keeps in its controllers: the accounts logged in
// here, the registered game servers and the IP bans.
type Server struct {
	store      Store
	autoCreate bool
	log        *slog.Logger
	keys       []*crypt.KeyPair

	mu           sync.Mutex
	onLogin      map[int32]*client
	reconnecting map[int32]reconnectingAccount
	gameServers  map[byte]*gameServer
	hidden       map[byte]bool
	bans         []Ban
}

type reconnectingAccount struct {
	account *Account
	key     int32
}

// NewServer loads the registered game servers and IP bans from store. With
// autoCreate, logging in with an unknown account name creates it.
func NewServer(store Store, autoCreate bool, log *slog.Logger) (*Server, error) {
	s := &Server{
		store:        store,
		autoCreate:   autoCreate,
		log:          log,
		onLogin:      map[int32]*client{},
		reconnecting: map[int32]reconnectingAccount{},
		gameServers:  map[byte]*gameServer{},
	}
	for range keyPairs {
		pair, err := crypt.NewKeyPair()
		if err != nil {
			return nil, err
		}
		s.keys = append(s.keys, pair)
	}
	rows, err := store.GameServers()
	if err != nil {
		return nil, fmt.Errorf("loading game servers: %w", err)
	}
	for _, row := range rows {
		s.gameServers[row.ID] = &gameServer{row: row, accounts: map[int32]*Account{}}
	}
	if s.bans, err = store.Bans(); err != nil {
		return nil, fmt.Errorf("loading IP bans: %w", err)
	}
	log.Info("loaded", "gameServers", len(rows), "ipBans", len(s.bans))
	return s, nil
}

// HideServers keeps the given game servers out of the players' server list.
// They still register and validate any player's session keys, which are not
// tied to a game server, so a player who plays the listed server can be
// relayed to a hidden one (the Java-vs-Go debug setup). Call before serving.
func (s *Server) HideServers(ids ...byte) {
	if s.hidden == nil {
		s.hidden = map[byte]bool{}
	}
	for _, id := range ids {
		s.hidden[id] = true
	}
}

// ServeClients accepts game clients on l until it closes.
func (s *Server) ServeClients(l net.Listener) error {
	return serve(l, s.handleClient)
}

// ServeGameServers accepts game servers on l until it closes.
func (s *Server) ServeGameServers(l net.Listener) error {
	return serve(l, s.handleGameServer)
}

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

func (s *Server) randomKeyPair() *crypt.KeyPair {
	return s.keys[rand.IntN(len(s.keys))]
}

// gameServer is a registered game server, and its connection while it's up.
type gameServer struct {
	row            GameServerRow
	conn           *gameServerConn
	defaultAddress []byte
	ranges         []ipRange
	port           uint16
	maxPlayers     int32
	accounts       map[int32]*Account
}

func (g *gameServer) online() bool {
	return g.conn != nil
}

// addressFor is the address the player at ip connects to: the first of the
// game server's ranges that holds ip, or its default address.
func (g *gameServer) addressFor(ip string) []byte {
	if !g.online() {
		return []byte{127, 0, 0, 1}
	}
	for _, r := range g.ranges {
		if r.contains(ip) {
			return r.address
		}
	}
	return g.defaultAddress
}

// ipRange sends players whose address is between min and max to address.
type ipRange struct {
	min, max uint32
	address  []byte
}

func (r ipRange) contains(ip string) bool {
	parsed := net.ParseIP(ip).To4()
	if parsed == nil {
		return false
	}
	v := uint32(parsed[0])<<24 | uint32(parsed[1])<<16 | uint32(parsed[2])<<8 | uint32(parsed[3])
	return v >= r.min && v <= r.max
}

func addressValue(b []byte) uint32 {
	if len(b) != 4 {
		return 0
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// ipMatches is AL-Login's IP mask: dotted parts that are a number, `*`, or a
// range like `10-20`, and `*` or `*.*.*.*` for any address.
func ipMatches(pattern, ip string) bool {
	if pattern == "*" || pattern == "*.*.*.*" {
		return true
	}
	mask := strings.Split(pattern, ".")
	address := strings.Split(ip, ".")
	if len(address) < len(mask) {
		return false
	}
	for i, part := range mask {
		if part == "*" || part == address[i] {
			continue
		}
		low, high, isRange := strings.Cut(part, "-")
		if !isRange {
			return false
		}
		value, err1 := strconv.Atoi(address[i])
		lo, err2 := strconv.Atoi(low)
		hi, err3 := strconv.Atoi(high)
		if err1 != nil || err2 != nil || err3 != nil || value < lo || value > hi {
			return false
		}
	}
	return true
}

// isBanned reports whether an active IP ban covers ip. Call with s.mu held.
func (s *Server) isBanned(ip string) bool {
	now := time.Now()
	for _, b := range s.bans {
		if b.active(now) && ipMatches(b.Mask, ip) {
			return true
		}
	}
	return false
}

// accountOnGameServer is the game server account id plays on, if any. Call with s.mu held.
func (s *Server) accountOnGameServer(id int32) *gameServer {
	for _, g := range s.gameServers {
		if _, ok := g.accounts[id]; ok {
			return g
		}
	}
	return nil
}

// kickAccount asks the game server holding the account to drop it, and closes
// its login connection. Call with s.mu held.
func (s *Server) kickAccount(id int32) {
	if g := s.accountOnGameServer(id); g != nil && g.conn != nil {
		g.conn.send(requestKickAccount(id))
	}
	if c, ok := s.onLogin[id]; ok {
		delete(s.onLogin, id)
		c.close()
	}
}

// updateOnLogin starts a session in the account's play and rest time: a new
// day resets both, otherwise the time since the last session counts as rest.
func (s *Server) updateOnLogin(a *Account) {
	now := time.Now()
	t := a.Time
	if t == nil {
		t = &AccountTime{LastActive: now}
	}
	if days(t.LastActive) < days(now) {
		t.AccumulatedOnline, t.AccumulatedRest = 0, 0
	} else {
		t.AccumulatedRest += now.Sub(t.LastActive).Milliseconds() - t.SessionDuration
	}
	t.LastActive = now
	a.Time = t
	if err := s.store.SaveAccountTime(a.ID, t); err != nil {
		s.log.Error("saving account time", "account", a.Name, "err", err)
	}
}

// updateOnLogout ends the session and adds it to the account's play time.
func (s *Server) updateOnLogout(a *Account) {
	t := a.Time
	if t == nil {
		return
	}
	t.SessionDuration = time.Since(t.LastActive).Milliseconds()
	t.AccumulatedOnline += t.SessionDuration
	if err := s.store.SaveAccountTime(a.ID, t); err != nil {
		s.log.Error("saving account time", "account", a.Name, "err", err)
	}
}

func days(t time.Time) int64 {
	return t.UnixMilli() / 1000 / 3600 / 24
}

func (t *AccountTime) expired(now time.Time) bool {
	return t != nil && t.Expiration != nil && t.Expiration.Before(now)
}

func (t *AccountTime) penaltyActive(now time.Time) bool {
	return t != nil && t.PenaltyEnd != nil && (t.PenaltyEnd.Equal(permanentPenalty) || !t.PenaltyEnd.Before(now))
}
