package store

import (
	"database/sql"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestAccountWarehouseRows saves an item in the account warehouse (location 2, owned by an account id) and reads it
// back, when AION_TEST_DB names a database host. It touches one inventory row with an id far above any real one.
func TestAccountWarehouseRows(t *testing.T) {
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
	s := Store{DB: db}
	const id, account = 2000000100, 2000000101
	clean := func() { s.DeleteItem(id) }
	clean()
	t.Cleanup(clean)
	item := &Item{UniqueID: id, ItemID: 182400001, Count: 5, Owner: account, Location: 2}
	if err := s.InsertItem(item); err != nil {
		t.Fatal(err)
	}
	item.Count = 9
	if err := s.UpdateItem(item); err != nil {
		t.Fatal(err)
	}
	got, err := s.Items(account, 2, false)
	if err != nil || len(got) != 1 || got[0].UniqueID != id || got[0].Count != 9 {
		t.Fatalf("account warehouse: %v, %v", got, err)
	}
	if other, _ := s.Items(account, 0, false); len(other) != 0 {
		t.Errorf("found in the cube: %v", other)
	}
}
