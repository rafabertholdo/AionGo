package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestThePowerOfElimProgressAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[powerOfElimQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 31 || template.NameID != 2204207 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: powerOfElimCollectItemID, Count: 50}) ||
		len(template.QuestDrops) != 4 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 987100 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Power of Elim metadata: %+v", template)
	}
	wantDropNPCs := map[int32]bool{211101: false, 211131: false, 211111: false, 211126: false}
	for _, drop := range template.QuestDrops {
		if drop.ItemID == powerOfElimCollectItemID && drop.Chance == 100 {
			if _, ok := wantDropNPCs[drop.NPCID]; ok {
				wantDropNPCs[drop.NPCID] = true
			}
		}
	}
	for npcID, found := range wantDropNPCs {
		if !found {
			t.Errorf("quest drop metadata is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 30
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.quests = []store.Quest{
		{ID: powerOfElimQuestID, Status: "LOCKED"},
		{ID: 1500, Status: "START"},
		{ID: 1002, Status: "COMPLETE"},
		{ID: 1032, Status: "COMPLETE"},
		{ID: 1052, Status: "COMPLETE"},
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.powerOfElimLevelUp() {
		t.Fatal("quest unlocked below level 31")
	}
	p.level = 31
	p.Exp = d.ExpStart(p.level)
	p.quest(1500).Status = "COMPLETE"
	for _, prerequisiteID := range []int32{1002, 1032, 1052} {
		p.quest(prerequisiteID).Status = "START"
		if c.powerOfElimLevelUp() {
			t.Fatalf("quest unlocked before prerequisite %d was complete", prerequisiteID)
		}
		p.quest(prerequisiteID).Status = "COMPLETE"
	}
	if !c.powerOfElimLevelUp() || p.quest(powerOfElimQuestID).Status != "START" || c.powerOfElimLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(powerOfElimQuestID))
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
	selectDialog := func(o *object, dialog int32) bool {
		return c.powerOfElimDialog(o, &data.QuestScript{ID: powerOfElimQuestID}, dialog)
	}
	start := npc(powerOfElimStartNPCID, 0x31054)
	end := npc(powerOfElimEndNPCID, 0x31055)
	artifact := npc(powerOfElimArtifactNPCID, 0x31056)
	report := npc(powerOfElimReportNPCID, 0x31057)

	if !selectDialog(start, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, powerOfElimQuestID).Data) {
		t.Fatalf("start page mismatch: %x", packets.last(smDialogWindow))
	}
	if !selectDialog(start, 10000) || questVar(p.quest(powerOfElimQuestID).Vars, 0) != 1 {
		t.Fatalf("start conversation did not advance: %+v", p.quest(powerOfElimQuestID))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 1352, powerOfElimQuestID).Data) {
		t.Fatalf("end NPC stage-one page mismatch: %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 10001) || questVar(p.quest(powerOfElimQuestID).Vars, 0) != 2 {
		t.Fatalf("end NPC did not advance to artifact: %+v", p.quest(powerOfElimQuestID))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10001, powerOfElimQuestID).Data) {
		t.Fatalf("incomplete collect-item check did not show page 10001: %x", packets.last(smDialogWindow))
	}
	if !selectDialog(artifact, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(artifact.id, 1693, powerOfElimQuestID).Data) {
		t.Fatalf("artifact NPC page mismatch: %x", packets.last(smDialogWindow))
	}
	if !selectDialog(artifact, 10002) || questVar(p.quest(powerOfElimQuestID).Vars, 0) != 3 || s.countItems(p, powerOfElimArtifactID) != 1 {
		t.Fatalf("artifact NPC did not grant its item: quest=%+v item=%d", p.quest(powerOfElimQuestID), s.countItems(p, powerOfElimArtifactID))
	}
	if !selectDialog(report, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 2034, powerOfElimQuestID).Data) {
		t.Fatalf("report NPC page mismatch: %x", packets.last(smDialogWindow))
	}
	if !selectDialog(report, 10003) || questVar(p.quest(powerOfElimQuestID).Vars, 0) != 4 || s.countItems(p, powerOfElimReportItemID) != 1 {
		t.Fatalf("report NPC did not grant its item: quest=%+v item=%d", p.quest(powerOfElimQuestID), s.countItems(p, powerOfElimReportItemID))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2375, powerOfElimQuestID).Data) {
		t.Fatalf("end NPC stage-four page mismatch: %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 10004) || questVar(p.quest(powerOfElimQuestID).Vars, 0) != 5 || s.countItems(p, powerOfElimArtifactID) != 0 || s.countItems(p, powerOfElimReportItemID) != 0 {
		t.Fatalf("end NPC did not consume the two proof items: quest=%+v items=%d/%d", p.quest(powerOfElimQuestID), s.countItems(p, powerOfElimArtifactID), s.countItems(p, powerOfElimReportItemID))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2716, powerOfElimQuestID).Data) {
		t.Fatalf("end NPC stage-five page mismatch: %x", packets.last(smDialogWindow))
	}
	if selectDialog(end, 2377) || !bytes.Equal(packets.last(smPlayMovie), playMovie(187).Data) {
		t.Fatalf("movie event should play movie 187 and remain unhandled: %x", packets.last(smPlayMovie))
	}
	if !s.addItem(p, powerOfElimCollectItemID, 50) {
		t.Fatal("could not add the 50 quest drops for turn-in")
	}
	if !selectDialog(end, 33) || p.quest(powerOfElimQuestID).Status != "REWARD" || s.countItems(p, powerOfElimCollectItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, powerOfElimQuestID).Data) {
		t.Fatalf("collect-item turn-in mismatch: quest=%+v items=%d dialog=%x", p.quest(powerOfElimQuestID), s.countItems(p, powerOfElimCollectItemID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, powerOfElimQuestID).Data) {
		t.Fatal("reward click did not show window 5")
	}
	if !selectDialog(end, 8) {
		t.Fatal("first selectable reward was not handled")
	}
	quest := p.quest(powerOfElimQuestID)
	if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || p.Exp != d.ExpStart(31)+987100 {
		t.Fatalf("reward completion mismatch: quest=%+v exp=%d reward=%d", quest, p.Exp, s.countItems(p, template.Rewards[0].SelectableItems[0].ID))
	}
}
