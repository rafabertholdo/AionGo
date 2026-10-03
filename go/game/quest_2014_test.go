package game

import (
	"bytes"
	"testing"
	"time"
)

func TestScoutItOutGraveEvidenceNamedKillAndRewardChoice(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, scoutItOutQuestID, 0)
	start := questCatalogNPC(s, p, scoutItOutStartNPCID, 0x32141)
	grave := questCatalogNPC(s, p, scoutItOutGraveNPCID, 0x32142)
	scout := questCatalogNPC(s, p, scoutItOutScoutNPCID, 0x32143)
	report := questCatalogNPC(s, p, scoutItOutReportNPCID, 0x32144)
	if !c.scoutItOutDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, scoutItOutQuestID).Data) {
		t.Fatal("initial page")
	}
	if !c.scoutItOutDialog(start, script, 10000) || questVar(p.quest(scoutItOutQuestID).Vars, 0) != 1 {
		t.Fatal("quest did not enter grave stage")
	}
	p.targetID = grave.id
	if !c.scoutItOutDialog(grave, script, -1) || grave.useTask == nil {
		t.Fatal("grave interaction did not start")
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(scoutItOutQuestID).Vars, 0) != 2 || s.countItems(p, scoutItOutEvidenceItemID) != 1 {
		t.Fatal("grave did not grant its evidence item and progress")
	}
	if !c.scoutItOutDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1352, scoutItOutQuestID).Data) {
		t.Fatal("evidence turn-in page")
	}
	if !c.scoutItOutDialog(start, script, 10001) || questVar(p.quest(scoutItOutQuestID).Vars, 0) != 3 || s.countItems(p, scoutItOutEvidenceItemID) != 0 {
		t.Fatal("start NPC did not consume the evidence and advance")
	}
	if !c.scoutItOutDialog(scout, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scout.id, 1693, scoutItOutQuestID).Data) || !c.scoutItOutDialog(scout, script, 10002) {
		t.Fatal("scout report step")
	}
	if !c.scoutItOutKill(scoutItOutNamedNPCID) || p.quest(scoutItOutQuestID).Status != "REWARD" {
		t.Fatal("named scout kill did not open reward")
	}
	before := p.Exp
	if !c.scoutItOutDialog(report, script, 8) || p.quest(scoutItOutQuestID).Status != "COMPLETE" || p.Exp-before != s.data.Quests[scoutItOutQuestID].Rewards[0].Experience {
		t.Fatal("selectable reward did not complete the quest")
	}
}
