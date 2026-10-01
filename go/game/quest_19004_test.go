package game

import "testing"

func TestPeriklessInsight(t *testing.T) {
	f := newChainFixture(t, 19004)
	f.start(1011)
	f.talk(203701, 10001) // out of order
	f.expectVars(0)
	f.talk(203752, 10000)
	f.talk(203752, 10000)
	f.expectVars(1)
	f.talk(203701, 10001)
	f.talk(798500, 10255)
	q := f.p.quest(19004)
	if q.Status != "REWARD" {
		t.Fatalf("quest = %+v", q)
	}
	// every chain NPC shows the reward page; any of them can finish it
	for _, npc := range []int32{203757, 203752} {
		o := f.npcs[npc]
		if o == nil {
			o = f.talk(npc, 0)
		}
		if !f.c.customQuestShowDialog(o, f.script) {
			t.Fatalf("npc %d did not show reward page", npc)
		}
		f.expectWindow(o, 10002)
	}
	f.finish(203752)
}
