package game

import "testing"

func TestZombiesDescendant(t *testing.T) {
	f := newChainFixture(t, 4060)
	f.talk(205156, 10000)
	if f.p.quest(4060) != nil {
		t.Fatal("advanced an unstarted quest")
	}
	f.give(182209037, 1)
	if !f.c.itemStartedQuestDialog(f.script, 1002) {
		t.Fatal("item start refused")
	}
	f.expectVars(0)
	f.talk(204143, 10001) // out of order
	f.expectVars(0)
	f.talk(205156, 10000)
	f.talk(204143, 10001)
	f.talk(204731, 10002)
	f.expectVars(3)
	f.expectWindow(f.talk(205204, 25), 2375)
	f.expectWindow(f.talk(205204, 1009), 5)
	if q := f.p.quest(4060); q.Status != "REWARD" || f.s.countItems(f.p, 182209037) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.finish(205204)
}
