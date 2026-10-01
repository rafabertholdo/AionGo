package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestTreasureOfTheDeceasedLevelUp(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 2
	c := &conn{s: s, player: p}
	q := store.Quest{ID: 2003, Status: "LOCKED"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.treasureOfTheDeceasedLevelUp() {
		t.Fatal("unlocked without prerequisite")
	}
	previous := store.Quest{ID: 2100, Status: "COMPLETE"}
	p.quests = append(p.quests, previous)
	p.level = 1
	if c.treasureOfTheDeceasedLevelUp() {
		t.Fatal("unlocked below minimum level")
	}
	p.level = 2
	if !c.treasureOfTheDeceasedLevelUp() || p.quest(2003).Status != "START" || c.treasureOfTheDeceasedLevelUp() {
		t.Fatal("level unlock or duplicate")
	}
}

func TestTreasureOfTheDeceasedDialogCollectionAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 3
	p.Exp = d.ExpStart(3)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2003, Kind: data.QuestCustom, EndNPC: 203539}
	npc := questCatalogNPC(s, p, 203539, 0x30003)
	wrong := questCatalogNPC(s, p, 203540, 0x30004)
	q := store.Quest{ID: 2003, Status: "START"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.treasureOfTheDeceasedEvent(wrong, script, 25) || c.treasureOfTheDeceasedEvent(npc, nil, 25) {
		t.Fatal("invalid target or script")
	}
	if !c.treasureOfTheDeceasedEvent(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1011, 2003).Data) {
		t.Fatal("opening page")
	}
	if !c.treasureOfTheDeceasedEvent(npc, script, 10000) || p.quest(2003).Vars != 1 {
		t.Fatal("first conversation")
	}
	if !c.treasureOfTheDeceasedEvent(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1352, 2003).Data) {
		t.Fatal("collection page")
	}
	if !c.treasureOfTheDeceasedEvent(npc, script, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1693, 2003).Data) {
		t.Fatal("missing collection items")
	}
	if !s.addItem(p, 182203004, 4) {
		t.Fatal("give collection items")
	}
	if !c.treasureOfTheDeceasedEvent(npc, script, 33) || p.quest(2003).Status != "REWARD" || s.countItems(p, 182203004) != 0 {
		t.Fatal("collection turn-in")
	}
	if c.treasureOfTheDeceasedEvent(npc, script, 33) || !c.treasureOfTheDeceasedEvent(npc, script, -1) || !c.treasureOfTheDeceasedEvent(npc, script, 1009) {
		t.Fatal("repeat collection or reward page")
	}
	before := p.Exp
	if !c.treasureOfTheDeceasedEvent(npc, script, 17) || p.quest(2003).Status != "COMPLETE" || p.Exp-before != 3250 || s.countItems(p, 182000557) != 1 || s.countItems(p, 162000002) != 3 || p.kinah.Count != 500 {
		t.Fatal("completion rewards")
	}
	if c.treasureOfTheDeceasedEvent(npc, script, 17) {
		t.Fatal("repeat reward")
	}
}

func TestTreasureOfTheDeceasedMovieAndDialogFallthrough(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.seen = map[int32]*object{}
	p.cube = nil
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	script := &data.QuestScript{ID: 2003, Kind: data.QuestCustom}
	npc := questCatalogNPC(s, p, 203539, 0x30005)
	q := store.Quest{ID: 2003, Status: "START", Vars: 1}
	p.quests = append(p.quests, q)
	if c.treasureOfTheDeceasedEvent(npc, script, 1012) || !bytes.Equal(packets.last(smPlayMovie), wheresRaeMovie(53).Data) {
		t.Fatal("movie")
	}
	if !c.treasureOfTheDeceasedEvent(npc, script, 10000) {
		t.Fatal("Java collection fallthrough")
	}
	if p.quest(2003).Status != "START" {
		t.Fatal("unexpected progress")
	}
}
