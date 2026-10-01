package game

import (
	"testing"

	"aionlightning/game/data"
)

func TestEquipAndUnequip(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	p.cube = nil
	s.spawn(p)
	staff := p.equippedIn(data.SlotMainHand)
	if staff == nil {
		t.Fatal("the mage carries no staff")
	}
	attack := p.stats.current(data.MaxDamages)

	s.visMu.Lock()
	defer s.visMu.Unlock()
	if s.unequipItem(p, staff.UniqueID) == nil || p.equippedIn(data.SlotMainHand) != nil || len(p.cube) != 1 {
		t.Fatalf("the staff wasn't put in the cube: %d items", len(p.cube))
	}
	if p.stats.current(data.MaxDamages) >= attack {
		t.Errorf("stats don't show the staff is gone: %d", p.stats.current(data.MaxDamages))
	}
	if s.equipItem(p, staff.UniqueID, 0) == nil || p.equippedIn(data.SlotMainHand) == nil || len(p.cube) != 0 {
		t.Fatalf("the staff wasn't worn again: %d items in the cube", len(p.cube))
	}
	if p.stats.current(data.MaxDamages) != attack {
		t.Errorf("stats after equipping: %d, want %d", p.stats.current(data.MaxDamages), attack)
	}
	if tap.count(smUpdateItem) < 2 || tap.count(smStatsInfo) < 2 {
		t.Errorf("the client heard %d item updates and %d stats infos", tap.count(smUpdateItem), tap.count(smStatsInfo))
	}
}
