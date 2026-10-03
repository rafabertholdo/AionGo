package store

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// Godstone persistence tests require a disposable database with the repository schema.
func godstoneTestStore(t *testing.T) Store {
	t.Helper()
	dsn := os.Getenv("AION_GODSTONE_TEST_DSN")
	if dsn == "" {
		t.Skip("AION_GODSTONE_TEST_DSN isn't set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return Store{DB: db}
}

func godstoneFixture(t *testing.T, s Store, base int32, count int64) (Item, Item, Item) {
	t.Helper()
	weapon := Item{UniqueID: base, ItemID: 100000001, Count: 1, Owner: base + 3}
	stone := Item{UniqueID: base + 1, ItemID: 168000001, Count: count, Owner: weapon.Owner}
	kinah := Item{UniqueID: base + 2, ItemID: 182400001, Count: 1000000, Owner: weapon.Owner}
	for _, item := range []Item{weapon, stone, kinah} {
		if err := s.InsertItem(&item); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.DeleteItem(item.UniqueID); err != nil {
				t.Error(err)
			}
		})
	}
	for _, socket := range []Stone{{ItemID: 167000001, Slot: 0}, {ItemID: 168000002, Category: 1}} {
		if err := s.AddStone(weapon.UniqueID, socket); err != nil {
			t.Fatal(err)
		}
	}
	return weapon, stone, kinah
}

func TestSocketGodstonePersistsAndReloads(t *testing.T) {
	s := godstoneTestStore(t)
	for index, count := range []int64{2, 1} {
		name := "stack"
		if count == 1 {
			name = "final stone"
		}
		t.Run(name, func(t *testing.T) {
			weapon, stone, kinah := godstoneFixture(t, s, 2000000300+int32(index)*10, count)
			const price = 100000
			if err := s.SocketGodstone(context.Background(), weapon, stone, kinah, price); err != nil {
				t.Fatal(err)
			}
			items, err := s.Items(weapon.Owner, 0, false)
			if err != nil {
				t.Fatal(err)
			}
			wantItems := 3
			if count == 1 {
				wantItems = 2
			}
			if len(items) != wantItems {
				t.Fatalf("reloaded %d inventory rows, want %d", len(items), wantItems)
			}
			for _, item := range items {
				switch item.UniqueID {
				case weapon.UniqueID:
					if item.Godstone != stone.ItemID {
						t.Errorf("reloaded godstone %d, want %d", item.Godstone, stone.ItemID)
					}
				case stone.UniqueID:
					if count == 1 || item.Count != count-1 {
						t.Errorf("remaining stone count %d, initial count %d", item.Count, count)
					}
				case kinah.UniqueID:
					if item.Count != kinah.Count-price {
						t.Errorf("remaining kinah %d, want %d", item.Count, kinah.Count-price)
					}
				}
			}
			sockets, err := s.Stones(weapon.UniqueID)
			if err != nil {
				t.Fatal(err)
			}
			if len(sockets) != 2 || !slices.Contains(sockets, Stone{ItemID: 167000001}) ||
				!slices.Contains(sockets, Stone{ItemID: stone.ItemID, Category: 1}) {
				t.Fatalf("replacement did not preserve manastone: %+v", sockets)
			}
		})
	}
}

