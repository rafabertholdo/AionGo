package game

import "testing"

func TestBookletOnStigma(t *testing.T) {
	f := newChainFixture(t, 4935)
	f.start(4762)
	f.talk(279005, 10255) // out of order
	f.expectVars(0)
	f.talk(204285, 10000)
	f.expectVars(1)
	f.give(182207104, 3)
	f.expectWindow(f.talk(204285, 33), 10001)
	f.expectVars(1)
	f.give(182207104, 1)
	f.expectWindow(f.talk(204285, 33), 10000)
	f.expectVars(2)
	if f.s.countItems(f.p, 182207104) != 0 || f.s.countItems(f.p, 182207107) != 1 {
		t.Fatal("logs not taken or letter not given")
	}
	f.expectWindow(f.talk(279005, 25), 1693)
	f.talk(279005, 10255)
	if q := f.p.quest(4935); q.Status != "REWARD" || f.s.countItems(f.p, 182207107) != 0 || f.s.countItems(f.p, 182207108) != 1 {
		t.Fatalf("state %+v", q)
	}
	f.finish(204051)
}
