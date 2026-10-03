package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestRefreshingSpringsLevelUpAndFullRoute(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[refreshingSpringsQuestID], d.Quests[refreshingSpringsQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 26 || template.NameID != 2204109 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: springWaterItemID, Count: 1}) || len(template.QuestDrops) != 3 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 340400 || len(template.Rewards[0].SelectableItems) != 7 {
		t.Fatalf("unexpected Refreshing the Springs metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{springsGuideNPCID, springGuideTwoNPCID, springObjectOneNPCID, springGuideThreeNPCID, springGuideFourNPCID, springGuideFiveNPCID, springObjectTwoNPCID, springGuideSixNPCID, springObjectThreeNPCID} {
		if len(d.QuestCustomTalks[npcID]) == 0 {
			t.Fatalf("Refreshing the Springs talk index is missing NPC %d", npcID)
		}
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 25
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: refreshingSpringsQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.refreshingSpringsLevelUp() {
		t.Fatal("quest unlocked below level 26")
	}
	p.level = 26
	if !c.refreshingSpringsLevelUp() || p.quest(refreshingSpringsQuestID).Status != "START" || c.refreshingSpringsLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(refreshingSpringsQuestID))
	}
	npc := func(id, objectID int32) *object {
		return questCatalogNPC(s, p, id, objectID)
	}
	valerius := npc(springsGuideNPCID, 0x31050)
	guideTwo := npc(springGuideTwoNPCID, 0x31051)
	waterSource := npc(springObjectOneNPCID, 0x31052)
	guideFour := npc(springGuideFourNPCID, 0x31053)
	guideFive := npc(springGuideFiveNPCID, 0x31054)
	secondSource := npc(springObjectTwoNPCID, 0x31055)
	guideSix := npc(springGuideSixNPCID, 0x31056)
	thirdSource := npc(springObjectThreeNPCID, 0x31057)
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, refreshingSpringsQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, valerius.id, 0, 0))
	selectDialog(valerius, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(valerius.id, 1011, refreshingSpringsQuestID).Data) {
		t.Fatalf("Valerius opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(valerius, 10000)
	selectDialog(guideTwo, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideTwo.id, 1352, refreshingSpringsQuestID).Data) {
		t.Fatalf("first spring-guide page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideTwo, 10001)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 2 {
		t.Fatalf("first guide did not advance the route: %+v", p.quest(refreshingSpringsQuestID))
	}
	if !s.addItem(p, springWaterItemID, 1) {
		t.Fatal("could not add the spring water quest item")
	}
	c.showDialog(dialogRequest(cmShowDialog, waterSource.id, 0, 0))
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 2 || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, waterSource.id, 1).Data) {
		t.Fatalf("first spring did not start its interaction: quest=%+v task=%v packet=%x", p.quest(refreshingSpringsQuestID), waterSource.useTask != nil, packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 3 || s.countItems(p, springWaterItemID) != 0 {
		t.Fatalf("first spring did not consume water and advance: quest=%+v items=%d task=%v", p.quest(refreshingSpringsQuestID), s.countItems(p, springWaterItemID), waterSource.useTask != nil)
	}
	selectDialog(guideTwo, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideTwo.id, 1693, refreshingSpringsQuestID).Data) {
		t.Fatalf("second spring-guide page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideTwo, 10002)
	selectDialog(valerius, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(valerius.id, 1352, refreshingSpringsQuestID).Data) {
		t.Fatalf("Valerius follow-up page = %x", packets.last(smDialogWindow))
	}
	selectDialog(valerius, 10001)
	selectDialog(guideFour, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideFour.id, 2375, refreshingSpringsQuestID).Data) {
		t.Fatalf("third spring-guide page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideFour, 10004)
	selectDialog(guideFive, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideFive.id, 2716, refreshingSpringsQuestID).Data) {
		t.Fatalf("fourth spring-guide page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideFive, 10005)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 7 || s.countItems(p, springProofOneItemID) != 1 {
		t.Fatalf("first proof was not issued: quest=%+v items=%d", p.quest(refreshingSpringsQuestID), s.countItems(p, springProofOneItemID))
	}
	c.showDialog(dialogRequest(cmShowDialog, secondSource.id, 0, 0))
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 7 || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(31).Data) {
		t.Fatalf("second spring did not start its movie interaction: quest=%+v task=%v movie=%x", p.quest(refreshingSpringsQuestID), secondSource.useTask != nil, packets.last(smPlayMovie))
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 8 || s.countItems(p, springProofOneItemID) != 0 {
		t.Fatalf("second spring did not consume its proof and advance: quest=%+v items=%d", p.quest(refreshingSpringsQuestID), s.countItems(p, springProofOneItemID))
	}
	selectDialog(guideFive, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideFive.id, 3057, refreshingSpringsQuestID).Data) {
		t.Fatalf("fifth spring-guide page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideFive, 10006)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 9 || s.countItems(p, springProofTwoItemID) != 1 {
		t.Fatalf("second proof was not issued: quest=%+v items=%d", p.quest(refreshingSpringsQuestID), s.countItems(p, springProofTwoItemID))
	}
	selectDialog(guideSix, 10007)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 10 {
		t.Fatalf("guide did not unlock the final spring: %+v", p.quest(refreshingSpringsQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, thirdSource.id, 0, 0))
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 10 {
		t.Fatalf("third spring did not start its interaction: quest=%+v task=%v", p.quest(refreshingSpringsQuestID), thirdSource.useTask != nil)
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(refreshingSpringsQuestID).Vars, 0) != 11 || s.countItems(p, springProofTwoItemID) != 0 {
		t.Fatalf("third spring did not consume its proof and advance: quest=%+v items=%d", p.quest(refreshingSpringsQuestID), s.countItems(p, springProofTwoItemID))
	}
	selectDialog(guideSix, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideSix.id, 3739, refreshingSpringsQuestID).Data) {
		t.Fatalf("final report page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideSix, 10007)
	if p.quest(refreshingSpringsQuestID).Status != "REWARD" {
		t.Fatalf("final report did not open reward: %+v", p.quest(refreshingSpringsQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, valerius.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(valerius.id, 5, refreshingSpringsQuestID).Data) {
		t.Fatalf("Valerius reward page = %x", packets.last(smDialogWindow))
	}
	selectDialog(valerius, 14)
	if quest := p.quest(refreshingSpringsQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("selectable reward did not complete quest: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[6].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}
