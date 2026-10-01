package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAshesToAshesItemStartAndNPCProgress(t *testing.T) {
	d := staticDataOrSkip(t)
	script := &data.QuestScript{ID: 2122, Kind: data.QuestCustom, StartNPC: 203551, EndNPC: 203551, ItemID: 182203120}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 5
	p.Exp = d.ExpStart(5)
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if !s.addItem(p, 182203120, 1) {
		t.Fatal("could not add quest item")
	}
	item := p.cube[0]
	c.ashesToAshesEvent(nil, item, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(0, 4, 2122).Data) {
		t.Fatalf("item use dialog = %x", got)
	}
	c.ashesToAshesEvent(nil, nil, script, 1002)
	if q := p.quest(2122); q == nil || q.Status != "START" {
		t.Fatalf("item did not start quest: %+v", q)
	}
	npc := questCatalogNPC(s, p, 203551, 0x30001)
	c.ashesToAshesEvent(npc, nil, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1011, 2122).Data) {
		t.Fatalf("first NPC dialog = %x", got)
	}
	c.ashesToAshesEvent(npc, nil, script, 1012)
	if s.countItems(p, 182203120) != 0 {
		t.Fatal("quest item was not removed")
	}
	c.ashesToAshesEvent(npc, nil, script, 10000)
	if q := p.quest(2122); q.Vars != 1 || q.Status != "START" {
		t.Fatalf("NPC progress = %+v", q)
	}
	c.ashesToAshesEvent(npc, nil, script, 10000)
	if p.quest(2122).Vars != 1 {
		t.Fatal("repeated NPC dialog advanced the quest")
	}
}

func TestAshesToAshesUrnAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script := &data.QuestScript{ID: 2122, Kind: data.QuestCustom, StartNPC: 203551, EndNPC: 203551, ItemID: 182203120}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 5
	p.Exp = d.ExpStart(5)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.spawned = true
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if !s.addItem(p, 182203120, 1) {
		t.Fatal("could not add quest item")
	}
	c.ashesToAshesEvent(nil, nil, script, 1002)
	urn := questCatalogNPC(s, p, 730029, 0x30002)
	start := questCatalogNPC(s, p, 203551, 0x30001)
	c.ashesToAshesEvent(urn, nil, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(urn.id, 1693, 2122).Data) {
		t.Fatalf("missing ashes dialog = %x", got)
	}
	c.ashesToAshesEvent(urn, nil, script, 10001)
	if p.quest(2122).Status != "START" {
		t.Fatal("urn accepted missing ashes")
	}
	if !s.addItem(p, 182203133, 3) {
		t.Fatal("could not add ashes")
	}
	p.targetID = urn.id
	c.ashesToAshesEvent(urn, nil, script, -1)
	if got := packets.last(smUseObject); !bytes.Equal(got, useObject(p.ID, urn.id, 1).Data) {
		t.Fatalf("urn use packet = %x", got)
	}
	time.Sleep(3200 * time.Millisecond)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(urn.id, 1352, 2122).Data) {
		t.Fatalf("urn finish dialog = %x", got)
	}
	c.ashesToAshesEvent(urn, nil, script, 10001)
	if p.quest(2122).Status != "REWARD" || s.countItems(p, 182203133) != 0 {
		t.Fatalf("urn turn-in failed: %+v", p.quest(2122))
	}
	c.ashesToAshesEvent(start, nil, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(start.id, 2375, 2122).Data) {
		t.Fatalf("reward dialog = %x", got)
	}
	before := p.Exp
	c.ashesToAshesEvent(start, nil, script, 17)
	if p.quest(2122).Status != "COMPLETE" || p.Exp-before != 2520 || s.countItems(p, 162000012) != 2 {
		t.Fatalf("wrong reward: quest=%+v exp=%d items=%d", p.quest(2122), p.Exp-before, s.countItems(p, 162000012))
	}
	c.ashesToAshesEvent(start, nil, script, 17)
	if s.countItems(p, 162000012) != 2 {
		t.Fatal("reward was duplicated")
	}
}