func TestSocketGodstoneRejectsStaleInventoryAndRollsBack(t *testing.T) {
	s := godstoneTestStore(t)
	tests := []struct {
		name       string
		stoneCount int64
		query      string
		args       func(Item, Item, Item) []any
		stoneAfter int64
		kinahAfter int64
	}{
		{"stone count", 2, "UPDATE inventory SET itemCount = 3 WHERE itemUniqueId = ?",
			func(_, stone, _ Item) []any { return []any{stone.UniqueID} }, 3, 1000000},
		{"kinah count after stack consumption", 2, "UPDATE inventory SET itemCount = 999999 WHERE itemUniqueId = ?",
			func(_, _, kinah Item) []any { return []any{kinah.UniqueID} }, 2, 999999},
		{"kinah count after final consumption", 1, "UPDATE inventory SET itemCount = 999999 WHERE itemUniqueId = ?",
			func(_, _, kinah Item) []any { return []any{kinah.UniqueID} }, 1, 999999},
		{"weapon owner", 2, "UPDATE inventory SET itemOwner = itemOwner + 1 WHERE itemUniqueId = ?",
			func(weapon, _, _ Item) []any { return []any{weapon.UniqueID} }, 2, 1000000},
		{"weapon equipped", 2, "UPDATE inventory SET isEquiped = 1 WHERE itemUniqueId = ?",
			func(weapon, _, _ Item) []any { return []any{weapon.UniqueID} }, 2, 1000000},
		{"weapon template", 2, "UPDATE inventory SET itemId = itemId + 1 WHERE itemUniqueId = ?",
			func(weapon, _, _ Item) []any { return []any{weapon.UniqueID} }, 2, 1000000},
		{"stone owner", 2, "UPDATE inventory SET itemOwner = itemOwner + 1 WHERE itemUniqueId = ?",
			func(_, stone, _ Item) []any { return []any{stone.UniqueID} }, 2, 1000000},
		{"kinah owner after consumption", 2, "UPDATE inventory SET itemOwner = itemOwner + 1 WHERE itemUniqueId = ?",
			func(_, _, kinah Item) []any { return []any{kinah.UniqueID} }, 2, 1000000},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			weapon, stone, kinah := godstoneFixture(t, s, 2000000400+int32(index)*10, test.stoneCount)
			if _, err := s.DB.ExecContext(context.Background(), test.query, test.args(weapon, stone, kinah)...); err != nil {
				t.Fatal(err)
			}
			if err := s.SocketGodstone(context.Background(), weapon, stone, kinah, 100000); err == nil {
				t.Fatal("socketing succeeded with stale inventory")
			}
			for _, resource := range []struct {
				id    int32
				count int64
			}{{stone.UniqueID, test.stoneAfter}, {kinah.UniqueID, test.kinahAfter}} {
				var count int64
				if err := s.DB.QueryRowContext(context.Background(), "SELECT itemCount FROM inventory WHERE itemUniqueId = ?", resource.id).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != resource.count {
					t.Errorf("resource %d count %d after rollback, want %d", resource.id, count, resource.count)
				}
			}
			sockets, err := s.Stones(weapon.UniqueID)
			if err != nil {
				t.Fatal(err)
			}
			if len(sockets) != 2 || !slices.Contains(sockets, Stone{ItemID: 167000001}) ||
				!slices.Contains(sockets, Stone{ItemID: 168000002, Category: 1}) {
				t.Fatalf("socket rows changed after rollback: %+v", sockets)
			}
		})
	}
}

func TestSocketGodstoneInsertFailureRollsBack(t *testing.T) {
	s := godstoneTestStore(t)
	weapon, stone, kinah := godstoneFixture(t, s, 2000000500, 1)
	// Inject failure after both resource writes and removal of the old godstone.
	_, err := s.DB.ExecContext(context.Background(), `CREATE TRIGGER godstone_fixture_insert_failure BEFORE INSERT ON item_stones
 FOR EACH ROW BEGIN IF NEW.itemUniqueId = 2000000500 AND NEW.category = 1 THEN
 SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected godstone insert failure'; END IF; END`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := s.DB.ExecContext(context.Background(), `DROP TRIGGER godstone_fixture_insert_failure`); err != nil {
			t.Error(err)
		}
	})
	if err := s.SocketGodstone(context.Background(), weapon, stone, kinah, 100000); err == nil {
		t.Fatal("insert failure was ignored")
	}
	items, err := s.Items(weapon.Owner, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("rollback lost inventory: %+v", items)
	}
	for _, item := range items {
		if item.UniqueID == stone.UniqueID && item.Count != 1 || item.UniqueID == kinah.UniqueID && item.Count != kinah.Count || item.UniqueID == weapon.UniqueID && item.Godstone != 168000002 {
			t.Fatalf("rollback changed item: %+v", item)
		}
	}
	sockets, err := s.Stones(weapon.UniqueID)
	if err != nil || len(sockets) != 2 {
		t.Fatalf("rollback lost sockets: %+v, %v", sockets, err)
	}
	if err := s.DeleteStones(weapon.UniqueID); err != nil {
		t.Fatal(err)
	}
	sockets, err = s.Stones(weapon.UniqueID)
	if err != nil || len(sockets) != 1 || sockets[0].Category != 1 {
		t.Fatalf("manastone removal destroyed godstone: %+v, %v", sockets, err)
	}
}
