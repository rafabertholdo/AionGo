package game

import "testing"

func TestASuspiciousCall(t *testing.T) {
	f := newChainFixture(t, 4200)
	f.start(4762)
	if f.s.data.QuestItemUses[suspiciousCallItem] == nil || f.s.data.QuestActions[700522] == nil {
		t.Fatal("scroll or bag not registered")
	}
	f.talk(798332, 10001) // out of order
	f.expectVars(0)
	f.expectWindow(f.talk(204839, 25), 1003)
	f.expectWindow(f.talk(204839, 1011), 1011)
	f.talk(204839, 10000)
	f.expectVars(1)
	// the fixture player is not spawned, so the teleport itself is a no-op; the instance is still registered
	if f.s.registeredInstance(suspiciousCallInstance, f.p.ID) == nil {
		t.Fatal("no instance registered for the player")
	}
	f.talk(204839, 10000) // repeat must not create another instance
	if f.s.instanceCount(suspiciousCallInstance) != 1 {
		t.Fatalf("instances = %d", f.s.instanceCount(suspiciousCallInstance))
	}
	f.expectVars(1)
	f.expectWindow(f.talk(798332, 25), 1352)
	f.talk(798332, 10001)
	f.expectVars(2)

	f.give(suspiciousCallItem, 1)
	item := f.p.cube[len(f.p.cube)-1]
	f.talk(279006, 10255) // out of order
	f.expectVars(2)
	f.c.suspiciousCallItemDone(item, f.script)
	f.expectVars(3)
	if f.s.countItems(f.p, suspiciousCallItem) != 0 {
		t.Fatal("scroll not consumed")
	}
	f.c.suspiciousCallItemDone(item, f.script) // repeated timer does nothing
	f.expectVars(3)
	f.expectWindow(f.talk(279006, 25), 2034)
	f.talk(279006, 10255)
	if q := f.p.quest(4200); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	f.finish(204286)
}

func TestASuspiciousCallItemUseNeedsVariableTwo(t *testing.T) {
	f := newChainFixture(t, 4200)
	f.start(4762)
	f.give(suspiciousCallItem, 1)
	if f.c.suspiciousCallItemUse(f.p.cube[len(f.p.cube)-1], f.script) {
		t.Fatal("scroll worked before variable 2")
	}
}
