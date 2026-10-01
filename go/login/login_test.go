package login

import (
	"encoding/binary"
	"io"
	"log/slog"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

// memoryStore is Store in memory, for tests.
type memoryStore struct {
	mu       sync.Mutex
	accounts map[string]*Account
	times    map[int32]*AccountTime
	bans     []Ban
}

func newMemoryStore() *memoryStore {
	return &memoryStore{accounts: map[string]*Account{}, times: map[int32]*AccountTime{}}
}

func (m *memoryStore) Account(name string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[name]
	if !ok {
		return nil, nil
	}
	copied := *a
	copied.Time = m.times[a.ID]
	return &copied, nil
}

func (m *memoryStore) CreateAccount(name, hash string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := &Account{ID: int32(len(m.accounts) + 1), Name: name, PasswordHash: hash, Activated: 1}
	m.accounts[name] = a
	return a, nil
}

func (m *memoryStore) UpdateAccess(*Account) error                { return nil }
func (m *memoryStore) UpdateLastServer(int32, byte) error         { return nil }
func (m *memoryStore) UpdateLastIP(int32, string) error           { return nil }
func (m *memoryStore) LastIP(int32) (string, error)               { return "", nil }
func (m *memoryStore) AccountTime(id int32) (*AccountTime, error) { return m.times[id], nil }
func (m *memoryStore) SaveAccountTime(id int32, t *AccountTime) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.times[id] = t
	return nil
}
func (m *memoryStore) GameServers() ([]GameServerRow, error) {
	return []GameServerRow{{ID: 1, Mask: "*", Password: "aion"}}, nil
}
func (m *memoryStore) Bans() ([]Ban, error)            { return m.bans, nil }
func (m *memoryStore) AddBan(string, *time.Time) error { return nil }
func (m *memoryStore) RemoveBan(string) error          { return nil }

// startServer runs a login server on loopback ports and returns their addresses.
func startServer(t *testing.T, store Store) (clients, gameServers string) {
	t.Helper()
	server, err := NewServer(store, true, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := net.Listen("tcp", "127.0.0.1:0")
	g, _ := net.Listen("tcp", "127.0.0.1:0")
	t.Cleanup(func() { c.Close(); g.Close() })
	go server.ServeClients(c)
	go server.ServeGameServers(g)
	return c.Addr().String(), g.Addr().String()
}

// testClient speaks the 1.9 client's side of the protocol.
type testClient struct {
	t         *testing.T
	conn      net.Conn
	cipher    *crypt.Blowfish
	sessionID int32
	modulus   *big.Int
}

func dial(t *testing.T, address string) *testClient {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	c := &testClient{t: t, conn: conn}

	// SM_INIT: the fixed initial key, then undo the XOR pass from the key stored 8 bytes from the end.
	data := c.readFrame()
	crypt.NewBlowfish([]byte{0x6b, 0x60, 0xcb, 0x5b, 0x82, 0xce, 0x90, 0xb1, 0xcc, 0x2b, 0x6c, 0x55, 0x6c, 0x6c, 0x6c, 0x6c}).Decrypt(data)
	stop := len(data) - 8
	key := binary.LittleEndian.Uint32(data[stop:])
	for pos := stop - 4; pos >= 4; pos -= 4 {
		word := binary.LittleEndian.Uint32(data[pos:]) ^ key
		key -= word
		binary.LittleEndian.PutUint32(data[pos:], word)
	}
	r := wire.NewReader(data)
	if opcode := r.C(); opcode != 0x00 {
		t.Fatalf("first packet is %#x, not SM_INIT", opcode)
	}
	c.sessionID = r.D()
	if revision := r.D(); revision != protocolRevision {
		t.Fatalf("protocol revision %#x", revision)
	}
	c.modulus = new(big.Int).SetBytes(unscramble(r.B(128)))
	r.B(16)
	c.cipher = crypt.NewBlowfish(r.B(16))
	return c
}

// unscramble undoes crypt.ScrambleModulus, as the client does.
func unscramble(m []byte) []byte {
	for i := range 0x40 {
		m[0x40+i] ^= m[i]
	}
	for i := range 4 {
		m[0x0d+i] ^= m[0x34+i]
	}
	for i := range 0x40 {
		m[i] ^= m[0x40+i]
	}
	for i := range 4 {
		m[i], m[0x4d+i] = m[0x4d+i], m[i]
	}
	return m
}

func (c *testClient) readFrame() []byte {
	c.t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var header [2]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		c.t.Fatal(err)
	}
	data := make([]byte, binary.LittleEndian.Uint16(header[:])-2)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		c.t.Fatal(err)
	}
	return data
}

