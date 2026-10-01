package game

import "testing"

func TestSexiestManAlive(t *testing.T) {
	f := newChainFixture(t, 3969)
	f.p.Gender = "FEMALE"
	for _, npc := range []int32{798390, 798391} {
		found := false
		for _, s := range f.s.data.QuestCustomTalks[npc] {
			found = found || s.ID == 3969
		}
		if !found {
			t.Fatalf("npc %d not registered for plain clicks", npc)
		}
	}
	palentine := questCatalogNPC(f.s, f.p, 798390, 0x40001)
	if !f.c.talkChainClick(palentine, f.script, talkChains[3969]) {
		t.Fatal("plain click at Palentine showed nothing")
	}
	f.expectWindow(palentine, 1011)
	f.talk(798390, 1002)
	f.expectVars(0)
	if f.s.countItems(f.p, 182206126) != 1 {
		t.Fatal("letter not given")
	}
	andu := questCatalogNPC(f.s, f.p, 798391, 0x40002)
	f.c.talkChainClick(andu, f.script, talkChains[3969])
	f.expectWindow(andu, 1352)
	f.talk(798391, 10000)
	if q := f.p.quest(3969); q.Status != "REWARD" || f.s.countItems(f.p, 182206126) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.c.talkChainShowDialog(palentine, f.script, talkChains[3969])
	f.expectWindow(palentine, 2375)
	f.finish(798390)
}

func TestSexiestManAliveNeedsLetter(t *testing.T) {
	f := newChainFixture(t, 3969)
	f.p.Gender = "FEMALE"
	f.talk(798390, 1002)
	f.s.removeItemsByID(f.p, 182206126, 1)
	f.talk(798391, 10000)
	f.expectVars(0)
}
