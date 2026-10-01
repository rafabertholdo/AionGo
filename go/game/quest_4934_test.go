package game

import "testing"

func TestShulacksStigma(t *testing.T) {
	f := newChainFixture(t, 4934)
	f.start(4762)
	if f.s.data.QuestActions[700562] == nil {
		t.Fatal("loot object 700562 is not registered")
	}
	f.talk(204285, 10001) // out of order
	f.expectVars(0)
	f.talk(204211, 10000)
	f.talk(204285, 10001)
	f.expectVars(2)
	f.expectWindow(f.talk(204285, 25), 1693)
	f.expectWindow(f.talk(204285, 33), 10001) // no Surkana
	f.expectVars(2)
	f.give(182207102, 1)
	f.expectWindow(f.talk(204285, 33), 10000)
	if q := f.p.quest(4934); q.Status != "REWARD" || f.s.countItems(f.p, 182207102) != 0 {
		t.Fatalf("state %+v surkana=%d", q, f.s.countItems(f.p, 182207102))
	}
	f.finish(204051)
}
