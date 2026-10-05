package game

import (
	"bytes"
	"encoding/hex"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

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

// Expected bytes follow the original 1.9 server's warehouse packet bytecode,
// including its per-packet kinah slot and ordinary-item encoding for stigmas.
func TestWarehouseItemPackets19(t *testing.T) {
	cases := []struct {
		name      string
		id        int32
		equipment string
		details   string
	}{
		{"ordinary", 160010002, "", "1600003412050000000000000000000000000000000000002301"},
		{"stigma", 140000001, "", "1600003412050000000000000000000000000000000000002301"},
		{"weapon", 100000001, "WEAPON", "4b0006000000000100000000020000000b000001e1f5050000000000000000000000000000000000000000000000000000000000000000003412050000000000000000000000000000000000002301"},
		{"armor", 110000001, "ARMOR", "4f000600000000020000000000000000000000000b000081778e060000000000000000000000000000000000000000000000000000000000000000003412050000000000000000000000000000000000002301"},
		{"kinah", data.Kinah, "", "160000341205000000000000000000000000000000000000ffff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := &data.ItemTemplate{ID: tc.id, NameID: 0x12345678, Mask: 0x1234, EquipmentType: tc.equipment}
			s := &Server{data: &data.Data{Items: map[int32]*data.ItemTemplate{tc.id: tmpl}}}
			p := &player{}
			item := &store.Item{UniqueID: 0x10203040, ItemID: tc.id, Count: 5, Slot: 0x123}
			details, err := hex.DecodeString(tc.details)
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range []byte{storageWarehouse, storageAccount} {
				var got *wire.Writer
				p.conn = &conn{s: s, tap: func(w *wire.Writer) { got = w }}
				header := wire.Packet(0)
				header.D(item.UniqueID)
				header.D(item.ItemID)
				header.C(0)
				header.H(0x24)
				header.D(tmpl.NameID)
				header.H(0)
				want := wire.Packet(smWarehouseInfo)
				want.C(kind)
				want.C(1)
				want.C(3)
				want.H(0)
				want.H(2)
				// Two consecutive entries expose any detail bytes that shift the next item.
				want.B(header.Data[1:])
				want.B(details)
				want.B(header.Data[1:])
				want.B(details)
				got = s.warehouseInfo(p, []*store.Item{item, item}, kind, 3, true)
				if !bytes.Equal(got.Data, want.Data) {
					t.Errorf("load kind %d: got %x want %x", kind, got.Data, want.Data)
				}
				want = wire.Packet(smWarehouseUpdate)
				want.C(kind)
				want.H(13)
				want.H(1)
				want.B(header.Data[1:])
				addedDetails := bytes.Clone(details)
				if tc.id == data.Kinah {
					addedDetails[len(addedDetails)-1] = 0
				}
				want.B(addedDetails)
				s.sendAdded(p, kind, item)
				if !bytes.Equal(got.Data, want.Data) {
					t.Errorf("add kind %d: got %x want %x", kind, got.Data, want.Data)
				}
				want = wire.Packet(smUpdateWarehouseItem)
				want.D(item.UniqueID)
				want.C(kind)
				want.H(0x24)
				want.D(tmpl.NameID)
				want.H(0)
				want.B(details)
				s.sendUpdated(p, kind, item)
				if !bytes.Equal(got.Data, want.Data) {
					t.Errorf("update kind %d: got %x want %x", kind, got.Data, want.Data)
				}
			}
		})
	}
}
