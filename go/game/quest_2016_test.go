package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestFearThisKillsCollectionItemUseAndReward(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, fearThisQuestID, 0)
	report := questCatalogNPC(s, p, fearThisReportNPCID, 0x32161)
	supply := questCatalogNPC(s, p, fearThisSupplyNPCID, 0x32162)
	if !c.fearThisDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1011, fearThisQuestID).Data) ||
		!c.fearThisDialog(report, script, 10000) {
		t.Fatal("opening dialogue")
	}
	for range 5 {
		if !c.fearThisKill(210455) {
			t.Fatal("kill counter stopped before Java's var-six limit")
		}
	}
	if c.fearThisKill(210458) || questVar(p.quest(fearThisQuestID).Vars, 0) != 6 {
		t.Fatal("kill counter exceeded Java's var-six limit")
	}
	if !c.fearThisDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1352, fearThisQuestID).Data) ||
		!c.fearThisDialog(report, script, 10001) || !c.fearThisDialog(supply, script, 25) || !c.fearThisDialog(supply, script, 10002) {
		t.Fatal("supply collection stage")
	}
	if !c.fearThisDialog(supply, script, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(supply.id, 2120, fearThisQuestID).Data) {
		t.Fatal("missing collection did not show Java's failure page")
	}
	for _, item := range s.data.Quests[fearThisQuestID].CollectItems {
		if !s.addItem(p, item.ID, item.Count) {
			t.Fatalf("could not add collection item %d", item.ID)
		}
	}
	if !c.fearThisDialog(supply, script, 33) || questVar(p.quest(fearThisQuestID).Vars, 0) != 9 || s.countItems(p, 182203018) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(supply.id, 2035, fearThisQuestID).Data) {
		t.Fatal("collection was not consumed and advanced")
	}
	if !c.fearThisDialog(supply, script, 10003) || questVar(p.quest(fearThisQuestID).Vars, 0) != 10 || s.countItems(p, fearThisItemID) != 1 {
		t.Fatal("item reward stage")
	}
	p.zone = &data.Zone{Name: fearThisZone, MapID: altgardDutiesMapID}
	var item *store.Item
	for _, candidate := range p.cube {
		if candidate.ItemID == fearThisItemID {
			item = candidate
			break
		}
	}
	if item == nil || !c.fearThisItemUse(item) || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0).Data) {
		t.Fatal("item use did not start in Q2016")
	}
	time.Sleep(3100 * time.Millisecond)
	quest := p.quest(fearThisQuestID)
	if quest.Status != "REWARD" || s.countItems(p, fearThisItemID) != 0 || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0).Data) {
		t.Fatal("delayed item use did not consume the item and complete the quest")
	}
	if !c.fearThisDialog(report, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 2375, fearThisQuestID).Data) {
		t.Fatal("reward dialogue did not show Java's page 2375")
	}
	before := p.Exp
	if !c.fearThisDialog(report, script, 8) || quest.Status != "COMPLETE" || p.Exp-before != s.data.Quests[fearThisQuestID].Rewards[0].Experience {
		t.Fatal("selectable reward did not complete the quest")
	}
}
