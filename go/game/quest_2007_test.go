package game

import (
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestWheresRaeThisTimeUnlockAndFinale(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.level, p.Exp = "ASMODIANS", 5, d.ExpStart(5)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.spawned = true
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	p.quests = []store.Quest{{ID: 2007, Status: "LOCKED"}}
	for id := int32(2001); id <= 2006; id++ {
		p.quests = append(p.quests, store.Quest{ID: id, Status: "COMPLETE", CompleteCount: 1})
	}
	p.quest(2006).Status = "START"
	if c.wheresRaeThisTimeLevelUp() {
		t.Fatal("finale unlocked before every prerequisite")
	}
	p.quest(2006).Status = "COMPLETE"
	if !c.wheresRaeThisTimeLevelUp() || p.quest(2007).Status != "START" || c.wheresRaeThisTimeLevelUp() {
		t.Fatal("finale unlock")
	}
	script := d.QuestScripts[2007]
	if script == nil {
		t.Fatal("quest 2007 unregistered")
	}
	for index, npcID := range []int32{203516, 203519, 203539, 203552, 203554} {
		o := questCatalogNPC(s, p, npcID, 0x31000+int32(index))
		if !c.wheresRaeThisTimeDialog(o, script, 25) || !c.wheresRaeThisTimeDialog(o, script, int32(10000+index)) || p.quest(2007).Vars != int32(index+1) {
			t.Fatalf("conversation %d: %+v", index, p.quest(2007))
		}
	}
	for index, npcID := range []int32{700081, 700082, 700083} {
		o := questCatalogNPC(s, p, npcID, 0x32000+int32(index))
		p.targetID = o.id
		if !c.wheresRaeThisTimeDialog(o, script, -1) || c.wheresRaeThisTimeDialog(o, script, -1) {
			t.Fatalf("object %d use guard", index)
		}
		time.Sleep(3200 * time.Millisecond)
		if p.quest(2007).Vars != int32(index+6) {
			t.Fatalf("object %d progress: %+v", index, p.quest(2007))
		}
	}
	if packets.last(smPlayMovie) == nil {
		t.Fatal("final object omitted movie")
	}
	rae := questCatalogNPC(s, p, 203554, 0x33000)
	if !c.wheresRaeThisTimeDialog(rae, script, 25) || !c.wheresRaeThisTimeDialog(rae, script, 10005) || p.quest(2007).Status != "REWARD" {
		t.Fatal("final report")
	}
	end := questCatalogNPC(s, p, 203516, 0x33001)
	if !c.wheresRaeThisTimeDialog(end, script, -1) {
		t.Fatal("reward preview")
	}
	before := p.Exp
	if !c.wheresRaeThisTimeDialog(end, script, 8) || p.quest(2007).Status != "COMPLETE" || p.Exp-before != 11500 || s.countItems(p, 110100859) != 1 {
		t.Fatal("final reward")
	}
	c.wheresRaeThisTimeDialog(end, script, 8)
	if s.countItems(p, 110100859) != 1 {
		t.Fatal("duplicate final reward")
	}
}
