package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSpiritOfNatureStartWorkItemsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[spiritOfNatureQuestID], d.Quests[spiritOfNatureQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.NPCStart || script.StartNPC != spiritOfNatureStartNPCID || script.EndNPC != spiritOfNatureStartNPCID || template.Race != "ELYOS" || template.MinLevel != 18 || template.NameID != 2204707 ||
		len(template.QuestWorkItems) != 2 || template.QuestWorkItems[0] != (data.QuestItem{ID: spiritOfNatureFirstItemID, Count: 1}) || template.QuestWorkItems[1] != (data.QuestItem{ID: spiritOfNatureSecondItemID, Count: 1}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 17400 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 162000024, Count: 3}) {
		t.Fatalf("unexpected Spirit of Nature metadata: script=%+v template=%+v", script, template)
	}
	if len(d.QuestStarts[spiritOfNatureStartNPCID]) == 0 || len(d.QuestEnds[spiritOfNatureStartNPCID]) == 0 {
		t.Fatalf("quest start/end index is missing NPC %d", spiritOfNatureStartNPCID)
	}
	for _, npcID := range []int32{spiritOfNatureStartNPCID, spiritOfNatureFirstNPCID, spiritOfNatureSecondNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestStarts[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 17
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	start := npc(spiritOfNatureStartNPCID, 0x31181)
	first := npc(spiritOfNatureFirstNPCID, 0x31182)
	second := npc(spiritOfNatureSecondNPCID, 0x31183)
	selectDialog := func(target *object, dialogID int32) bool {
		return c.spiritOfNatureDialog(target, script, dialogID)
	}
	if selectDialog(start, 25) || p.quest(spiritOfNatureQuestID) != nil {
		t.Fatal("quest should not start below level 18")
	}
	p.level = 18
	if !selectDialog(start, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, spiritOfNatureQuestID).Data) {
		t.Fatalf("quest offer page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(start, 1002) || p.quest(spiritOfNatureQuestID) == nil || p.quest(spiritOfNatureQuestID).Status != "START" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1003, spiritOfNatureQuestID).Data) {
		t.Fatalf("acceptance did not start the quest: quest=%+v dialog=%x", p.quest(spiritOfNatureQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(first, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(first.id, 1352, spiritOfNatureQuestID).Data) {
		t.Fatalf("first helper plain click page = %x", packets.last(smDialogWindow))
	}
	// The Java case for NPC 730013 falls through to NPC 730014 when dialog 25
	// arrives, so the later page is sent by this NPC object's id.
	if !selectDialog(first, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(first.id, 1693, spiritOfNatureQuestID).Data) {
		t.Fatalf("first helper's Java fallthrough page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(first, 10000) || questVar(p.quest(spiritOfNatureQuestID).Vars, 0) != 1 || s.countItems(p, spiritOfNatureFirstItemID) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(first.id, 10, 0).Data) {
		t.Fatalf("first work item was not granted: quest=%+v count=%d dialog=%x", p.quest(spiritOfNatureQuestID), s.countItems(p, spiritOfNatureFirstItemID), packets.last(smDialogWindow))
	}
	if !selectDialog(second, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(second.id, 1693, spiritOfNatureQuestID).Data) {
		t.Fatalf("second helper page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(second, 10001) || questVar(p.quest(spiritOfNatureQuestID).Vars, 0) != 2 || s.countItems(p, spiritOfNatureSecondItemID) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(second.id, 10, 0).Data) {
		t.Fatalf("second work item was not granted: quest=%+v count=%d dialog=%x", p.quest(spiritOfNatureQuestID), s.countItems(p, spiritOfNatureSecondItemID), packets.last(smDialogWindow))
	}
	if !selectDialog(second, 10001) || questVar(p.quest(spiritOfNatureQuestID).Vars, 0) != 3 || s.countItems(p, spiritOfNatureSecondItemID) != 1 {
		t.Fatalf("repeated item request should advance without duplicating the item: quest=%+v count=%d", p.quest(spiritOfNatureQuestID), s.countItems(p, spiritOfNatureSecondItemID))
	}
	if !selectDialog(start, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 2375, spiritOfNatureQuestID).Data) {
		t.Fatalf("turn-in page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(start, 1009) || p.quest(spiritOfNatureQuestID).Status != "REWARD" || questVar(p.quest(spiritOfNatureQuestID).Vars, 0) != 3 || s.countItems(p, spiritOfNatureFirstItemID) != 0 || s.countItems(p, spiritOfNatureSecondItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 5, spiritOfNatureQuestID).Data) {
		t.Fatalf("turn-in did not remove both work items and open rewards: quest=%+v first=%d second=%d page=%x", p.quest(spiritOfNatureQuestID), s.countItems(p, spiritOfNatureFirstItemID), s.countItems(p, spiritOfNatureSecondItemID), packets.last(smDialogWindow))
	}
	if !selectDialog(start, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 5, spiritOfNatureQuestID).Data) {
		t.Fatalf("reward-state click did not show rewards: %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	if !selectDialog(start, 17) || p.quest(spiritOfNatureQuestID).Status != "COMPLETE" || p.quest(spiritOfNatureQuestID).CompleteCount != 1 || p.Exp-beforeExperience != template.Rewards[0].Experience {
		t.Fatalf("Spirit of Nature did not finish: quest=%+v XP gained=%d", p.quest(spiritOfNatureQuestID), p.Exp-beforeExperience)
	}
	if count := s.countItems(p, 162000024); count != 3 {
		t.Fatalf("reward item count = %d, want 3", count)
	}
}
