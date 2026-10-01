package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAetherInsanityLevelUpDialogsCollectionAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[aetherInsanityQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 38 || template.NameID != 2204215 ||
		len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1036 ||
		len(template.CollectItems) != 2 || template.CollectItems[0] != (data.QuestItem{ID: aetherInsanityFirstItemID, Count: 10}) ||
		template.CollectItems[1] != (data.QuestItem{ID: aetherInsanitySecondItemID, Count: 1}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 3218600 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Aether Insanity metadata: %+v", template)
	}
	for _, tc := range []struct {
		npcID int32
		item  int32
	}{{211179, aetherInsanityFirstItemID}, {211204, aetherInsanityFirstItemID}, {212235, aetherInsanityFirstItemID},
		{211180, aetherInsanityFirstItemID}, {211205, aetherInsanityFirstItemID}, {212236, aetherInsanityFirstItemID},
		{212237, aetherInsanityFirstItemID}, {212238, aetherInsanityFirstItemID}, {212239, aetherInsanityFirstItemID},
		{212240, aetherInsanityFirstItemID}, {212241, aetherInsanityFirstItemID}, {212242, aetherInsanityFirstItemID},
		{212614, aetherInsanitySecondItemID}} {
		found := false
		for _, drop := range template.QuestDrops {
			found = found || drop.NPCID == tc.npcID && drop.ItemID == tc.item && drop.Chance == 100
		}
		if !found {
			t.Fatalf("quest drop index is missing NPC %d, item %d", tc.npcID, tc.item)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 37
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: aetherInsanityQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.aetherInsanityLevelUp() {
		t.Fatal("quest unlocked below level 38")
	}
	p.level = 38
	p.Exp = d.ExpStart(p.level)
	if c.aetherInsanityLevelUp() {
		t.Fatal("quest unlocked before quest 1500 was completed")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.aetherInsanityLevelUp() || p.quest(aetherInsanityQuestID).Status != "START" || c.aetherInsanityLevelUp() {
		t.Fatalf("quest did not unlock exactly once after level and prerequisite: %+v", p.quest(aetherInsanityQuestID))
	}

	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	start := npc(aetherInsanityStartNPCID, 0x31170)
	end := npc(aetherInsanityEndNPCID, 0x31171)
	script := &data.QuestScript{ID: aetherInsanityQuestID, Kind: data.QuestCustom}
	selectDialog := func(o *object, dialogID int32) bool {
		return c.aetherInsanityDialog(o, script, dialogID)
	}
	if !selectDialog(start, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, aetherInsanityQuestID).Data) {
		t.Fatalf("start page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(start, 10000) || questVar(p.quest(aetherInsanityQuestID).Vars, 0) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 10, 0).Data) {
		t.Fatalf("start conversation did not advance: quest=%+v dialog=%x", p.quest(aetherInsanityQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 1352, aetherInsanityQuestID).Data) {
		t.Fatalf("end NPC first page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 10001) || questVar(p.quest(aetherInsanityQuestID).Vars, 0) != 2 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10, 0).Data) {
		t.Fatalf("end NPC conversation did not advance: quest=%+v dialog=%x", p.quest(aetherInsanityQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 1693, aetherInsanityQuestID).Data) {
		t.Fatalf("end NPC follow-up page = %x", packets.last(smDialogWindow))
	}
	if selectDialog(end, 1353) || packets.last(smPlayMovie) == nil || questVar(p.quest(aetherInsanityQuestID).Vars, 0) != 2 {
		t.Fatalf("movie branch should play movie 191 without handling the dialog or changing progress: movie=%x quest=%+v", packets.last(smPlayMovie), p.quest(aetherInsanityQuestID))
	}
	if !selectDialog(end, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10001, aetherInsanityQuestID).Data) || p.quest(aetherInsanityQuestID).Status != "START" {
		t.Fatalf("incomplete collection was accepted: quest=%+v dialog=%x", p.quest(aetherInsanityQuestID), packets.last(smDialogWindow))
	}
	if !s.addItem(p, aetherInsanityFirstItemID, 10) || !s.addItem(p, aetherInsanitySecondItemID, 1) {
		t.Fatal("could not add collection items")
	}
	if !selectDialog(end, 33) || p.quest(aetherInsanityQuestID).Status != "REWARD" ||
		s.countItems(p, aetherInsanityFirstItemID) != 0 || s.countItems(p, aetherInsanitySecondItemID) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, aetherInsanityQuestID).Data) {
		t.Fatalf("case-25 collection fallthrough failed: quest=%+v items=(%d,%d) dialog=%x", p.quest(aetherInsanityQuestID), s.countItems(p, aetherInsanityFirstItemID), s.countItems(p, aetherInsanitySecondItemID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, aetherInsanityQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, aetherInsanityQuestID).Data) {
		t.Fatalf("reward-ready page = %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	choice := template.Rewards[0].SelectableItems[0].ID
	if !selectDialog(end, 8) || p.quest(aetherInsanityQuestID).Status != "COMPLETE" || p.quest(aetherInsanityQuestID).CompleteCount != 1 ||
		s.countItems(p, choice) != 1 || p.Exp-beforeExperience != template.Rewards[0].Experience {
		t.Fatalf("quest reward did not complete: quest=%+v reward=%d experience=%d", p.quest(aetherInsanityQuestID), s.countItems(p, choice), p.Exp-beforeExperience)
	}
}
