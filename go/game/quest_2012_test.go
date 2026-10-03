package game

import (
	"bytes"
	"testing"
)

func TestEncroachersFourBrutesAndReward(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, encroachersQuestID, 0)
	report := questCatalogNPC(s, p, encroachersReportNPCID, 0x32121)
	brute := questCatalogNPC(s, p, encroachersBruteNPCID, 0x32122)
	if !c.encroachersDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1011, encroachersQuestID).Data) {
		t.Fatal("initial page")
	}
	if !c.encroachersDialog(report, script, 10000) || questVar(p.quest(encroachersQuestID).Vars, 0) != 1 {
		t.Fatal("quest did not enter kill stage")
	}
	for kill := 1; kill <= 4; kill++ {
		if !c.encroachersKill(brute.npc.ID) {
			t.Fatalf("brute kill %d was not counted", kill)
		}
	}
	if p.quest(encroachersQuestID).Status != "REWARD" || questVar(p.quest(encroachersQuestID).Vars, 0) != 4 {
		t.Fatalf("fourth kill did not complete progress: %+v", p.quest(encroachersQuestID))
	}
	before := p.Exp
	if !c.encroachersDialog(report, script, 17) || p.quest(encroachersQuestID).Status != "COMPLETE" || p.Exp-before != s.data.Quests[encroachersQuestID].Rewards[0].Experience {
		t.Fatal("fixed reward did not complete the quest")
	}
}
