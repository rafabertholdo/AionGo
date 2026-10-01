package game

import "testing"

func TestPriceOfGoodwill(t *testing.T) {
	f := newChainFixture(t, 3200)
	f.start(4762)
	const scroll = 182209082
	if f.s.data.QuestItemUses[scroll] == nil || f.s.data.QuestActions[700522] == nil {
		t.Fatal("scroll or bag not registered")
	}
	f.expectWindow(f.talk(204658, 25), 1003)
	f.talk(204658, 10000)
	f.expectVars(1)
	f.expectWindow(f.talk(798332, 25), 1352)
	f.talk(798332, 10001)
	f.expectVars(2)
	f.give(scroll, 1)
	item := f.p.cube[len(f.p.cube)-1]
	f.c.suspiciousCallItemDone(item, f.script)
	f.expectVars(3)
	if f.s.countItems(f.p, scroll) != 0 {
		t.Fatal("scroll not consumed")
	}
	f.expectWindow(f.talk(279006, 25), 2034)
	f.talk(279006, 10255)
	if q := f.p.quest(3200); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	f.expectWindow(f.talk(798322, 1009), 5)
	f.finish(798322)
}
