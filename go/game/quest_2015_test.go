package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
)

func TestAltgardStartupCampaignHandlersRegistered(t *testing.T) {
	d := staticDataOrSkip(t)
	talks := map[int32][]int32{
		takeTheInitiativeQuestID: {takeInitiativeReportNPCID},
		fearThisQuestID:          {fearThisReportNPCID, fearThisSupplyNPCID},
		observatoryQuestID:       {observatoryTalkNPCID, observatoryReportNPCID},
		impetusiumQuestID:        {impetusiumReportNPCID, impetusiumJewelBoxNPCID, impetusiumGraveNPCID},
	}
	kills := map[int32][]int32{
		takeTheInitiativeQuestID: {210510, 210504, 210506},
		fearThisQuestID:          {210455, 210458, 214032},
		observatoryQuestID:       {210528, 210721},
		impetusiumQuestID:        {210588, impetusiumNamedNPCID},
	}
	for questID, npcIDs := range talks {
		script := d.QuestScripts[questID]
		if script == nil || script.Kind != data.QuestCustom {
			t.Fatalf("quest %d is not registered as a custom handler: %+v", questID, script)
		}
		for _, npcID := range npcIDs {
			found := false
			for _, registered := range d.QuestCustomTalks[npcID] {
				found = found || registered.ID == questID
			}
			if !found {
				t.Errorf("quest %d is missing talk registration for npc %d", questID, npcID)
			}
		}
		for _, npcID := range kills[questID] {
			found := false
			for _, registered := range d.QuestKills[npcID] {
				found = found || registered.ID == questID
			}
			if !found {
				t.Errorf("quest %d is missing kill registration for npc %d", questID, npcID)
			}
		}
	}
	if len(d.QuestItemUses[fearThisItemID]) != 1 || d.QuestItemUses[fearThisItemID][0].ID != fearThisQuestID {
		t.Fatal("Fear This item-use registration is missing")
	}
	for npcID, wantQuest := range map[int32]int32{210445: fearThisQuestID, 210532: observatoryQuestID, impetusiumJewelBoxNPCID: impetusiumQuestID} {
		found := false
		for _, registered := range d.QuestDropsByNPC[npcID] {
			found = found || registered.ID == wantQuest
		}
		if !found {
			t.Errorf("quest %d drop registration is missing for npc %d", wantQuest, npcID)
		}
	}
}

func TestTakeTheInitiativeThreeCountersAndReward(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, takeTheInitiativeQuestID, 0)
	report := questCatalogNPC(s, p, takeInitiativeReportNPCID, 0x32151)
	if !c.takeTheInitiativeDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1011, takeTheInitiativeQuestID).Data) {
		t.Fatal("opening dialogue page")
	}
	if !c.takeTheInitiativeDialog(report, script, 10000) || questVar(p.quest(takeTheInitiativeQuestID).Vars, 0) != 1 {
		t.Fatal("opening dialogue did not advance")
	}
	for _, npcID := range []int32{210510, 210504, 210506} {
		count := 5
		if npcID == 210510 {
			count = 1
		}
		for range count {
			if !c.takeTheInitiativeKill(npcID) {
				t.Fatalf("kill counter did not advance for npc %d", npcID)
			}
		}
		if c.takeTheInitiativeKill(npcID) {
			t.Fatalf("kill counter exceeded its cap for npc %d", npcID)
		}
	}
	quest := p.quest(takeTheInitiativeQuestID)
	if questVar(quest.Vars, 1) != 1 || questVar(quest.Vars, 2) != 5 || questVar(quest.Vars, 3) != 5 {
		t.Fatalf("three objectives = vars %d", quest.Vars)
	}
	if !c.takeTheInitiativeDialog(report, script, -1) || quest.Status != "REWARD" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1352, takeTheInitiativeQuestID).Data) {
		t.Fatal("completed objectives did not open the reward dialogue")
	}
	before := p.Exp
	if !c.takeTheInitiativeDialog(report, script, 8) || quest.Status != "COMPLETE" || p.Exp-before != s.data.Quests[takeTheInitiativeQuestID].Rewards[0].Experience {
		t.Fatal("selectable reward did not complete the quest")
	}
}