// receive reads a packet and returns its opcode and fields.
func (c *testClient) receive() (byte, *wire.Reader) {
	c.t.Helper()
	data := c.readFrame()
	c.cipher.Decrypt(data)
	if !verifyWords(data) {
		c.t.Fatal("bad checksum from the server")
	}
	return data[0], wire.NewReader(data[1:])
}

func verifyWords(data []byte) bool {
	var sum uint32
	for i := 0; i+4 <= len(data); i += 4 {
		sum ^= binary.LittleEndian.Uint32(data[i:])
	}
	return sum == 0
}

// send pads, checksums and encrypts a packet as the client does: the checksum
// word, then a last word of filler that the server's check leaves out.
func (c *testClient) send(w *wire.Writer) {
	size := len(w.Data) + 8
	size += 8 - size%8
	data := make([]byte, size)
	copy(data, w.Data)
	var sum uint32
	for i := 0; i < size-8; i += 4 {
		sum ^= binary.LittleEndian.Uint32(data[i:])
	}
	binary.LittleEndian.PutUint32(data[size-8:], sum)
	binary.LittleEndian.PutUint32(data[size-4:], 0xdeadbeef)
	c.cipher.Encrypt(data)
	frame := binary.LittleEndian.AppendUint16(nil, uint16(size+2))
	if _, err := c.conn.Write(append(frame, data...)); err != nil {
		c.t.Fatal(err)
	}
}

// login runs the GameGuard step and sends the credentials, RSA-encrypted.
func (c *testClient) login(name, password string) (byte, *wire.Reader) {
	c.t.Helper()
	w := packet(0x07)
	w.D(c.sessionID)
	w.B(make([]byte, 16))
	c.send(w)
	if opcode, r := c.receive(); opcode != 0x0b || r.D() != c.sessionID {
		c.t.Fatalf("SM_AUTH_GG: opcode %#x", opcode)
	}
	block := make([]byte, 128)
	copy(block[64:], name)
	copy(block[96:], password)
	encrypted := new(big.Int).Exp(new(big.Int).SetBytes(block), big.NewInt(65537), c.modulus).FillBytes(make([]byte, 128))
	w = packet(0x0b)
	w.D(0)
	w.B(encrypted)
	c.send(w)
	return c.receive()
}

// testGameServer speaks the Java game server's side of the login link.
type testGameServer struct {
	t    *testing.T
	conn net.Conn
}

func registerGameServer(t *testing.T, address string) *testGameServer {
	t.Helper()
	return registerGameServerID(t, address, 1)
}

func registerGameServerID(t *testing.T, address string, id byte) *testGameServer {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	g := &testGameServer{t: t, conn: conn}
	w := packet(0x00)
	w.C(id)
	w.C(4)
	w.B([]byte{127, 0, 0, 1})
	w.D(0)
	w.H(7777)
	w.D(100)
	w.S("aion")
	g.send(w)
	if opcode, r := g.receive(); opcode != 0x00 || r.C() != gsAuthed {
		t.Fatal("game server wasn't registered")
	}
	return g
}

