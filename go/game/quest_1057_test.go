package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestCreatingAMonsterProgressEventsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[creatingMonsterQuestID], d.Quests[creatingMonsterQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != creatingMonsterStartNPCID || script.EndNPC != creatingMonsterEndNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 37 || template.NameID != 2204213 || len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1056 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: creatingMonsterArtifactID, Count: 1}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 2897900 || template.Rewards[0].TitleID != 24 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Creating a Monster metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{creatingMonsterStartNPCID, creatingMonsterSecondNPCID, creatingMonsterEndNPCID, creatingMonsterTabletID, creatingMonsterFinalObjectID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC/object %d", npcID)
		}
	}
	for _, npcID := range []int32{creatingMonsterKillNPCID, creatingMonsterFinalKillID} {
		found := false
		for _, registered := range d.QuestKills[npcID] {
			found = found || registered.ID == creatingMonsterQuestID
		}
		if !found {
			t.Fatalf("quest kill index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 36
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.WorldID = creatingMonsterHeironWorldID + 1
	p.quests = []store.Quest{{ID: creatingMonsterQuestID, Status: "LOCKED"}, {ID: 1056, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.creatingMonsterLevelUp() {
		t.Fatal("quest unlocked below level 37")
	}
	p.level = 37
	if c.creatingMonsterLevelUp() {
		t.Fatal("quest unlocked before quest 1056 was completed")
	}
	p.quest(1056).Status = "COMPLETE"
	if !c.creatingMonsterLevelUp() || p.quest(creatingMonsterQuestID).Status != "START" || c.creatingMonsterLevelUp() {
		t.Fatalf("quest did not unlock exactly once after its prerequisite: %+v", p.quest(creatingMonsterQuestID))
	}

	makeNPC := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	startNPC := makeNPC(creatingMonsterStartNPCID, 0x31151)
	secondNPC := makeNPC(creatingMonsterSecondNPCID, 0x31152)
	endNPC := makeNPC(creatingMonsterEndNPCID, 0x31153)
	tablet := &object{id: 0x31154, npc: &data.NpcTemplate{ID: creatingMonsterTabletID}}
	finalObject := &object{id: 0x31155, npc: &data.NpcTemplate{ID: creatingMonsterFinalObjectID}}
	selectDialog := func(o *object, dialogID int32) bool {
		return c.creatingMonsterDialog(o, script, dialogID)
	}

	if !selectDialog(startNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, creatingMonsterQuestID).Data) {
		t.Fatalf("opening offer page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(startNPC, 10000) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 1 {
		t.Fatalf("opening conversation did not advance: %+v", p.quest(creatingMonsterQuestID))
	}
	if !selectDialog(secondNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(secondNPC.id, 1352, creatingMonsterQuestID).Data) {
		t.Fatalf("second NPC offer page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(secondNPC, 10001) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 2 {
		t.Fatalf("second NPC conversation did not advance: %+v", p.quest(creatingMonsterQuestID))
	}
	if selectDialog(tablet, -1) || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, tablet.id, 1).Data) {
		t.Fatalf("tablet interaction did not begin: task=%v use=%x", tablet.useTask != nil, packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(tablet.id, 1693, creatingMonsterQuestID).Data) {
		t.Fatalf("tablet interaction did not present its page: task=%v dialog=%x", tablet.useTask != nil, packets.last(smDialogWindow))
	}
	if !selectDialog(tablet, 10002) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 3 || s.countItems(p, creatingMonsterArtifactID) != 1 {
		t.Fatalf("tablet did not grant the artifact and advance: quest=%+v items=%d", p.quest(creatingMonsterQuestID), s.countItems(p, creatingMonsterArtifactID))
	}
	if !selectDialog(startNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 2034, creatingMonsterQuestID).Data) {
		t.Fatalf("follow-up page = %x", packets.last(smDialogWindow))
	}
	if selectDialog(startNPC, 2036) || packets.last(smPlayMovie) == nil || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 3 {
		t.Fatalf("movie branch changed quest progress or did not play: movie=%x quest=%+v", packets.last(smPlayMovie), p.quest(creatingMonsterQuestID))
	}
	if !selectDialog(startNPC, 10003) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 4 {
		t.Fatalf("report did not advance to map transition: %+v", p.quest(creatingMonsterQuestID))
	}
	p.WorldID = creatingMonsterHeironWorldID + 1
	c.creatingMonsterEnterWorld()
	if questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 4 {
		t.Fatal("entering another world advanced the quest")
	}
	p.WorldID = creatingMonsterHeironWorldID
	c.creatingMonsterEnterWorld()
	if questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 5 {
		t.Fatalf("entering Heiron did not advance the quest: %+v", p.quest(creatingMonsterQuestID))
	}
	if c.creatingMonsterKill(creatingMonsterFinalKillID) {
		t.Fatal("final monster kill advanced before the ordinary kills")
	}

	for killCount := int32(0); killCount < 3; killCount++ {
		if !c.creatingMonsterKill(creatingMonsterKillNPCID) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != killCount+6 {
			t.Fatalf("ordinary monster kill %d did not increment the objective: %+v", killCount+1, p.quest(creatingMonsterQuestID))
		}
	}
	if c.creatingMonsterKill(creatingMonsterKillNPCID) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 8 {
		t.Fatalf("ordinary kill objective accepted an extra target: %+v", p.quest(creatingMonsterQuestID))
	}
	if !c.creatingMonsterKill(creatingMonsterFinalKillID) || questVar(p.quest(creatingMonsterQuestID).Vars, 0) != 9 {
		t.Fatalf("final monster kill did not advance to the object: %+v", p.quest(creatingMonsterQuestID))
	}
	if selectDialog(finalObject, -1) || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, finalObject.id, 1).Data) {
		t.Fatalf("final object interaction did not begin: task=%v use=%x", finalObject.useTask != nil, packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if p.quest(creatingMonsterQuestID).Status != "REWARD" {
		t.Fatalf("final object did not make quest ready for reward: task=%v quest=%+v", finalObject.useTask != nil, p.quest(creatingMonsterQuestID))
	}
	if !selectDialog(endNPC, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 10002, creatingMonsterQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(endNPC, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, creatingMonsterQuestID).Data) {
		t.Fatalf("reward selection page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(endNPC, 8) || p.quest(creatingMonsterQuestID).Status != "COMPLETE" || p.quest(creatingMonsterQuestID).CompleteCount != 1 ||
		s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || !bytes.Equal(packets.last(smTitleList), titleList(p).Data) {
		t.Fatalf("quest reward did not complete: quest=%+v selected item=%d", p.quest(creatingMonsterQuestID), s.countItems(p, template.Rewards[0].SelectableItems[0].ID))
	}
}
