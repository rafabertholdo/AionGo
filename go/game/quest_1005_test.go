package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestBarringTheGateLevelUp(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.quests = []store.Quest{{ID: 1005, Status: "LOCKED"}}
	c := &conn{s: s, player: p, tap: (&questPackets{}).tap}
	p.conn = c
	if c.barringTheGateLevelUp() {
		t.Fatal("unlocked without prerequisites")
	}
	for _, id := range []int32{1001, 1002, 1003, 1004} {
		p.quests = append(p.quests, store.Quest{ID: id, Status: "COMPLETE"})
	}
	if !c.barringTheGateLevelUp() || p.quest(1005).Status != "START" || c.barringTheGateLevelUp() {
		t.Fatalf("unexpected unlock state: %+v", p.quest(1005))
	}
}

func TestBarringTheGateDialogAndGateSequence(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: 1005, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1005, Kind: data.QuestCustom, EndNPC: 203067}
	for index, npcID := range []int32{203067, 203081, 790001, 203085, 203086} {
		npc := questCatalogNPC(s, p, npcID, int32(0x51001+index))
		c.barringTheGateDialog(npc, script, 25)
		page := uint16(1011 + 341*index)
		if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, page, 1005).Data) {
			t.Fatalf("NPC %d page = %x", npcID, got)
		}
		c.barringTheGateDialog(npc, script, int32(10000+index))
		c.barringTheGateDialog(npc, script, int32(10000+index))
		if got := questVar(p.quest(1005).Vars, 0); got != int32(index+1) {
			t.Fatalf("NPC %d variable = %d", npcID, got)
		}
	}
	for index, npcID := range []int32{700081, 700082, 700083, 700080} {
		npc := questCatalogNPC(s, p, npcID, int32(0x52001+index))
		p.targetID = npc.id
		c.barringTheGateDialog(npc, script, -1)
		if got := packets.last(smUseObject); !bytes.Equal(got, useObject(p.ID, npc.id, 1).Data) {
			t.Fatalf("gate %d start packet = %x", npcID, got)
		}
		c.barringTheGateDialog(npc, script, -1)
		time.Sleep(3200 * time.Millisecond)
		if got := packets.last(smUseObject); !bytes.Equal(got, useObject(p.ID, npc.id, 0).Data) {
			t.Fatalf("gate %d finish packet = %x", npcID, got)
		}
		if index < 3 && questVar(p.quest(1005).Vars, 0) != int32(index+6) {
			t.Fatalf("gate %d variable = %d", npcID, p.quest(1005).Vars)
		}
	}
	if p.quest(1005).Status != "REWARD" || !bytes.Equal(packets.last(smPlayMovie), barringTheGateMovie().Data) {
		t.Fatalf("final gate result: %+v movie=%x", p.quest(1005), packets.last(smPlayMovie))
	}
	end := questCatalogNPC(s, p, 203067, 0x53001)
	c.barringTheGateDialog(end, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(end.id, 2716, 1005).Data) {
		t.Fatalf("reward preview = %x", got)
	}
	c.barringTheGateDialog(end, script, 8)
	if p.quest(1005).Status != "COMPLETE" || s.countItems(p, 110100869) != 1 {
		t.Fatalf("reward result: %+v item=%d", p.quest(1005), s.countItems(p, 110100869))
	}
	c.barringTheGateDialog(end, script, 8)
	if s.countItems(p, 110100869) != 1 {
		t.Fatal("reward duplicated")
	}
}
