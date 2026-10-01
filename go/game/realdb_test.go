package game

import (
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// TestRealDatabase starts a server on a real database and loads its characters, when AION_TEST_DB names the
// database's host: it checks every query the server makes at start and when a player enters the world. The links
// to the login and chat servers point nowhere, so no real server is disturbed.
func TestRealDatabase(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("AION_TEST_DB isn't set")
	}
	d := staticDataOrSkip(t)
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd, config.DBName, config.ParseTime = "tcp", host+":3306", "root", "aion", "au_server_gs", true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewServer(Config{ID: 99, Name: "Test", Mode: 1, MaxPlayers: 10, LoginAddress: "127.0.0.1:1", ChatAddress: "127.0.0.1:1",
		NamePattern: "[a-zA-Z]{2,16}"}, d, store.Store{DB: db}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT id FROM players`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int32
	for rows.Next() {
		var id int32
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		row, err := s.store.Character(id)
		if err != nil || row == nil {
			t.Fatalf("character %d: %v", id, err)
		}
		ch := &character{Character: row, appearance: &store.Appearance{}}
		p, err := s.loadPlayer(ch)
		if err != nil {
			t.Fatalf("loading %s: %v", row.Name, err)
		}
		p.conn = &conn{s: s, tap: func(*wire.Writer) {}, account: &account{}}
		s.visMu.Lock()
		if err := s.loadMailbox(p); err != nil {
			t.Errorf("mailbox of %s: %v", row.Name, err)
		}
		if p.stats == nil || len(skillList(p).Data) == 0 || len(s.friendListPacket(p).Data) == 0 || len(s.mailLetters(p).Data) == 0 {
			t.Errorf("%s isn't loaded", row.Name)
		}
		s.visMu.Unlock()
	}
}
