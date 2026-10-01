package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestVerteronReinforcementsTalkChainAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[verteronReinforcementsQuestID], d.Quests[verteronReinforcementsQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || script.StartNPC != verteronReinforcementsStartNPCID ||
		script.EndNPC != verteronReinforcementsStartNPCID || !script.NPCStart || template.Race != "ELYOS" || template.MinLevel != 19 ||
		template.NameID != 2204725 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 48800 {
		t.Fatalf("unexpected Verteron Reinforcements metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{verteronReinforcementsStartNPCID, verteronReinforcementsFirstNPCID, verteronReinforcementsSecondNPCID} {
		if len(d.QuestCustomTalks[npcID]) == 0 {
			t.Fatalf("custom talk index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 19
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	npcs := map[int32]*object{}
	for index, npcID := range []int32{verteronReinforcementsStartNPCID, verteronReinforcementsFirstNPCID, verteronReinforcementsSecondNPCID} {
		npc := questCatalogNPC(s, p, npcID, int32(0x31190+index))
		npc.npc = d.Npcs[npcID]
		if npc.npc == nil {
			t.Fatalf("NPC template %d is missing", npcID)
		}
		s.initNpc(npc)
		npcs[npcID] = npc
	}
	selectDialog := func(npcID int32, dialogID int32) bool {
		return c.verteronReinforcementsDialog(npcs[npcID], script, dialogID)
	}
	if !selectDialog(verteronReinforcementsStartNPCID, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npcs[verteronReinforcementsStartNPCID].id, 1011, verteronReinforcementsQuestID).Data) {
		t.Fatalf("quest offer page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(verteronReinforcementsStartNPCID, 1002) {
		t.Fatal("eligible character did not start the quest")
	}
	quest := p.quest(verteronReinforcementsQuestID)
	if quest == nil || quest.Status != "START" || quest.Vars != 0 {
		t.Fatalf("unexpected started quest state: %+v", quest)
	}
	if !selectDialog(verteronReinforcementsFirstNPCID, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npcs[verteronReinforcementsFirstNPCID].id, 1352, verteronReinforcementsQuestID).Data) {
		t.Fatalf("first reporter page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(verteronReinforcementsFirstNPCID, 10000) || questVar(quest.Vars, 0) != 1 {
		t.Fatalf("first reporter did not advance the quest: %+v", quest)
	}
	if !selectDialog(verteronReinforcementsSecondNPCID, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npcs[verteronReinforcementsSecondNPCID].id, 1693, verteronReinforcementsQuestID).Data) {
		t.Fatalf("second reporter page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(verteronReinforcementsSecondNPCID, 10001) || questVar(quest.Vars, 0) != 2 {
		t.Fatalf("second reporter did not advance the quest: %+v", quest)
	}
	if !selectDialog(verteronReinforcementsStartNPCID, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npcs[verteronReinforcementsStartNPCID].id, 2375, verteronReinforcementsQuestID).Data) {
		t.Fatalf("final report page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(verteronReinforcementsStartNPCID, 1009) || quest.Status != "REWARD" || questVar(quest.Vars, 0) != 3 {
		t.Fatalf("final reporter did not move quest to reward: %+v", quest)
	}
	if !selectDialog(verteronReinforcementsStartNPCID, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npcs[verteronReinforcementsStartNPCID].id, 5, verteronReinforcementsQuestID).Data) {
		t.Fatalf("reward preview page = %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	if !selectDialog(verteronReinforcementsStartNPCID, 8) || p.quest(verteronReinforcementsQuestID).Status != "COMPLETE" ||
		p.quest(verteronReinforcementsQuestID).CompleteCount != 1 || p.Exp-beforeExperience != 48800 {
		t.Fatalf("quest did not complete with its fixed reward: quest=%+v experience gained=%d", p.quest(verteronReinforcementsQuestID), p.Exp-beforeExperience)
	}
}
