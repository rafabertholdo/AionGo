package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestTeachingALessonLevelUp(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.level = 3
	c := &conn{s: s, player: p}
	q := store.Quest{ID: 2005, Status: "LOCKED"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.teachingALessonLevelUp() {
		t.Fatal("unlocked below level 4")
	}
	p.level = 4
	if !c.teachingALessonLevelUp() || p.quest(2005).Status != "START" || c.teachingALessonLevelUp() {
		t.Fatal("unlock or duplicate")
	}
}

func TestTeachingALessonDialogItemsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2005, Kind: data.QuestCustom, EndNPC: 203540}
	npc := questCatalogNPC(s, p, 203540, 0x32501)
	wrong := questCatalogNPC(s, p, 203539, 0x32502)
	q := store.Quest{ID: 2005, Status: "START"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.teachingALessonEvent(wrong, script, 25) || c.teachingALessonEvent(npc, nil, 25) || c.teachingALessonEvent(npc, script, 33) {
		t.Fatal("invalid event")
	}
	if !c.teachingALessonEvent(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1011, 2005).Data) {
		t.Fatal("opening page")
	}
	if c.teachingALessonEvent(npc, script, 1012) || !bytes.Equal(packets.last(smPlayMovie), wheresRaeMovie(54).Data) {
		t.Fatal("movie")
	}
	if !c.teachingALessonEvent(npc, script, 10000) || questVar(p.quest(2005).Vars, 0) != 1 || c.teachingALessonEvent(npc, script, 10000) {
		t.Fatal("conversation")
	}
	if !c.teachingALessonEvent(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1352, 2005).Data) {
		t.Fatal("second page")
	}
	if !c.teachingALessonEvent(npc, script, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1693, 2005).Data) {
		t.Fatal("missing items page")
	}
	if !s.addItem(p, 182203006, 5) {
		t.Fatal("add collect items")
	}
	if !c.teachingALessonEvent(npc, script, 33) || p.quest(2005).Status != "REWARD" || s.countItems(p, 182203006) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 5, 2005).Data) {
		t.Fatal("item turn-in")
	}
	if c.teachingALessonEvent(npc, script, 33) || !c.teachingALessonEvent(npc, script, -1) || !c.teachingALessonEvent(npc, script, 1009) {
		t.Fatal("reward dialog")
	}
	before := p.Exp
	if !c.teachingALessonEvent(npc, script, 8) || p.quest(2005).Status != "COMPLETE" || p.Exp-before != 4150 || s.countItems(p, 113100773) != 1 {
		t.Fatal("selected reward")
	}
	if c.teachingALessonEvent(npc, script, 8) {
		t.Fatal("duplicate completion")
	}
}
