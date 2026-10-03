package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSecretDeliveryNPCChainAndJavaFallthrough(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, secretDeliveryQuestID, []store.Quest{{ID: 1219, Status: "COMPLETE"}})
	template := d.Quests[secretDeliveryQuestID]
	if script.Kind != data.QuestCustom || !script.NPCStart || script.StartNPC != secretDeliveryStartID || script.MiddleNPC != secretDeliveryMiddleID || script.EndNPC != secretDeliveryEndID ||
		template.Race != "ELYOS" || template.MinLevel != 19 || len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1219 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 61900 || template.Rewards[0].Kinah != 2000 {
		t.Fatalf("unexpected A Secret Delivery metadata: script=%+v template=%+v", script, template)
	}
	if !slicesContainsQuest(d.QuestStarts[secretDeliveryStartID], secretDeliveryQuestID) || !slicesContainsQuest(d.QuestEnds[secretDeliveryMiddleID], secretDeliveryQuestID) || !slicesContainsQuest(d.QuestEnds[secretDeliveryEndID], secretDeliveryQuestID) {
		t.Fatal("A Secret Delivery is missing a talk-event index")
	}
	startNPC := questCatalogNPC(s, p, secretDeliveryStartID, 0x31220)
	middleNPC := questCatalogNPC(s, p, secretDeliveryMiddleID, 0x32220)
	endNPC := questCatalogNPC(s, p, secretDeliveryEndID, 0x33220)

	if !c.secretDeliveryDialog(startNPC, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, secretDeliveryQuestID).Data) {
		t.Fatalf("offer page = %x", packets.last(smDialogWindow))
	}
	if !c.secretDeliveryDialog(startNPC, script, 1002) || p.quest(secretDeliveryQuestID) == nil || p.quest(secretDeliveryQuestID).Status != "START" {
		t.Fatal("quest was not accepted")
	}
	if !c.secretDeliveryDialog(middleNPC, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middleNPC.id, 1352, secretDeliveryQuestID).Data) {
		t.Fatalf("middle page = %x", packets.last(smDialogWindow))
	}
	if !c.secretDeliveryDialog(middleNPC, script, 10000) || questVar(p.quest(secretDeliveryQuestID).Vars, 0) != 1 {
		t.Fatal("middle report did not advance the quest variable")
	}
	if !c.secretDeliveryDialog(endNPC, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 2375, secretDeliveryQuestID).Data) {
		t.Fatalf("final page = %x", packets.last(smDialogWindow))
	}
	if !c.secretDeliveryDialog(endNPC, script, 1009) || p.quest(secretDeliveryQuestID).Status != "REWARD" || questVar(p.quest(secretDeliveryQuestID).Vars, 0) != 2 {
		t.Fatalf("final report did not ready reward: %+v", p.quest(secretDeliveryQuestID))
	}
	if !c.secretDeliveryDialog(endNPC, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, secretDeliveryQuestID).Data) {
		t.Fatal("reward preview was not shown")
	}
	if !c.secretDeliveryDialog(endNPC, script, 17) || p.quest(secretDeliveryQuestID).Status != "COMPLETE" {
		t.Fatalf("fixed reward did not complete quest: %+v", p.quest(secretDeliveryQuestID))
	}

	// Java's 798004 switch falls through to the 798046 case on dialog 1009.
	_, _, secondPlayer, secondConn, secondScript, _ := customQuestPortFixture(t, secretDeliveryQuestID, []store.Quest{
		{ID: 1219, Status: "COMPLETE"},
		{ID: secretDeliveryQuestID, Status: "START", Vars: 1},
	})
	secondMiddleNPC := questCatalogNPC(secondConn.s, secondPlayer, secretDeliveryMiddleID, 0x34220)
	if !secondConn.secretDeliveryDialog(secondMiddleNPC, secondScript, 1009) || secondPlayer.quest(secretDeliveryQuestID).Status != "REWARD" || questVar(secondPlayer.quest(secretDeliveryQuestID).Vars, 0) != 2 {
		t.Fatalf("Java's middle-NPC fallthrough was lost: %+v", secondPlayer.quest(secretDeliveryQuestID))
	}
}

func TestSecretDeliveryEligibilityAndRewardClickRouting(t *testing.T) {
	_, s, p, c, script, packets := customQuestPortFixture(t, secretDeliveryQuestID, nil)
	startNPC := questCatalogNPC(s, p, secretDeliveryStartID, 0x35220)
	if c.secretDeliveryDialog(startNPC, script, 1002) || p.quest(secretDeliveryQuestID) != nil {
		t.Fatal("quest started before prerequisite 1219")
	}
	p.quests = append(p.quests, store.Quest{ID: 1219, Status: "COMPLETE"})
	c.secretDeliveryDialog(startNPC, script, 1002)
	if p.quest(secretDeliveryQuestID) == nil {
		t.Fatal("quest did not start after prerequisite 1219")
	}
	endNPC := questCatalogNPC(s, p, secretDeliveryEndID, 0x36220)
	p.quest(secretDeliveryQuestID).Status = "REWARD"
	c.showDialog(dialogRequest(cmShowDialog, endNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, secretDeliveryQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	middleNPC := questCatalogNPC(s, p, secretDeliveryMiddleID, 0x37220)
	c.showDialog(dialogRequest(cmShowDialog, middleNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middleNPC.id, 10, 0).Data) {
		t.Fatalf("Java does not offer reward at the middle NPC, got %x", packets.last(smDialogWindow))
	}
}