func (g *testGameServer) send(w *wire.Writer) {
	frame := binary.LittleEndian.AppendUint16(nil, uint16(len(w.Data)+2))
	if _, err := g.conn.Write(append(frame, w.Data...)); err != nil {
		g.t.Fatal(err)
	}
}

func (g *testGameServer) receive() (byte, *wire.Reader) {
	g.t.Helper()
	_ = g.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var header [2]byte
	if _, err := io.ReadFull(g.conn, header[:]); err != nil {
		g.t.Fatal(err)
	}
	data := make([]byte, binary.LittleEndian.Uint16(header[:])-2)
	if _, err := io.ReadFull(g.conn, data); err != nil {
		g.t.Fatal(err)
	}
	return data[0], wire.NewReader(data[1:])
}

func TestLoginToGameServer(t *testing.T) {
	store := newMemoryStore()
	clientAddress, gameServerAddress := startServer(t, store)
	gameServer := registerGameServer(t, gameServerAddress)

	client := dial(t, clientAddress)
	opcode, r := client.login("Admin", "secret")
	if opcode != 0x03 {
		t.Fatalf("login answered %#x, not SM_LOGIN_OK", opcode)
	}
	accountID, loginOK := r.D(), r.D()
	if a, _ := store.Account("admin"); a == nil || a.PasswordHash != HashPassword("secret") {
		t.Fatal("the account wasn't created with the name in lower case and the password's hash")
	}

	w := packet(0x05)
	w.D(accountID)
	w.D(loginOK)
	w.D(0)
	client.send(w)
	opcode, r = client.receive()
	if opcode != 0x04 {
		t.Fatalf("server list answered %#x", opcode)
	}
	count, _ := r.C(), r.C()
	id, address, port := r.C(), r.B(4), r.D()
	r.C()
	r.C()
	r.H()
	r.H()
	online := r.C()
	if count != 1 || id != 1 || address[0] != 127 || port != 7777 || online != 1 {
		t.Fatalf("server list: %d servers, id %d at %v:%d, online %d", count, id, address, port, online)
	}

	w = packet(0x02)
	w.D(accountID)
	w.D(loginOK)
	w.C(1)
	client.send(w)
	opcode, r = client.receive()
	if opcode != 0x07 {
		t.Fatalf("play answered %#x, not SM_PLAY_OK", opcode)
	}
	playOK1, playOK2 := r.D(), r.D()

	// The game server checks the keys the player brings.
	w = packet(0x01)
	w.D(accountID)
	w.D(loginOK)
	w.D(playOK1)
	w.D(playOK2)
	gameServer.send(w)
	opcode, r = gameServer.receive()
	if opcode != 0x01 || r.D() != accountID || r.C() != 1 || r.S() != "admin" {
		t.Fatal("the game server didn't get the account")
	}

	// Leaving the game server for the server list: a reconnect key, then a new session.
	w = packet(0x02)
	w.D(accountID)
	gameServer.send(w)
	opcode, r = gameServer.receive()
	if opcode != 0x03 || r.D() != accountID {
		t.Fatal("no reconnect key")
	}
	key := r.D()
	again := dial(t, clientAddress)
	w = packet(0x08)
	w.D(accountID)
	w.D(loginOK)
	w.D(key)
	again.send(w)
	if opcode, r := again.receive(); opcode != 0x0c || r.D() != accountID {
		t.Fatalf("reconnect answered %#x", opcode)
	}
}

func TestWrongPasswordFails(t *testing.T) {
	store := newMemoryStore()
	_, _ = store.CreateAccount("admin", HashPassword("secret"))
	clientAddress, _ := startServer(t, store)
	opcode, r := dial(t, clientAddress).login("admin", "wrong")
	if opcode != 0x01 || r.D() != invalidPassword {
		t.Fatalf("got %#x", opcode)
	}
}

