package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestThinkingAheadLockedDialogAndCollect(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2001, Kind: data.QuestCustom, StartNPC: 203518, EndNPC: 203518}
	npc := questCatalogNPC(s, p, 203518, 0x30001)
	stone := questCatalogNPC(s, p, 700093, 0x30002)
	wrong := questCatalogNPC(s, p, 203533, 0x30003)
	if c.thinkingAheadLevelUp() {
		t.Fatal("uncreated quest unlocked")
	}
	locked := store.Quest{ID: 2001, Status: "LOCKED"}
	if err := s.quests.SaveQuest(p.ID, locked); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, locked)
	if c.thinkingAheadEvent(npc, script, 25, false) || !c.thinkingAheadLevelUp() || c.thinkingAheadLevelUp() {
		t.Fatal("locked/start transition failed")
	}
	if c.thinkingAheadEvent(wrong, script, 25, false) || c.thinkingAheadEvent(stone, script, -1, false) {
		t.Fatal("unexpected early dialogue")
	}
	if !c.thinkingAheadEvent(npc, script, 25, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1011, 2001).Data) {
		t.Fatal("initial page")
	}
	c.thinkingAheadEvent(npc, script, 1012, false)
	if !bytes.Equal(packets.last(smPlayMovie), thinkingAheadMovie().Data) {
		t.Fatal("movie 51 packet")
	}
	if !c.thinkingAheadEvent(npc, script, 10000, false) || p.quest(2001).Vars != 1 || !c.thinkingAheadEvent(stone, script, -1, false) {
		t.Fatal("first stage")
	}
	c.thinkingAheadEvent(npc, script, 25, false)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1352, 2001).Data) {
		t.Fatal("collection page")
	}
	c.thinkingAheadEvent(npc, script, 33, false)
	if p.quest(2001).Vars != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1693, 2001).Data) {
		t.Fatal("missing collection")
	}
	if !c.thinkingAheadEvent(npc, script, 10000, false) || p.quest(2001).Vars != 1 {
		t.Fatal("Java collection fallthrough")
	}
	if !s.addItem(p, 182203002, 4) {
		t.Fatal("could not add quest items")
	}
	if !c.thinkingAheadEvent(npc, script, 33, false) || p.quest(2001).Vars != 2 || s.countItems(p, 182203002) != 0 {
		t.Fatal("collection turn-in")
	}
	if c.thinkingAheadEvent(npc, script, 33, false) || !c.thinkingAheadEvent(npc, script, 10002, false) || p.quest(2001).Vars != 3 {
		t.Fatal("third stage")
	}
	if c.thinkingAheadEvent(npc, script, 10002, false) || p.quest(2001).Vars != 3 {
		t.Fatal("repeat advanced quest")
	}
}

func TestThinkingAheadKillsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 3
	p.Exp = d.ExpStart(3)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2001, Kind: data.QuestCustom, EndNPC: 203518}
	q := store.Quest{ID: 2001, Status: "START", Vars: 3}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	npc := questCatalogNPC(s, p, 203518, 0x30001)
	killA := questCatalogNPC(s, p, 210368, 0x30002)
	killB := questCatalogNPC(s, p, 210369, 0x30003)
	if c.thinkingAheadEvent(npc, script, 0, true) {
		t.Fatal("wrong kill counted")
	}
	for index := 0; index < 6; index++ {
		target := killA
		if index%2 == 1 {
			target = killB
		}
		if !c.thinkingAheadEvent(target, script, 0, true) {
			t.Fatalf("kill %d rejected", index+1)
		}
	}
	if p.quest(2001).Status != "REWARD" || p.quest(2001).Vars != 8 || c.thinkingAheadEvent(killA, script, 0, true) {
		t.Fatal("kill completion or repeat")
	}
	if !c.thinkingAheadEvent(npc, script, -1, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 2034, 2001).Data) {
		t.Fatal("reward preview")
	}
	before := p.Exp
	c.thinkingAheadEvent(npc, script, 8, false)
	if p.quest(2001).Status != "COMPLETE" || p.Exp-before != 2250 || s.countItems(p, 114100794) != 1 {
		t.Fatalf("reward = %+v, exp = %d", p.quest(2001), p.Exp-before)
	}
	c.thinkingAheadEvent(npc, script, 8, false)
	if s.countItems(p, 114100794) != 1 {
		t.Fatal("repeated reward")
	}
}
