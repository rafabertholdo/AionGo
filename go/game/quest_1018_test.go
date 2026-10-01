package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestMarkOfVengeanceProgressionAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 12
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x101801, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: markOfVengeanceQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := d.QuestScripts[markOfVengeanceQuestID]
	if script == nil {
		t.Fatal("Mark of Vengeance is absent from the custom quest catalog")
	}
	npc := questCatalogNPC(s, p, markOfVengeanceNPCID, 0x101802)

	if c.markOfVengeanceLevelUp() {
		t.Fatal("quest unlocked below level 13")
	}
	p.level = 13
	if !c.markOfVengeanceLevelUp() || c.markOfVengeanceLevelUp() || p.quest(markOfVengeanceQuestID).Status != "START" {
		t.Fatalf("level-up unlock failed: %+v", p.quest(markOfVengeanceQuestID))
	}
	if !c.markOfVengeanceDialog(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1011, markOfVengeanceQuestID).Data) {
		t.Fatalf("opening dialog = %x", packets.last(smDialogWindow))
	}
	if !c.markOfVengeanceDialog(npc, script, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1097, markOfVengeanceQuestID).Data) || p.quest(markOfVengeanceQuestID).Status != "START" {
		t.Fatal("turn-in without the collection did not show the missing-items page")
	}
	if !s.addItem(p, 182200020, 50) {
		t.Fatal("could not add the fifty Mark of Vengeance items")
	}
	if !c.markOfVengeanceDialog(npc, script, 33) || p.quest(markOfVengeanceQuestID).Status != "REWARD" || s.countItems(p, 182200020) != 0 {
		t.Fatalf("turn-in failed: quest=%+v items=%d", p.quest(markOfVengeanceQuestID), s.countItems(p, 182200020))
	}
	if !c.markOfVengeanceDialog(npc, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 5, markOfVengeanceQuestID).Data) {
		t.Fatalf("reward preview = %x", packets.last(smDialogWindow))
	}
	if !c.markOfVengeanceDialog(npc, script, 8) || p.quest(markOfVengeanceQuestID).Status != "COMPLETE" || p.quest(markOfVengeanceQuestID).CompleteCount != 1 {
		t.Fatalf("reward completion failed: %+v", p.quest(markOfVengeanceQuestID))
	}
	if c.markOfVengeanceDialog(npc, script, 8) || p.quest(markOfVengeanceQuestID).CompleteCount != 1 {
		t.Fatal("completed quest could be rewarded twice")
	}
}

func TestMarkOfVengeanceDropsAreIndexed(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[markOfVengeanceQuestID]
	if script == nil {
		t.Fatal("Mark of Vengeance is absent from the custom quest catalog")
	}
	for _, npcID := range []int32{210159, 210708, 210168, 210169, 210172, 210178, 210179, 210180, 210185, 210707, 210186, 210187, 210314, 210315, 210356, 210352, 210353, 210694, 210357} {
		found := false
		for _, candidate := range d.QuestDropsByNPC[npcID] {
			if candidate.ID == markOfVengeanceQuestID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("quest drop NPC %d is not indexed", npcID)
		}
	}
}
