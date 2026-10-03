package game

import (
	"testing"
	"time"

	"aionlightning/game/store"
)

func TestImprisonedGourmetStartAndTurnInBranches(t *testing.T) {
	for _, tc := range []struct {
		name     string
		itemID   int32
		dialogID int32
		variable int32
		page     uint16
		removed  bool
	}{
		{name: "proof checked but Java removes mismatched item", itemID: 182203121, dialogID: 10000, variable: 5, page: 5, removed: false},
		{name: "second proof", itemID: 182203122, dialogID: 10001, variable: 6, page: 6, removed: true},
		{name: "third proof", itemID: 182203123, dialogID: 10002, variable: 7, page: 7, removed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s, p, c, script, packets := customQuestPortFixture(t, imprisonedGourmetQuestID, nil)
			p.Race = "ASMODIANS"
			p.quests = append(p.quests, store.Quest{ID: imprisonedGourmetQuestID, Status: "START"})
			if !s.addItem(p, tc.itemID, 2) {
				t.Fatal("could not add proof item")
			}
			if tc.dialogID == 10000 && !s.addItem(p, 182004687, 1) {
				t.Fatal("could not add Java's mismatched removal item")
			}
			npc := questCatalogNPC(s, p, imprisonedGourmetStartNPC, 0x3123)
			if !c.imprisonedGourmetDialog(npc, script, tc.dialogID) {
				t.Fatal("proof turn-in was not handled")
			}
			quest := p.quest(imprisonedGourmetQuestID)
			if quest.Status != "REWARD" || questVar(quest.Vars, 0) != tc.variable {
				t.Fatalf("quest state = %+v", quest)
			}
			if got := packets.last(smDialogWindow); string(got) != string(dialogWindow(npc.id, tc.page, imprisonedGourmetQuestID).Data) {
				t.Fatalf("turn-in page = %x", got)
			}
			if got := s.countItems(p, tc.itemID); (got == 0) != tc.removed {
				t.Fatalf("proof item count = %d, removed=%v", got, tc.removed)
			}
			if tc.dialogID == 10000 && s.countItems(p, 182004687) != 0 {
				t.Fatal("first branch did not preserve Java's mismatched item removal")
			}
			c.imprisonedGourmetDialog(npc, script, 17)
			if quest.Status != "COMPLETE" {
				t.Fatalf("fixed reward did not complete quest: %+v", quest)
			}
		})
	}
}

func TestImprisonedGourmetStartAndMissingProof(t *testing.T) {
	_, s, p, c, script, packets := customQuestPortFixture(t, imprisonedGourmetQuestID, nil)
	p.Race = "ASMODIANS"
	npc := questCatalogNPC(s, p, imprisonedGourmetStartNPC, 0x3124)
	if !c.imprisonedGourmetDialog(npc, script, 25) || string(packets.last(smDialogWindow)) != string(dialogWindow(npc.id, 1011, imprisonedGourmetQuestID).Data) {
		t.Fatal("initial offer page")
	}
	if !c.imprisonedGourmetDialog(npc, script, 1002) || p.quest(imprisonedGourmetQuestID) == nil || p.quest(imprisonedGourmetQuestID).Status != "START" {
		t.Fatal("default start dialog did not start the quest")
	}
	if !c.imprisonedGourmetDialog(npc, script, 25) || string(packets.last(smDialogWindow)) != string(dialogWindow(npc.id, 1352, imprisonedGourmetQuestID).Data) {
		t.Fatal("started quest page")
	}
	if !c.imprisonedGourmetDialog(npc, script, 10000) || p.quest(imprisonedGourmetQuestID).Status != "START" || string(packets.last(smDialogWindow)) != string(dialogWindow(npc.id, 1693, imprisonedGourmetQuestID).Data) {
		t.Fatal("missing proof did not show the required page")
	}
}

func TestImprisonedGourmetRewardPreviewUsesQuestVariable(t *testing.T) {
	for _, tc := range []struct {
		variable int32
		page     uint16
	}{
		{5, 5},
		{6, 6},
		{7, 7},
	} {
		t.Run(string(rune('0'+tc.variable)), func(t *testing.T) {
			_, s, p, c, script, packets := customQuestPortFixture(t, imprisonedGourmetQuestID, []store.Quest{{ID: imprisonedGourmetQuestID, Status: "REWARD", Vars: tc.variable}})
			p.Race = "ASMODIANS"
			npc := questCatalogNPC(s, p, imprisonedGourmetStartNPC, 0x3126)
			if !c.imprisonedGourmetDialog(npc, script, 25) || string(packets.last(smDialogWindow)) != string(dialogWindow(npc.id, tc.page, imprisonedGourmetQuestID).Data) {
				t.Fatalf("variable %d reward preview = %x", tc.variable, packets.last(smDialogWindow))
			}
		})
	}
}

func TestImprisonedGourmetTalkTimerAndRegistration(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, imprisonedGourmetQuestID, []store.Quest{{ID: imprisonedGourmetQuestID, Status: "START"}})
	p.Race = "ASMODIANS"
	npc := questCatalogNPC(s, p, imprisonedGourmetTalkNPC, 0x3125)
	if len(d.QuestStarts[imprisonedGourmetStartNPC]) != 1 || len(d.QuestCustomTalks[imprisonedGourmetTalkNPC]) != 1 {
		t.Fatal("NPC start or auxiliary talk event is not registered")
	}
	if !c.imprisonedGourmetDialog(npc, script, -1) || packets.last(smEmotion) == nil {
		t.Fatal("auxiliary NPC did not play the Java emotion")
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(imprisonedGourmetQuestID).Vars, 0) != 0 || packets.last(smQuestAccepted) == nil {
		t.Fatal("three-second callback did not persist and publish the Java variable update")
	}
}
