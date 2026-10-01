package game

import "testing"

// TestRift opens a rift, has Ann see it and go through, and the last entry close it.
func TestRift(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	kind := riftKinds[0]
	if kind.master != "ELTNEN_AM" || riftKinds[27].slave != "HEIRON_GS" || riftKinds[27].destination != "ELYOS" {
		t.Fatalf("wrong rift table: %v %v", kind, riftKinds[27])
	}
	_, world, at, ok := s.riftSpot(kind.master)
	if !ok {
		t.Skip("rifts aren't in the static data")
	}
	for _, p := range []*player{ann, bob} {
		s.despawnLocked(p)
		p.WorldID, p.X, p.Y, p.Z = world, at[0]+3, at[1], at[2]
		s.spawnLocked(p)
	}
	master := s.spawnRift(kind)
	if master == nil || master.rift.slave == nil || master.rift.slave.worldID != 220020000 {
		t.Fatalf("rift didn't open: %v", master)
	}
	if annTap.count(smRiftAnnounce) != 1 || annTap.count(smRiftStatus) != 1 || master.rift.slave.rift.master {
		t.Errorf("Ann wasn't told: %v", annTap.counts)
	}
	ann.seen[master.id], bob.seen[master.id] = master, master
	master.rift.kind.entries = 2
	s.useRift(master, ann, true)
	if ann.WorldID != 220020000 || master.rift.used != 1 || !master.rift.accepting || bobTap.count(smRiftStatus) != 2 {
		t.Errorf("Ann didn't go through: world %d used %d", ann.WorldID, master.rift.used)
	}
	s.useRift(master, bob, true)
	if master.rift.accepting || master.rift.used != 2 {
		t.Errorf("the rift is still open: used %d", master.rift.used)
	}
	s.useRift(master, bob, true)
	if master.rift.used != 2 {
		t.Errorf("a third went through a full rift")
	}
	s.despawnOwned(master)
	if s.byID[master.id] != nil {
		t.Errorf("the rift stayed")
	}
}
