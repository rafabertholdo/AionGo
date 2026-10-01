package game

import "testing"

func TestSecretOfTheGreaterStigma(t *testing.T) {
	f := newChainFixture(t, 4936)
	f.start(1011)
	f.talk(204051, 33) // Vergelmir too early
	f.expectVars(0)
	f.talk(204837, 10000)
	f.expectVars(1)
	f.expectWindow(f.talk(204051, 25), 2375)
	f.give(182207109, 100)
	f.expectWindow(f.talk(204051, 33), 2716) // no Greater Stigma
	f.give(182207110, 1)
	f.expectWindow(f.talk(204051, 33), 5)
	if q := f.p.quest(4936); q.Status != "REWARD" || f.s.countItems(f.p, 182207109) != 0 || f.s.countItems(f.p, 182207110) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.finish(204051)
}
