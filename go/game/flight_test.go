package game

import (
	"slices"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// TestFlying has a player fly for a while: the fly time runs out, and it comes back after landing.
func TestFlying(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, tap := fighter(t, s, 1000)
	s.visMu.Lock()
	fp := p.life.FP
	s.startFly(p)
	if !p.inState(stateFlying) || p.flyState != 1 {
		t.Fatalf("state %d, fly state %d", p.state, p.flyState)
	}
	s.visMu.Unlock()
	time.Sleep(3500 * time.Millisecond)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.life.FP >= fp || tap.count(smFlyTime) == 0 {
		t.Errorf("fp %d, was %d", p.life.FP, fp)
	}
	s.switchToGliding(p)
	if p.flyState != 2 || !p.inState(stateGliding) {
		t.Errorf("not gliding: %d", p.flyState)
	}
	s.stopGliding(p)
	s.endFly(p)
	if p.inState(stateFlying) || p.flyState != 0 || p.fpTask == nil {
		t.Errorf("didn't land: state %d, fly state %d, task %v", p.state, p.flyState, p.fpTask)
	}
}

// TestFlightTeleporter has a player pay a flight teleporter, fly the client's path and land.
func TestFlightTeleporter(t *testing.T) {
	d := staticDataOrSkip(t)
	var npc int32
	var dest *data.Destination
	for _, id := range slices.Sorted(mapKeys(d.Teleporters)) {
		tp := d.Teleporters[id]
		if d.Npcs[id] != nil && tp.Type == "FLIGHT" && len(tp.Locations) > 0 && d.Npcs[id].Race != "ASMODIANS" && d.TeleLocations[tp.Locations[0].LocID] != nil {
			npc, dest = id, &tp.Locations[0]
			break
		}
	}
	if npc == 0 {
		t.Skip("no flight teleporter for the elyos")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	o.npc = d.Npcs[npc]
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.kinah.Count = 100000
	s.visMu.Unlock()
	p.conn.teleportSelect(packet(cmTeleportSelect, func(w *wire.Writer) { w.D(o.id); w.D(dest.LocID) }))
	s.visMu.Lock()
	if !p.usingFlyTeleport() || p.flightTeleportID != dest.TeleportID || p.state&stateActive != 0 || p.kinah.Count == 100000 || tap.count(smEmotion) == 0 {
		t.Fatalf("no flight: state %d, path %d, kinah %d", p.state, p.flightTeleportID, p.kinah.Count)
	}
	s.visMu.Unlock()
	p.conn.flightTeleport(packet(cmFlightTeleport, func(w *wire.Writer) {
		w.D(p.WorldID)
		w.F(10)
		w.F(20)
		w.F(30)
		w.C(0)
		w.D(77)
	}))
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.X != 10 || p.Y != 20 || p.flightDistance != 77 {
		t.Errorf("at %v,%v distance %d", p.X, p.Y, p.flightDistance)
	}
	s.endFlightTeleport(p)
	if p.usingFlyTeleport() || p.inState(stateFlying) || p.state&stateActive == 0 || p.flightDistance != 0 {
		t.Errorf("didn't land: state %d", p.state)
	}
}
