package game

import (
	"testing"

	"aionlightning/game/store"
)

func TestManastoneSuccessRates(t *testing.T) {
	want := []int{76, 57, 43, 33, 25, 19, 2, 2}
	for sockets, rate := range want {
		if got := manastoneSuccessRate(sockets); got != rate {
			t.Errorf("socket count %d: chance %d, want %d", sockets, got, rate)
		}
	}
}

func TestSocketManastoneUpdatesItemAndStats(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	var target *store.Item
	for _, item := range p.equipment {
		if template := d.Items[item.ItemID]; template != nil && template.IsWeapon() {
			target = item
			break
		}
	}
	if target == nil {
		t.Skip("no equipped weapon in the data")
	}
	var stoneID int32
	for id, template := range d.Items {
		if id >= 167000000 && id < 168000000 && len(template.Modifiers) != 0 {
			stoneID = id
			break
		}
	}
	if stoneID == 0 {
		t.Skip("no manastone with a stat modifier in the data")
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	stat := handStat(d.Items[stoneID].Modifiers[0].Stat, target.Slot)
	before := p.stats.current(stat)
	for attempt := 0; attempt < 100 && len(p.stones[target.UniqueID]) == 0; attempt++ {
		stone := &store.Item{UniqueID: int32(0x40000 + attempt), ItemID: stoneID, Count: 1, Owner: p.ID}
		p.cube = append(p.cube, stone)
		s.socketManastone(p, stone, target, nil)
	}
	if len(p.stones[target.UniqueID]) != 1 {
		t.Fatal("manastone did not socket after 100 attempts")
	}
	if got := p.stats.current(stat); got == before {
		t.Errorf("stat %s stayed %d after socketing", stat, got)
	}
}
