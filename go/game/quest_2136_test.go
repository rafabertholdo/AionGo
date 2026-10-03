package game

import (
	"testing"
	"time"

	"aionlightning/game/store"
)

func TestLostAxeItemUseObjectSpawnAndReward(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, lostAxeQuestID, nil)
	p.Race = "ASMODIANS"
	p.level = d.Quests[lostAxeQuestID].MinLevel
	p.spawned = true
	itemUses := d.QuestItemUses[lostAxeWorkItemID]
	if len(itemUses) != 1 || itemUses[0].ID != lostAxeQuestID || len(d.QuestActions[lostAxeActionNPCID]) != 1 {
		t.Fatal("item or action-object registration is missing")
	}
	if !s.addItem(p, lostAxeWorkItemID, 1) || !c.questStartItemUse(p.cube[0], script) || packets.last(smDialogWindow) == nil {
		t.Fatal("quest item use did not open the item-start window")
	}
	if !c.itemStartedQuestDialog(script, 1002) || p.quest(lostAxeQuestID) == nil || p.quest(lostAxeQuestID).Status != "START" {
		t.Fatal("item-start dialog did not start the quest")
	}
	grave := questCatalogNPC(s, p, lostAxeActionNPCID, 0x3136)
	if !c.lostAxeAction(grave, script, -1) || grave.useTask == nil || string(packets.last(smUseObject)) != string(useObject(p.ID, grave.id, 1).Data) {
		t.Fatal("grave interaction did not begin")
	}
	if !c.lostAxeAction(grave, script, -1) {
		t.Fatal("repeated object click was not acknowledged")
	}
	time.Sleep(3100 * time.Millisecond)
	quest := p.quest(lostAxeQuestID)
	if questVar(quest.Vars, 0) != 1 || string(packets.last(smPlayMovie)) != string(playMovie(lostAxeMovieID).Data) || string(packets.last(smUseObject)) != string(useObject(p.ID, grave.id, 0).Data) {
		t.Fatalf("grave timer did not finish: quest=%+v movie=%x use=%x", quest, packets.last(smPlayMovie), packets.last(smUseObject))
	}
	var report *object
	for _, object := range s.byID {
		if object.npc != nil && object.npc.ID == lostAxeReportNPCID {
			report = object
			break
		}
	}
	if report == nil || report.worldID != lostAxeReportWorldID || report.instance != p.instance || report.x != 1088.5 || report.y != 2371.8 || report.z != 258.375 {
		t.Fatalf("temporary report NPC = %+v", report)
	}
	if !c.lostAxeAsmodianDialog(report, script, 25) || string(packets.last(smDialogWindow)) != string(dialogWindow(report.id, 1011, lostAxeQuestID).Data) {
		t.Fatal("report NPC did not show its page")
	}
	if !s.addItem(p, lostAxeWorkItemID, 2) {
		t.Fatal("could not add duplicate work items")
	}
	if !c.lostAxeAsmodianDialog(report, script, 10000) || quest.Status != "REWARD" || s.countItems(p, lostAxeWorkItemID) != 0 {
		t.Fatalf("report turn-in = %+v, item count=%d", quest, s.countItems(p, lostAxeWorkItemID))
	}
	if !c.lostAxeAsmodianDialog(report, script, -1) || string(packets.last(smDialogWindow)) != string(dialogWindow(report.id, 5, lostAxeQuestID).Data) {
		t.Fatal("reward click did not show the default end page")
	}
	c.lostAxeAsmodianDialog(report, script, 17)
	if quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("quest did not complete: %+v", quest)
	}
}

func TestLostAxeAlternateReportChoice(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, lostAxeQuestID, []store.Quest{{ID: lostAxeQuestID, Status: "START", Vars: 1}})
	p.Race = "ASMODIANS"
	report := questCatalogNPC(s, p, lostAxeReportNPCID, 0x3236)
	if !s.addItem(p, lostAxeWorkItemID, 1) || !c.lostAxeAsmodianDialog(report, script, 10001) {
		t.Fatal("alternate report dialog failed")
	}
	if p.quest(lostAxeQuestID).Status != "REWARD" || string(packets.last(smDialogWindow)) != string(dialogWindow(report.id, 5, lostAxeQuestID).Data) {
		t.Fatalf("alternate report status=%+v page=%x", p.quest(lostAxeQuestID), packets.last(smDialogWindow))
	}
	_ = d
}