func TestGameServerWithWrongPasswordIsRefused(t *testing.T) {
	_, gameServerAddress := startServer(t, newMemoryStore())
	conn, err := net.Dial("tcp", gameServerAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	g := &testGameServer{t: t, conn: conn}
	w := packet(0x00)
	w.C(1)
	w.C(4)
	w.B([]byte{127, 0, 0, 1})
	w.D(0)
	w.H(7777)
	w.D(100)
	w.S("wrong")
	g.send(w)
	if opcode, r := g.receive(); opcode != 0x00 || r.C() != gsNotAuthed {
		t.Fatal("registered with a wrong password")
	}
}

// A game server that restarts without its old connection closing, as when its
// container is killed, registers again and replaces the old connection.
func TestRestartedGameServerReplacesItsOldConnection(t *testing.T) {
	_, gameServerAddress := startServer(t, newMemoryStore())
	old := registerGameServer(t, gameServerAddress)
	registerGameServer(t, gameServerAddress)
	_ = old.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := old.conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("the old connection is still open")
	}
}

func TestHashPasswordMatchesALLogin(t *testing.T) {
	// What AL-Login stored for the password "admin".
	if got := HashPassword("admin"); got != "0DPiKuNIrrVmD8IUCuw1hQxNqZc=" {
		t.Fatal(got)
	}
}

func TestIPMatches(t *testing.T) {
	cases := []struct {
		pattern, ip string
		want        bool
	}{
		{"*", "10.0.0.1", true},
		{"*.*.*.*", "10.0.0.1", true},
		{"192.168.*.*", "192.168.64.1", true},
		{"192.168.10-20.*", "192.168.15.3", true},
		{"192.168.10-20.*", "192.168.21.3", false},
		{"10.0.0.1", "10.0.0.2", false},
	}
	for _, c := range cases {
		if got := ipMatches(c.pattern, c.ip); got != c.want {
			t.Errorf("ipMatches(%q, %q) = %v", c.pattern, c.ip, got)
		}
	}
}

// twoServers is a store with game servers 1 and 2.
type twoServers struct{ *memoryStore }

func (twoServers) GameServers() ([]GameServerRow, error) {
	return []GameServerRow{{ID: 1, Mask: "*", Password: "aion"}, {ID: 2, Mask: "*", Password: "aion"}}, nil
}

func TestHiddenGameServerRegistersAndValidatesButIsNotListed(t *testing.T) {
	store := twoServers{newMemoryStore()}
	server, err := NewServer(store, true, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	server.HideServers(2)
	c, _ := net.Listen("tcp", "127.0.0.1:0")
	g, _ := net.Listen("tcp", "127.0.0.1:0")
	t.Cleanup(func() { c.Close(); g.Close() })
	go server.ServeClients(c)
	go server.ServeGameServers(g)
	registerGameServerID(t, g.Addr().String(), 1)
	hidden := registerGameServerID(t, g.Addr().String(), 2)

	client := dial(t, c.Addr().String())
	_, r := client.login("admin", "secret")
	accountID, loginOK := r.D(), r.D()
	w := packet(0x05)
	w.D(accountID)
	w.D(loginOK)
	w.D(0)
	client.send(w)
	_, r = client.receive()
	if count := r.C(); count != 1 {
		t.Fatalf("server list has %d servers, want only the shown one", count)
	}
	r.C()
	if id := r.C(); id != 1 {
		t.Fatalf("listed server %d", id)
	}

	w = packet(0x02)
	w.D(accountID)
	w.D(loginOK)
	w.C(1)
	client.send(w)
	_, r = client.receive()
	playOK1, playOK2 := r.D(), r.D()
	w = packet(0x01)
	w.D(accountID)
	w.D(loginOK)
	w.D(playOK1)
	w.D(playOK2)
	hidden.send(w)
	if opcode, r := hidden.receive(); opcode != 0x01 || r.D() != accountID || r.C() != 1 {
		t.Fatal("the hidden game server could not validate the player's keys")
	}
}
