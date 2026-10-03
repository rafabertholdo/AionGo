package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func altgardStartupQuestFixture(t *testing.T, questID int32, vars int32) (*Server, *player, *conn, *questPackets, *data.QuestScript) {
	t.Helper()
	_, s, p, c, script, packets := customQuestPortFixture(t, questID, nil)
	p.Race = "ASMODIANS"
	p.WorldID = altgardDutiesMapID
	p.spawned = true
	quest := store.Quest{ID: questID, Status: "START", Vars: vars}
	p.quests = append(p.quests, quest)
	return s, p, c, packets, script
}

func TestFungusAmongUsDialogsKillsAndClassReward(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, fungusAmongUsQuestID, 0)
	captain := questCatalogNPC(s, p, fungusAmongUsCaptainID, 0x32111)
	scout := questCatalogNPC(s, p, fungusAmongUsScoutID, 0x32112)
	if !c.fungusAmongUsDialog(captain, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(captain.id, 1011, fungusAmongUsQuestID).Data) {
		t.Fatal("initial page")
	}
	if !c.fungusAmongUsDialog(captain, script, 10000) || questVar(p.quest(fungusAmongUsQuestID).Vars, 0) != 1 {
		t.Fatal("captain did not initialize the kill stage")
	}
	if !c.fungusAmongUsDialog(scout, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scout.id, 1352, fungusAmongUsQuestID).Data) {
		t.Fatal("scout page")
	}
	if c.fungusAmongUsDialog(scout, script, 1352) || !bytes.Equal(packets.last(smPlayMovie), playMovie(60).Data) {
		t.Fatal("movie must preserve Java's false return and page echo")
	}
	if !c.fungusAmongUsDialog(scout, script, 10001) || questVar(p.quest(fungusAmongUsQuestID).Vars, 0) != 2 {
		t.Fatal("scout did not advance the variable")
	}
	mushroom := questCatalogNPC(s, p, fungusAmongUsMushroomID, 0x32113)
	for kill := 1; kill <= 5; kill++ {
		if !c.fungusAmongUsKill(mushroom) {
			t.Fatalf("mushroom kill %d was not counted", kill)
		}
	}
	if quest := p.quest(fungusAmongUsQuestID); quest.Status != "REWARD" || questVar(quest.Vars, 0) != 6 {
		t.Fatalf("last kill did not open reward: %+v", quest)
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(mushroom.id, 10, 0).Data) {
		t.Fatal("last kill did not send Java's unscoped window 10")
	}
	before := p.Exp
	if !c.fungusAmongUsDialog(captain, script, 17) || p.quest(fungusAmongUsQuestID).Status != "COMPLETE" || p.Exp-before != s.data.Quests[fungusAmongUsQuestID].Rewards[0].Experience {
		t.Fatal("class reward did not complete the quest")
	}
}

func TestAltgardStartupQuestLevelUpTransitions(t *testing.T) {
	s, p, _, _, _ := altgardStartupQuestFixture(t, fungusAmongUsQuestID, 0)
	p.quests = []store.Quest{
		{ID: fungusAmongUsQuestID, Status: "LOCKED"},
		{ID: encroachersQuestID, Status: "LOCKED"},
		{ID: dangerousCropQuestID, Status: "LOCKED"},
		{ID: scoutItOutQuestID, Status: "LOCKED"},
		{ID: takeTheInitiativeQuestID, Status: "LOCKED"},
		{ID: fearThisQuestID, Status: "LOCKED"},
		{ID: observatoryQuestID, Status: "LOCKED"},
		{ID: impetusiumQuestID, Status: "LOCKED"},
	}
	p.quests = append(p.quests, store.Quest{ID: altgardDutiesPrerequisite, Status: "COMPLETE"})
	p.level = 10
	s.levelUp(p)
	for _, id := range []int32{fungusAmongUsQuestID, encroachersQuestID, dangerousCropQuestID, scoutItOutQuestID, fearThisQuestID} {
		if quest := p.quest(id); quest == nil || quest.Status != "START" {
			t.Fatalf("level-up did not start locked quest %d: %+v", id, quest)
		}
	}
	if p.quest(takeTheInitiativeQuestID).Status != "LOCKED" || p.quest(observatoryQuestID).Status != "LOCKED" || p.quest(impetusiumQuestID).Status != "LOCKED" {
		t.Fatal("campaign follow-ups started without their prerequisites or level")
	}
	p.quest(scoutItOutQuestID).Status = "COMPLETE"
	s.levelUp(p)
	if p.quest(takeTheInitiativeQuestID).Status != "START" || p.quest(observatoryQuestID).Status != "LOCKED" {
		t.Fatal("Take the Initiative did not unlock after Scout it Out, or Observatory skipped its prerequisite")
	}
	p.quest(takeTheInitiativeQuestID).Status = "COMPLETE"
	p.level = 11
	s.levelUp(p)
	if p.quest(observatoryQuestID).Status != "LOCKED" {
		t.Fatal("Observatory unlocked below level 12")
	}
	p.level = 12
	s.levelUp(p)
	if p.quest(observatoryQuestID).Status != "START" || p.quest(impetusiumQuestID).Status != "LOCKED" {
		t.Fatal("Observatory did not honor level 12 / Impetusium unlocked below level 13")
	}
	p.level = 13
	s.levelUp(p)
	if p.quest(impetusiumQuestID).Status != "START" {
		t.Fatal("Reconstructing Impetusium did not unlock at level 13")
	}
}
