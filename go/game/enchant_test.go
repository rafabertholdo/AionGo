package game

import (
	"slices"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// TestEnchant has a player enchant the staff it wears with stones: each success adds to its attack.
func TestEnchant(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	staff := p.equipment[0]
	for _, item := range p.equipment {
		if item.ItemID == 100600034 {
			staff = item
		}
	}
	tmpl := d.Items[staff.ItemID]
	var stone int32
	for _, id := range slices.Sorted(mapKeys(d.Items)) {
		if id > 166000000 && id < 167000000 && d.Items[id].Level >= tmpl.Level && stone == 0 {
			stone = id
		}
	}
	if tmpl == nil || stone == 0 {
		t.Skip("no staff or enchant stone in the data")
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	before := p.stats.current(data.MainHandPower)
	s.addItem(p, stone, 60)
	for range 60 {
		if staff.Enchant == 1 || p.cube[0].Count == 0 {
			break
		}
		s.enchantItem(p, s.cubeStone(p, stone), staff)
	}
	if staff.Enchant != 1 {
		t.Fatalf("the staff is +%d", staff.Enchant)
	}
	if got := p.stats.current(data.MainHandPower); got <= before {
		t.Errorf("attack %d, was %d", got, before)
	}
}

// cubeStone is the stack of the item in the cube.
func (s *Server) cubeStone(p *player, id int32) *store.Item {
	for _, item := range p.cube {
		if item.ItemID == id {
			return item
		}
	}
	return nil
}
