package game

import (
	"slices"
	"testing"
	"time"

	"aionlightning/wire"
)

// TestSoulHealingAndExpansion has a player buy back its lost experience, and expand its cube, from npcs' dialogs.
func TestSoulHealingAndExpansion(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	var npc int32 = -1
	for _, id := range slices.Sorted(mapKeys(d.CubeExpand)) {
		if d.Npcs[id] != nil && d.CubeExpand[id][1] != 0 {
			npc = id
			break
		}
	}
	if npc < 0 {
		t.Skip("no npc expands the cube")
	}
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	o.npc = d.Npcs[npc]
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.kinah.Count = 1_000_000
	p.RecoverExp = 1000
	s.visMu.Unlock()

	answer := func(code int32, yes byte) {
		p.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(code); w.C(yes) }))
	}
	dialog := func(id uint16) {
		p.conn.serviceDialog(o.id, id)
	}
	dialog(dialogSoulHealing)
	answer(questionSoulHealing, 1)
	if p.RecoverExp != 0 || p.kinah.Count >= 1_000_000 {
		t.Errorf("recoverable %d, kinah %d", p.RecoverExp, p.kinah.Count)
	}
	dialog(dialogExpandCube)
	answer(questionExpandWarehouse, 1)
	if p.CubeSize != 1 || tap.count(smCubeUpdate) != 1 {
		t.Errorf("cube size %d, %d updates", p.CubeSize, tap.count(smCubeUpdate))
	}
}

func mapKeys[V any](m map[int32]V) func(yield func(int32) bool) {
	return func(yield func(int32) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// TestRegularTeleporter has a player pay a teleporter and arrive where it goes.
func TestRegularTeleporter(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	var npc, loc int32
	for _, id := range slices.Sorted(mapKeys(d.Teleporters)) {
		tp := d.Teleporters[id]
		if d.Npcs[id] == nil || tp.Type != "REGULAR" || tp.TeleportID == 0 || len(tp.Locations) == 0 || d.Npcs[id].Race == "ASMODIANS" {
			continue
		}
		if place := d.TeleLocations[tp.Locations[0].LocID]; place != nil && place.MapID != 210010000 {
			npc, loc = id, tp.Locations[0].LocID
			break
		}
	}
	if npc == 0 {
		t.Skip("no regular teleporter for the elyos")
	}
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	o.npc = d.Npcs[npc]
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.kinah.Count = 100000
	s.showTeleportMap(p, o)
	s.visMu.Unlock()
	if tap.count(smTeleportMap) != 1 {
		t.Fatalf("no map was shown")
	}
	p.conn.teleportSelect(packet(cmTeleportSelect, func(w *wire.Writer) { w.D(o.id); w.D(loc) }))
	if tap.count(smTeleportLoc) != 1 || p.kinah.Count == 100000 {
		t.Fatalf("no teleport: %d packets, %d kinah", tap.count(smTeleportLoc), p.kinah.Count)
	}
	time.Sleep(2500 * time.Millisecond)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if want := d.TeleLocations[loc].MapID; p.WorldID != want {
		t.Errorf("in map %d, want %d", p.WorldID, want)
	}
}

// TestCraftMaster has a master teach cooking to a player.
func TestCraftMaster(t *testing.T) {
	d := staticDataOrSkip(t)
	if d.Npcs[203784] == nil {
		t.Skip("no cooking master in the data")
	}
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	o.npc = d.Npcs[203784]
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.level = 10
	p.kinah.Count = 10000
	s.visMu.Unlock()
	p.conn.serviceDialog(o.id, dialogCraftMaster)
	p.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionCraftSkill); w.C(1) }))
	if !p.hasSkill(40001) || p.kinah.Count != 10000-3500 {
		t.Errorf("skills %v, kinah %d", p.skills, p.kinah.Count)
	}
}

// TestDialogHandlersAreWrapped checks the client's dialog select and show dialog reach the services first: the
// handlers are registered by init functions, whose order is that of the files' names.
func TestDialogHandlersAreWrapped(t *testing.T) {
	d := staticDataOrSkip(t)
	var npc int32
	for _, id := range slices.Sorted(mapKeys(d.TradeLists)) {
		if l := d.TradeLists[id]; d.Npcs[id] != nil && !l.Abyss && len(l.Tabs) > 0 {
			npc = id
			break
		}
	}
	if npc == 0 {
		t.Skip("no shop")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	o.npc = d.Npcs[npc]
	s.visMu.Lock()
	s.updateNpcKnown(o)
	s.visMu.Unlock()
	handlers[cmDialogSelect](p.conn, packet(cmDialogSelect, func(w *wire.Writer) { w.D(o.id); w.H(dialogBuy); w.H(0); w.H(0); w.D(0) }))
	if tap.count(smTradelist) != 1 {
		t.Errorf("the shop's dialog didn't reach the shop: %v", tap.counts)
	}
}
