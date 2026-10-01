package game

import "testing"

func TestProvingGround(t *testing.T) {
	f := newChainFixture(t, 4939)
	f.talk(204055, 10000)
	if f.p.quest(4939) != nil {
		t.Fatal("Njord advanced a quest that was not started")
	}
	f.start(4762)
	f.talk(204273, 10001) // out of order
	f.expectVars(0)
	f.expectWindow(f.talk(204055, 25), 1011)
	f.talk(204055, 10000)
	f.talk(204055, 10000) // repeat must not advance again
	f.expectVars(1)
	f.talk(204273, 10001)
	f.talk(204054, 10002)
	f.expectVars(3)
	f.give(186000079, 29)
	f.expectWindow(f.talk(204054, 33), 10001)
	f.expectVars(3)
	f.give(186000079, 1)
	f.expectWindow(f.talk(204054, 33), 10000)
	f.expectVars(4)
	if f.s.countItems(f.p, 186000079) != 0 {
		t.Fatal("medals were not taken")
	}
	f.purify(204075, 186000085, 2461, 2375)
	f.expectWindow(f.talk(204053, 1009), 5)
	f.finish(204053)
}
