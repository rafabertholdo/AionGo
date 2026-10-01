package game

import "testing"

// TestAccountWarehouse puts items and money in the account warehouse, and a second character of the account sees them.
func TestAccountWarehouse(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	const potion = 160010002
	if d.Items[potion] == nil {
		t.Skip("no item to store")
	}
	p.AccountID = 77
	p.acctWH = &accountWarehouse{}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, potion, 10)
	item := p.cube[0]
	s.moveItem(p, item.UniqueID, storageCube, storageAccount, 0)
	if len(p.cube) != 0 || len(p.acctWH.items) != 1 || item.Location != storageAccount || item.Owner != 77 {
		t.Fatalf("cube %d, account %d, location %d, owner %d", len(p.cube), len(p.acctWH.items), item.Location, item.Owner)
	}
	if tap.count(smWarehouseUpdate) != 1 {
		t.Errorf("packets: %v", tap.counts)
	}
	p.kinah.Count = 500
	s.moveKinah(p, storageCube, storageAccount, 200)
	if p.kinah.Count != 300 || p.acctWH.kinah == nil || p.acctWH.kinah.Count != 200 || p.acctWH.kinah.Owner != 77 {
		t.Errorf("kinah %d and %v", p.kinah.Count, p.acctWH.kinah)
	}
	// A character of the same account shares the warehouse.
	other, _ := fighter(t, s, 1000)
	other.ID, other.AccountID, other.acctWH = 0x20001, 77, p.acctWH
	s.moveItem(other, item.UniqueID, storageAccount, storageCube, 2)
	if len(p.acctWH.items) != 0 || len(other.cube) != 1 || other.cube[0].Owner != 0x20001 || other.cube[0].Location != storageCube {
		t.Fatalf("taken out: %d in the warehouse, %d carried", len(p.acctWH.items), len(other.cube))
	}
	// The info lists the items with the money last.
	s.moveItem(other, other.cube[0].UniqueID, storageCube, storageAccount, 0)
	if got := p.acctWH.all(); got[len(got)-1] != p.acctWH.kinah {
		t.Errorf("money isn't last")
	}
	tap.counts = map[byte]int{}
	s.sendWarehouseInfo(p, true)
	if tap.count(smWarehouseInfo) != 3 {
		t.Errorf("info packets: %v", tap.counts)
	}
}
