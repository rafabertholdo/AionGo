package store

import (
	"database/sql"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestAutoAnnouncements runs the announcements query against a real database, when AION_TEST_DB names its host.
// It adds one row with an id above any real one, and removes it.
func TestAutoAnnouncements(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("AION_TEST_DB isn't set")
	}
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd, config.DBName, config.ParseTime = "tcp", host+":3306", "root", "aion", "au_server_gs", true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const id = 987654 // announcements.id is int(3) for display only; any real row has a small id
	clean := func() { db.Exec(`DELETE FROM announcements WHERE id = ?`, id) }
	clean()
	t.Cleanup(clean)
	if _, err := db.Exec("INSERT INTO announcements (id, announce, faction, `type`, `delay`) VALUES (?, 'test row', 'ELYOS', 'YELLOW', 90)", id); err != nil {
		t.Fatal(err)
	}
	list, err := Store{DB: db}.AutoAnnouncements()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list {
		if a.ID == id {
			if a.Text != "test row" || a.Faction != "ELYOS" || a.Type != "YELLOW" || a.Delay != 90 {
				t.Fatalf("read %+v", a)
			}
			return
		}
	}
	t.Fatalf("row %d not read back from %d rows", id, len(list))
}
