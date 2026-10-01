package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestBalaurConspiracyPrerequisitesFallthroughKillAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[balaurConspiracyQuestID], d.Quests[balaurConspiracyQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != balaurConspiracyStartNPC || template.Race != "ELYOS" || template.MinLevel != 35 || template.NameID != 2204125 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 2467700 || template.Rewards[0].TitleID != 12 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Balaur Conspiracy metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{balaurConspiracyStartNPC, balaurConspiracySecondNPC, balaurConspiracyFinalNPC} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	foundKill := false
	for _, registered := range d.QuestKills[balaurConspiracyTargetID] {
		foundKill = foundKill || registered.ID == balaurConspiracyQuestID
	}
	if !foundKill {
		t.Fatal("Balaur Conspiracy kill index is missing NPC 211629")
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 35
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.WorldID, p.X, p.Y, p.Z = balaurConspiracyWorldID, 2500, 780, 409
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: balaurConspiracyQuestID, Status: "LOCKED"}}
	for _, prerequisiteID := range balaurConspiracyPrerequisites {
		p.quests = append(p.quests, store.Quest{ID: prerequisiteID, Status: "COMPLETE"})
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	s.addPlayerCell(p)
	p.quest(1042).Status = "START"
	if c.balaurConspiracyLevelUp() {
		t.Fatal("quest unlocked before every Java prerequisite was complete")
	}
	p.quest(1042).Status = "COMPLETE"
	if !c.balaurConspiracyLevelUp() || p.quest(balaurConspiracyQuestID).Status != "START" || c.balaurConspiracyLevelUp() {
		t.Fatalf("quest did not unlock exactly once after all prerequisites: %+v", p.quest(balaurConspiracyQuestID))
	}

	npc := func(id, objectID int32) *object {
		object := questCatalogNPC(s, p, id, objectID)
		object.npc = d.Npcs[id]
		if object.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(object)
		return object
	}
	selectDialog := func(object *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, balaurConspiracyQuestID))
	}
	startNPC := npc(balaurConspiracyStartNPC, 0x31109)
	secondNPC := npc(balaurConspiracySecondNPC, 0x3110a)
	finalNPC := npc(balaurConspiracyFinalNPC, 0x3110b)
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, balaurConspiracyQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10000)
	if questVar(p.quest(balaurConspiracyQuestID).Vars, 0) != 1 {
		t.Fatalf("opening dialogue did not advance: %+v", p.quest(balaurConspiracyQuestID))
	}
	selectDialog(startNPC, 25) // Java falls through the start NPC case to the second NPC case.
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1352, balaurConspiracyQuestID).Data) {
		t.Fatalf("stage-one fallthrough page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10001)
	if questVar(p.quest(balaurConspiracyQuestID).Vars, 0) != 2 {
		t.Fatalf("dialog fallthrough did not advance to two: %+v", p.quest(balaurConspiracyQuestID))
	}
	selectDialog(secondNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(secondNPC.id, 1693, balaurConspiracyQuestID).Data) {
		t.Fatalf("Balaur lead page = %x", packets.last(smDialogWindow))
	}
	selectDialog(secondNPC, 10002)
	if questVar(p.quest(balaurConspiracyQuestID).Vars, 0) != 3 {
		t.Fatalf("Balaur lead did not advance to the hunt: %+v", p.quest(balaurConspiracyQuestID))
	}
	if !c.balaurConspiracyKill(balaurConspiracyTargetID) || questVar(p.quest(balaurConspiracyQuestID).Vars, 0) != 4 || c.balaurConspiracyKill(balaurConspiracyTargetID) {
		t.Fatalf("target kill did not advance once: %+v", p.quest(balaurConspiracyQuestID))
	}
	selectDialog(finalNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(finalNPC.id, 2034, balaurConspiracyQuestID).Data) {
		t.Fatalf("final briefing page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10003) // Java falls through both earlier NPC cases to the final status/teleport branch.
	quest := p.quest(balaurConspiracyQuestID)
	if quest.Status != "REWARD" || questVar(quest.Vars, 0) != 4 || p.WorldID != balaurConspiracyWorldID || p.X != float32(2502.1948) || p.Y != float32(782.9152) || p.Z != float32(408.97723) {
		t.Fatalf("final report did not award and teleport: quest=%+v location=%d %.4f,%.4f,%.4f", quest, p.WorldID, p.X, p.Y, p.Z)
	}
	startNPC.worldID, startNPC.x, startNPC.y, startNPC.z = p.WorldID, p.X+2, p.Y, p.Z
	p.seen = map[int32]*object{startNPC.id: startNPC}
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 2375, balaurConspiracyQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 1009)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 5, balaurConspiracyQuestID).Data) {
		t.Fatalf("reward choice page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 8)
	if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || !bytes.Equal(packets.last(smTitleList), titleList(p).Data) {
		t.Fatalf("selectable reward did not complete the quest: quest=%+v item=%d titles=%x", quest, s.countItems(p, template.Rewards[0].SelectableItems[0].ID), packets.last(smTitleList))
	}
}
