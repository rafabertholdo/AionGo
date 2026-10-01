package game

import "testing"

func TestLuckAndPersistence(t *testing.T) {
	f := newChainFixture(t, 4943)
	f.start(4762)
	f.talk(204096, 10000)
	f.expectVars(1)
	f.talk(204097, 1354) // no kinah
	f.expectWindow(f.talk(204097, 1354), 1438)
	f.expectVars(1)
	f.s.increaseKinah(f.p, 3400001)
	f.talk(204097, 1354)
	f.expectVars(2)
	if f.p.kinah.Count != 1 || f.s.countItems(f.p, 182207123) != 1 {
		t.Fatalf("kinah=%d vessel=%d", f.p.kinah.Count, f.s.countItems(f.p, 182207123))
	}
	f.give(182207124, 19)
	f.expectWindow(f.talk(204096, 33), 10001)
	f.give(182207124, 1)
	f.expectWindow(f.talk(204096, 33), 10000)
	f.expectVars(3)
	f.purify(204075, 186000085, 2120, 2034)
	f.finish(204053)
}
