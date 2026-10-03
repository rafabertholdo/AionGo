package game

import (
	"bytes"
	"database/sql"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
	"github.com/go-sql-driver/mysql"
)

// Run only against a disposable database supplied through AION_TEST_DB.
func TestShutdownPersistsCharacterProgressDatabase(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("AION_TEST_DB isn't set")
	}
	d := staticDataOrSkip(t)
	cfg := mysql.NewConfig()
	cfg.Net, cfg.Addr, cfg.User, cfg.Passwd, cfg.DBName, cfg.ParseTime = "tcp", host+":3306", "root", "aion", "au_server_gs", true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := testServer(d)
	s.store = store.Store{DB: db}
	s.punishDB = s.store
	s.chat = &chatLink{}
	s.clockStart = time.Now()
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, nil))
	p, _ := fighter(t, s, 1000)
	p.ID = 2000000001
	p.Name = "ShutdownFixture"
	p.Created = time.Now()
	p.appearance = &store.Appearance{}
	if err = s.store.CreateCharacter(p.Character, p.appearance, nil, nil); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.store.DeleteCharacter(p.ID); err != nil {
			t.Error(err)
		}
	}()
	p.Exp = 987654
	p.RecoverExp = 321
	p.Class = "CLERIC"
	p.X, p.Y, p.Z = 640, 1060, 99
	p.Heading = 12
	p.life = store.LifeStats{HP: 91, MP: 123, FP: 45}
	if err = s.store.SetOnline(p.ID, true); err != nil {
		t.Fatal(err)
	}
	serverPeer, clientPeer := net.Pipe()
	defer clientPeer.Close()
	c := s.newClient(serverPeer)
	c.player = p
	c.state = inGame
	p.conn = c
	s.clients = map[net.Conn]*conn{serverPeer: c}
	s.spawn(p)
	ready := make(chan struct{})
	go func() { _, _ = wire.ReadFrame(clientPeer); close(ready); _, _ = wire.ReadFrame(clientPeer) }()
	s.clientWG.Add(1)
	go func() { defer s.clientWG.Done(); s.handle(c) }()
	<-ready
	s.CloseClients()
	saved, err := s.store.Character(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.Exp != p.Exp || saved.RecoverExp != p.RecoverExp || saved.Class != p.Class || saved.X != p.X || saved.Y != p.Y || saved.Z != p.Z || saved.Heading != p.Heading {
		t.Fatalf("shutdown lost progress: %+v", saved)
	}
	life, err := s.store.LifeStats(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if life == nil || *life != p.life {
		t.Fatalf("shutdown lost life stats: %+v", life)
	}
	var online bool
	if err = db.QueryRow("SELECT online FROM players WHERE id = ?", p.ID).Scan(&online); err != nil {
		t.Fatal(err)
	}
	if online || bytes.Contains(logs.Bytes(), []byte("level=ERROR")) {
		t.Fatalf("shutdown did not finish cleanly: %s", logs.String())
	}
}
