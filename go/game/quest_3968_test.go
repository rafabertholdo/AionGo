package game

import "testing"

func TestPalentinesRequest(t *testing.T) {
	f := newChainFixture(t, 3968)
	f.p.Gender = "FEMALE"
	f.start(1011)
	f.talk(204528, 10001) // out of order
	f.expectVars(0)
	for i, step := range []struct {
		npc  int32
		item int32
	}{{798176, 182206123}, {204528, 182206124}, {203927, 182206125}} {
		f.talk(step.npc, uint16(10000+i))
		if f.s.countItems(f.p, step.item) != 1 {
			t.Fatalf("step %d gave no item", i)
		}
	}
	if q := f.p.quest(3968); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	f.expectWindow(f.talk(798390, 1009), 5)
	for _, item := range []int32{182206123, 182206124, 182206125} {
		if f.s.countItems(f.p, item) != 0 {
			t.Fatalf("item %d not taken", item)
		}
	}
	f.finish(798390)
}
