package game

import (
	"slices"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// twoOf is two ids of items of the same weapon type, or of the same armor type and slot, that aren't epic, the first no lower than the second.
func twoOf(d *data.Data, weapon bool) (int32, int32) {
	type key struct {
		kind string
		slot int32
	}
	seen := map[key]int32{}
	for _, id := range slices.Sorted(mapKeys(d.Items)) {
		t := d.Items[id]
		if t.Quality == "EPIC" || t.Quality == "MYTHIC" || t.Level < 1 || t.Level > 30 || t.Restrict == [12]int32{} {
			continue
		}
		k := key{t.ArmorType, t.Slot}
		if t.IsWeapon() {
			k = key{t.WeaponType, 0}
		} else if !t.IsArmor() || t.ArmorType == "CLOTHES" || weapon {
			continue
		}
		if other, ok := seen[k]; ok && other != id && d.Items[other].Level >= t.Level {
			return other, id
		}
		seen[k] = id
	}
	return 0, 0
}

func remodelPlayer(t *testing.T, weapon bool) (*Server, *player, *tapped, int32, int32) {
	d := staticDataOrSkip(t)
	a, b := twoOf(d, weapon)
	if a == 0 {
		t.Skip("no two matching items")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	p.level = 50
	p.kinah.Count = 100000
	return s, p, tap, a, b
}

func cubeItemOf(p *player, id int32) *store.Item {
	for _, item := range p.cube {
		if item.ItemID == id {
			return item
		}
	}
	return nil
}

// TestRemodelItem has an item take another's look for a fee and then lose it to a pattern reshaper.
func TestRemodelItem(t *testing.T) {
	s, p, _, a, b := remodelPlayer(t, false)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, a, 1)
	s.addItem(p, b, 1)
	s.addItem(p, patternReshaper, 1)
	keep, extract := cubeItemOf(p, a), cubeItemOf(p, b)
	keepID, extractID := keep.UniqueID, extract.UniqueID
	s.remodelItem(p, keepID, extractID)
	if keep.Skin != b || p.kinah.Count != 100000-remodelPrice || cubeItemOf(p, b) != nil {
		t.Fatalf("skin %d (want %d), kinah %d, extract still there %v", keep.Skin, b, p.kinah.Count, cubeItemOf(p, b) != nil)
	}
	s.remodelItem(p, keepID, cubeItemOf(p, patternReshaper).UniqueID)
	if keep.Skin != 0 || p.kinah.Count != 100000-2*remodelPrice || cubeItemOf(p, patternReshaper) != nil {
		t.Errorf("skin %d, kinah %d after the reshaper", keep.Skin, p.kinah.Count)
	}
	// A player below level 20 pays nothing and changes nothing.
	s.addItem(p, b, 1)
	p.level = 19
	s.remodelItem(p, keepID, cubeItemOf(p, b).UniqueID)
	if keep.Skin != 0 || p.kinah.Count != 100000-2*remodelPrice {
		t.Errorf("remodelled at level 19: skin %d, kinah %d", keep.Skin, p.kinah.Count)
	}
}

// TestFuseAndBreakWeapons has a weapon fuse with another at an npc, and be separated away from one.
func TestFuseAndBreakWeapons(t *testing.T) {
	s, p, _, a, b := remodelPlayer(t, true)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, a, 1)
	s.addItem(p, b, 1)
	first, second := cubeItemOf(p, a), cubeItemOf(p, b)
	firstID, secondID := first.UniqueID, second.UniqueID
	s.fuseWeapons(p, firstID, secondID)
	if first.Fusioned != 0 {
		t.Fatal("fused with no npc targeted")
	}
	o := monster(t, s, 1005)
	p.targetID = o.id
	s.fuseWeapons(p, firstID, secondID)
	if first.Fusioned != b || cubeItemOf(p, b) != nil {
		t.Fatalf("fused item %d (want %d), second still there %v", first.Fusioned, b, cubeItemOf(p, b) != nil)
	}
	s.breakWeapons(p, firstID)
	if first.Fusioned != b {
		t.Fatal("separated at an npc")
	}
	p.targetID = 0
	s.breakWeapons(p, firstID)
	if first.Fusioned != 0 {
		t.Errorf("still fused with %d", first.Fusioned)
	}
}
