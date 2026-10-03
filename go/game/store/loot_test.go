package store

import (
	"context"
	"database/sql"
	"math"
	"os"
	"reflect"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestReceiveLootValidatesInventory(t *testing.T) {
	item := Item{UniqueID: 1, ItemID: 160000001, Owner: 2, Count: 3}
	tests := []struct {
		name   string
		change func(*Item)
	}{
		{"wrong owner", func(i *Item) { i.Owner++ }},
		{"warehouse", func(i *Item) { i.Location = 1 }},
		{"equipped", func(i *Item) { i.Equipped = true }},
		{"invalid unique ID", func(i *Item) { i.UniqueID = 0 }},
		{"invalid template ID", func(i *Item) { i.ItemID = 0 }},
		{"kinah", func(i *Item) { i.ItemID = 182400001 }},
		{"zero count", func(i *Item) { i.Count = 0 }},
		{"excess count", func(i *Item) { i.Count = math.MaxInt32 + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := item
			test.change(&invalid)
			if err := (Store{}).ReceiveLoot(context.Background(), item.Owner, nil, []Item{invalid}); err == nil {
				t.Fatal("accepted invalid inventory row")
			}
		})
	}
	for _, count := range []int64{item.Count - 1, item.Count, math.MaxInt32 + 1} {
		if err := (Store{}).ReceiveLoot(context.Background(), item.Owner, []LootStack{{Before: item, Count: count}}, nil); err == nil {
			t.Errorf("accepted stack increase to %d", count)
		}
	}
	if err := (Store{}).ReceiveLoot(context.Background(), 0, nil, []Item{item}); err == nil {
		t.Error("accepted invalid recipient")
	}
	if err := (Store{}).ReceiveLoot(context.Background(), item.Owner, []LootStack{{Before: item, Count: 4}}, []Item{item}); err == nil {
		t.Error("accepted duplicate update and insert row")
	}
	for _, empty := range []struct {
		updates []LootStack
		inserts []Item
	}{{}, {updates: []LootStack{}, inserts: []Item{}}} {
		if err := (Store{}).ReceiveLoot(context.Background(), 0, empty.updates, empty.inserts); err != nil {
			t.Errorf("empty receipt failed: %v", err)
		}
	}
}

func lootTestStore(t *testing.T) Store {
	t.Helper()
	dsn := os.Getenv("AION_LOOT_TEST_DSN")
	if dsn == "" {
		t.Skip("AION_LOOT_TEST_DSN isn't set")
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

// These tests insert their own rows in a disposable database with the existing schema.
func TestReceiveLootCommitsAndRollsBack(t *testing.T) {
	s := lootTestStore(t)
	tests := []struct {
		name  string
		query string
	}{
		{name: "success"},
		{name: "late duplicate insert"},
		{name: "stale second stack", query: "UPDATE inventory SET itemCount = itemCount + 1 WHERE itemUniqueId = ?"},
		{name: "second stack owner", query: "UPDATE inventory SET itemOwner = itemOwner + 1 WHERE itemUniqueId = ?"},
		{name: "second stack location", query: "UPDATE inventory SET itemLocation = 1 WHERE itemUniqueId = ?"},
		{name: "second stack equipped", query: "UPDATE inventory SET isEquiped = 1 WHERE itemUniqueId = ?"},
		{name: "second stack template", query: "UPDATE inventory SET itemId = itemId + 1 WHERE itemUniqueId = ?"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := int32(2000000700 + index*10)
			owner := base + 9
			first := Item{UniqueID: base, ItemID: 160000001, Owner: owner, Count: 2}
			second := Item{UniqueID: base + 1, ItemID: 160000002, Owner: owner, Count: 3}
			collision := Item{UniqueID: base + 2, ItemID: 160000003, Owner: owner, Count: 1}
			for _, item := range []Item{first, second, collision} {
				if err := s.InsertItem(&item); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := s.DeleteItem(item.UniqueID); err != nil {
						t.Error(err)
					}
				})
			}
			if test.query != "" {
				if _, err := s.DB.ExecContext(context.Background(), test.query, second.UniqueID); err != nil {
					t.Fatal(err)
				}
			}
			counts := func() map[int32]int64 {
				t.Helper()
				rows, err := s.DB.QueryContext(context.Background(), "SELECT itemUniqueId, itemCount FROM inventory WHERE itemUniqueId BETWEEN ? AND ?", base, base+3)
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				result := make(map[int32]int64)
				for rows.Next() {
					var id int32
					var count int64
					if err := rows.Scan(&id, &count); err != nil {
						t.Fatal(err)
					}
					result[id] = count
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				return result
			}
			before := counts()
			inserted := Item{UniqueID: base + 3, ItemID: 160000004, Owner: owner, Count: 4, Slot: 65535}
			inserts := []Item{inserted}
			if test.name == "late duplicate insert" {
				inserts = append(inserts, collision)
			}
			err := s.ReceiveLoot(context.Background(), owner,
				[]LootStack{{Before: first, Count: 5}, {Before: second, Count: 6}}, inserts)
			if test.name != "success" {
				if err == nil {
					t.Fatal("accepted conflicting loot receipt")
				}
				if after := counts(); !reflect.DeepEqual(after, before) {
					t.Fatalf("receipt changed inventory after rollback: before %v, after %v", before, after)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := s.DeleteItem(inserted.UniqueID); err != nil {
					t.Error(err)
				}
			})
			items, err := s.Items(owner, 0, false)
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[int32]int64)
			for _, item := range items {
				got[item.UniqueID] = item.Count
			}
			want := map[int32]int64{first.UniqueID: 5, second.UniqueID: 6, collision.UniqueID: 1, inserted.UniqueID: 4}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("reloaded inventory %v, want %v", got, want)
			}
		})
	}
}
