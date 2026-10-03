package game

import (
	"bytes"
	"testing"
)

func TestObservatoryKillsCollectionAndReward(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, observatoryQuestID, 0)
	report := questCatalogNPC(s, p, observatoryReportNPCID, 0x32171)
	commander := questCatalogNPC(s, p, observatoryTalkNPCID, 0x32172)
	if !c.observatoryDialog(commander, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(commander.id, 1011, observatoryQuestID).Data) ||
		!c.observatoryDialog(commander, script, 10000) {
		t.Fatal("opening dialogue")
	}
	for range 5 {
		if !c.observatoryKill(210528) {
			t.Fatal("kill counter stopped before Java's var-six limit")
		}
	}
	if c.observatoryKill(210721) || questVar(p.quest(observatoryQuestID).Vars, 0) != 6 {
		t.Fatal("kill counter exceeded Java's var-six limit")
	}
	if !c.observatoryDialog(commander, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(commander.id, 1352, observatoryQuestID).Data) ||
		!c.observatoryDialog(commander, script, 10001) {
		t.Fatal("observatory report stage")
	}
	if !c.observatoryDialog(commander, script, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(commander.id, 1779, observatoryQuestID).Data) {
		t.Fatal("missing item did not show the Java failure page")
	}
	for _, item := range s.data.Quests[observatoryQuestID].CollectItems {
		if !s.addItem(p, item.ID, item.Count) {
			t.Fatalf("could not add observatory item %d", item.ID)
		}
	}
	if !c.observatoryDialog(commander, script, 33) || p.quest(observatoryQuestID).Status != "REWARD" || s.countItems(p, 182203020) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(commander.id, 1694, observatoryQuestID).Data) {
		t.Fatal("collection did not open the reward state")
	}
	if !c.observatoryDialog(report, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 2034, observatoryQuestID).Data) {
		t.Fatal("reward preview page")
	}
	before := p.Exp
	if !c.observatoryDialog(report, script, 8) || p.quest(observatoryQuestID).Status != "COMPLETE" || p.Exp-before != s.data.Quests[observatoryQuestID].Rewards[0].Experience {
		t.Fatal("selectable reward did not complete the quest")
	}
}
