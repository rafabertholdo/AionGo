package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestLepharistPoisonResearchProgressKillAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[lepharistPoisonResearchQuestID], d.Quests[lepharistPoisonResearchQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != lepharistPoisonStartNPCID || script.EndNPC != lepharistPoisonEndNPCID || template.Race != "ELYOS" || template.MinLevel != 35 || template.NameID != 2204211 ||
		len(template.FinishedQuestConditions) != 2 || template.FinishedQuestConditions[0] != 1016 || template.FinishedQuestConditions[1] != 1039 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: lepharistPoisonItemID, Count: 1}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 2108600 || len(template.Rewards[0].SelectableItems) != 2 ||
		template.Rewards[0].SelectableItems[0].ID != 121000775 || template.Rewards[0].SelectableItems[1].ID != 121000776 {
		t.Fatalf("unexpected Lepharist Poison Research metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{lepharistPoisonStartNPCID, lepharistPoisonSecondNPCID, lepharistPoisonReportNPCID, lepharistPoisonEndNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	foundKill := false
	for _, registered := range d.QuestKills[lepharistPoisonTargetNPCID] {
		foundKill = foundKill || registered.ID == lepharistPoisonResearchQuestID
	}
	if !foundKill {
		t.Fatal("quest kill index is missing NPC 212151")
	}
	for _, npcID := range []int32{211911, 211912} {
		foundDrop := false
		for _, registered := range d.QuestDropsByNPC[npcID] {
			foundDrop = foundDrop || registered.ID == lepharistPoisonResearchQuestID
		}
		if !foundDrop {
			t.Fatalf("quest drop index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 34
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{
		{ID: lepharistPoisonResearchQuestID, Status: "LOCKED"},
		{ID: 1500, Status: "START"},
		{ID: 1016, Status: "COMPLETE"},
		{ID: 1039, Status: "COMPLETE"},
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.lepharistPoisonResearchLevelUp() {
		t.Fatal("quest unlocked below level 35")
	}
	p.level = 35
	if c.lepharistPoisonResearchLevelUp() {
		t.Fatal("quest unlocked before quest 1500 completed")
	}
	p.quest(1500).Status = "COMPLETE"
	p.quest(1016).Status = "START"
	if c.lepharistPoisonResearchLevelUp() {
		t.Fatal("quest unlocked before quest 1016 completed")
	}
	p.quest(1016).Status = "COMPLETE"
	p.quest(1039).Status = "START"
	if c.lepharistPoisonResearchLevelUp() {
		t.Fatal("quest unlocked before quest 1039 completed")
	}
	p.quest(1039).Status = "COMPLETE"
	if !c.lepharistPoisonResearchLevelUp() || p.quest(lepharistPoisonResearchQuestID).Status != "START" || c.lepharistPoisonResearchLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(lepharistPoisonResearchQuestID))
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
	startNPC := npc(lepharistPoisonStartNPCID, 0x31131)
	secondNPC := npc(lepharistPoisonSecondNPCID, 0x31132)
	reportNPC := npc(lepharistPoisonReportNPCID, 0x31133)
	endNPC := npc(lepharistPoisonEndNPCID, 0x31134)
	selectDialog := func(o *object, dialogID int32) {
		if !c.lepharistPoisonResearchDialog(o, script, dialogID) {
			t.Fatalf("dialog %d at NPC %d was not handled", dialogID, o.npc.ID)
		}
	}
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, lepharistPoisonResearchQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10000)
	selectDialog(secondNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(secondNPC.id, 1352, lepharistPoisonResearchQuestID).Data) {
		t.Fatalf("second NPC page = %x", packets.last(smDialogWindow))
	}
	selectDialog(secondNPC, 10001)
	if questVar(p.quest(lepharistPoisonResearchQuestID).Vars, 0) != 2 {
		t.Fatalf("second NPC did not advance to report: %+v", p.quest(lepharistPoisonResearchQuestID))
	}
	if !s.addItem(p, lepharistPoisonItemID, 2) {
		t.Fatal("could not add poison sample items")
	}
	selectDialog(reportNPC, 33)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 10001, lepharistPoisonResearchQuestID).Data) || packets.last(smPlayMovie) != nil {
		t.Fatalf("two items should not satisfy Java's exact-one check: page=%x movie=%x", packets.last(smDialogWindow), packets.last(smPlayMovie))
	}
	s.removeItemsByID(p, lepharistPoisonItemID, 1)
	selectDialog(reportNPC, 33)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 10000, lepharistPoisonResearchQuestID).Data) || !bytes.Equal(packets.last(smPlayMovie), playMovie(101).Data) {
		t.Fatalf("single item did not play the movie and show page 10000: page=%x movie=%x", packets.last(smDialogWindow), packets.last(smPlayMovie))
	}
	selectDialog(reportNPC, 10003)
	if questVar(p.quest(lepharistPoisonResearchQuestID).Vars, 0) != 4 || s.countItems(p, lepharistPoisonItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 10, 0).Data) {
		t.Fatalf("report did not consume sample and advance to hunt: quest=%+v items=%d dialog=%x", p.quest(lepharistPoisonResearchQuestID), s.countItems(p, lepharistPoisonItemID), packets.last(smDialogWindow))
	}
	if c.lepharistPoisonResearchKill(212152) || questVar(p.quest(lepharistPoisonResearchQuestID).Vars, 0) != 4 {
		t.Fatal("unrelated kill advanced the quest")
	}
	if !c.lepharistPoisonResearchKill(lepharistPoisonTargetNPCID) || questVar(p.quest(lepharistPoisonResearchQuestID).Vars, 0) != 5 || c.lepharistPoisonResearchKill(lepharistPoisonTargetNPCID) {
		t.Fatalf("target kill did not advance exactly once: %+v", p.quest(lepharistPoisonResearchQuestID))
	}
	selectDialog(reportNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 2716, lepharistPoisonResearchQuestID).Data) {
		t.Fatalf("post-hunt report page = %x", packets.last(smDialogWindow))
	}
	selectDialog(reportNPC, 10003) // Java falls through to 10255 at variable five.
	quest := p.quest(lepharistPoisonResearchQuestID)
	if quest.Status != "REWARD" || questVar(quest.Vars, 0) != 5 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 10, 0).Data) {
		t.Fatalf("post-hunt report did not set reward: quest=%+v dialog=%x", quest, packets.last(smDialogWindow))
	}
	if c.lepharistPoisonResearchDialog(reportNPC, script, -1) {
		t.Fatal("non-end NPC handled reward dialog")
	}
	if !c.lepharistPoisonResearchDialog(endNPC, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, lepharistPoisonResearchQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	if !c.lepharistPoisonResearchDialog(endNPC, script, 8) {
		t.Fatal("reward selection was not handled")
	}
	if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || p.Exp != d.ExpStart(p.level)+2108600 {
		t.Fatalf("reward choice did not complete quest: quest=%+v item=%d exp=%d", quest, s.countItems(p, template.Rewards[0].SelectableItems[0].ID), p.Exp)
	}
}
