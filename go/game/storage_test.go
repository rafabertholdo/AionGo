package game

import "testing"

// TestWarehouse moves items between the cube and the warehouse, splits and merges stacks, and moves kinah.
func TestWarehouse(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	const potion = 160010002 // stacks to 1000
	if d.Items[potion] == nil {
		t.Skip("no item to store")
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, potion, 10)
	item := p.cube[0]
	s.moveItem(p, item.UniqueID, storageCube, storageWarehouse, 0)
	if len(p.cube) != 0 || len(p.warehouse) != 1 || p.warehouse[0].Location != storageWarehouse {
		t.Fatalf("cube %d, warehouse %d", len(p.cube), len(p.warehouse))
	}
	if tap.count(smDeleteItem) != 1 || tap.count(smWarehouseUpdate) != 1 {
		t.Errorf("packets: %v", tap.counts)
	}
	// Splitting 4 off it makes a new item, and merging them makes one again.
	s.split(p, item.UniqueID, 4, 5, storageWarehouse, storageWarehouse)
	if len(p.warehouse) != 2 || item.Count != 6 {
		t.Fatalf("after the split: %d items, %d in the first", len(p.warehouse), item.Count)
	}
	s.merge(p, p.warehouse[1].UniqueID, 4, item.UniqueID, storageWarehouse, storageWarehouse)
	if len(p.warehouse) != 1 || item.Count != 10 {
		t.Fatalf("after the merge: %d items, %d in the first", len(p.warehouse), item.Count)
	}
	// Back in the cube, and kinah goes the same way.
	s.moveItem(p, item.UniqueID, storageWarehouse, storageCube, 3)
	if len(p.cube) != 1 || len(p.warehouse) != 0 || p.cube[0].Count != 10 {
		t.Fatalf("back in the cube: %d, %d", len(p.cube), len(p.warehouse))
	}
	p.kinah.Count = 500
	s.moveKinah(p, storageCube, storageWarehouse, 200)
	if p.kinah.Count != 300 || p.warehouseKinah == nil || p.warehouseKinah.Count != 200 {
		t.Errorf("kinah %d and %v", p.kinah.Count, p.warehouseKinah)
	}
	s.removeItem(p, p.cube[0])
	if len(p.cube) != 0 {
		t.Errorf("the item wasn't deleted")
	}
}
