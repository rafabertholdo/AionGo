package game

import "testing"

// TestKisk has Ann set up a kisk, Bob bind to it, die and rise at it, and the kisk be destroyed.
func TestKisk(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, _, bobTap := twoPlayers(t, s)
	ann.Race, bob.Race = "ELYOS", "ELYOS"
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := s.spawnKisk(ann, 700118)
	if o == nil {
		t.Skip("kisk 700118 isn't in the static data")
	}
	s.updateNpcKnown(o)
	bob.seen[o.id] = o
	if !s.canBind(o, bob) {
		t.Fatalf("Bob can't bind to a kisk of his race")
	}
	s.bindToKisk(o, bob)
	if bob.kisk != o || len(o.kisk.members) != 1 || bobTap.count(smSetBindPoint) != 1 {
		t.Fatalf("Bob isn't bound: %v", bobTap.counts)
	}
	bob.dead = true
	o.kisk.resurrects = 2
	s.kiskRevive(bob)
	if bob.kisk != o || o.kisk.resurrects != 1 {
		t.Errorf("Bob rose wrongly: %d resurrections", o.kisk.resurrects)
	}
	bob.Race = "ASMODIANS"
	if s.canBind(o, bob) {
		t.Errorf("an Asmodian bound to an Elyos kisk")
	}
	bob.Race = "ELYOS"
	s.reduceNpcHP(o, o.maxHP, ann)
	if bob.kisk != nil || len(s.kisks) != 0 || bobTap.count(smSetBindPoint) != 2 {
		t.Errorf("the kisk didn't let go: %v", bobTap.counts)
	}
}
