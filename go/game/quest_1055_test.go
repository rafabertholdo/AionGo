package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestEternalRestLevelUpCollectionObjectAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[eternalRestQuestID], d.Quests[eternalRestQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != eternalRestStartNPCID || script.EndNPC != eternalRestStartNPCID || template.Race != "ELYOS" || template.MinLevel != 34 || template.NameID != 2204209 ||
		len(template.CollectItems) != 4 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 1754700 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Eternal Rest metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{eternalRestStartNPCID, eternalRestMiddleNPCID, eternalRestFirstNPCID, eternalRestSecondNPCID, eternalRestThirdNPCID, eternalRestFourthNPCID, eternalRestObjectID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC/object %d", npcID)
		}
	}
	for _, npcID := range []int32{eternalRestFirstNPCID, eternalRestSecondNPCID, eternalRestThirdNPCID, eternalRestFourthNPCID} {
		found := false
		for _, registered := range d.QuestDropsByNPC[npcID] {
			found = found || registered.ID == eternalRestQuestID
		}
		if !found {
			t.Fatalf("quest drop index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 33
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: eternalRestQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.eternalRestLevelUp() {
		t.Fatal("quest unlocked below level 34")
	}
	p.level = 34
	p.Exp = d.ExpStart(p.level)
	if c.eternalRestLevelUp() {
		t.Fatal("quest unlocked before quest 1500 was complete")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.eternalRestLevelUp() || p.quest(eternalRestQuestID).Status != "START" || c.eternalRestLevelUp() {
		t.Fatalf("quest did not unlock exactly once after level and prerequisite: %+v", p.quest(eternalRestQuestID))
	}

	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, eternalRestQuestID))
	}
	start := npc(eternalRestStartNPCID, 0x31130)
	middle := npc(eternalRestMiddleNPCID, 0x31131)
	first := npc(eternalRestFirstNPCID, 0x31132)
	second := npc(eternalRestSecondNPCID, 0x31133)
	third := npc(eternalRestThirdNPCID, 0x31134)
	fourth := npc(eternalRestFourthNPCID, 0x31135)
	urn := npc(eternalRestObjectID, 0x31136)

	selectDialog(start, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, eternalRestQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(start, 10000)
	if questVar(p.quest(eternalRestQuestID).Vars, 0) != 1 {
		t.Fatalf("start NPC did not advance to var one: %+v", p.quest(eternalRestQuestID))
	}
	selectDialog(start, 25) // Java falls through from case 25 to 10000 and 10001 at var one.
	if questVar(p.quest(eternalRestQuestID).Vars, 0) != 2 {
		t.Fatalf("var-one case 25 fallthrough did not advance: %+v", p.quest(eternalRestQuestID))
	}
	selectDialog(middle, 33)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 10001, eternalRestQuestID).Data) || questVar(p.quest(eternalRestQuestID).Vars, 0) != 2 {
		t.Fatalf("incomplete collection did not remain at var two: quest=%+v dialog=%x", p.quest(eternalRestQuestID), packets.last(smDialogWindow))
	}

	for _, tc := range []struct {
		o      *object
		itemID int32
		page   uint16
	}{{first, eternalRestFirstItemID, 1694}, {second, eternalRestSecondItemID, 1781}, {third, eternalRestThirdItemID, 1864}, {fourth, eternalRestFourthItemID, 1949}} {
		selectDialog(tc.o, 25)
		if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(tc.o.id, tc.page, eternalRestQuestID).Data) {
			t.Fatalf("NPC %d item page = %x", tc.o.npc.ID, packets.last(smDialogWindow))
		}
		selectDialog(tc.o, 10002)
		if s.countItems(p, tc.itemID) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(tc.o.id, 10, 0).Data) {
			t.Fatalf("NPC %d did not grant its item: count=%d dialog=%x", tc.o.npc.ID, s.countItems(p, tc.itemID), packets.last(smDialogWindow))
		}
		selectDialog(tc.o, 10002)
		if s.countItems(p, tc.itemID) != 1 {
			t.Fatalf("NPC %d duplicated its item", tc.o.npc.ID)
		}
	}

	selectDialog(middle, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 1693, eternalRestQuestID).Data) {
		t.Fatalf("collection page = %x", packets.last(smDialogWindow))
	}
	selectDialog(middle, 33)
	if questVar(p.quest(eternalRestQuestID).Vars, 0) != 3 || s.countItems(p, eternalRestOfferingItemID) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 10000, eternalRestQuestID).Data) {
		t.Fatalf("complete collection did not grant the offering: quest=%+v item=%d dialog=%x", p.quest(eternalRestQuestID), s.countItems(p, eternalRestOfferingItemID), packets.last(smDialogWindow))
	}
	for _, itemID := range []int32{eternalRestFirstItemID, eternalRestSecondItemID, eternalRestThirdItemID, eternalRestFourthItemID} {
		if count := s.countItems(p, itemID); count != 0 {
			t.Fatalf("collection item %d was not consumed: count=%d", itemID, count)
		}
	}

	p.seen[urn.id] = urn
	p.targetID = urn.id
	c.customQuestDialogID(urn, script, -1)
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, urn.id, 1).Data) || questVar(p.quest(eternalRestQuestID).Vars, 0) != 3 {
		t.Fatalf("offering interaction did not start without premature progress: quest=%+v use=%x", p.quest(eternalRestQuestID), packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, urn.id, 0).Data) || questVar(p.quest(eternalRestQuestID).Vars, 0) != 4 || s.countItems(p, eternalRestOfferingItemID) != 0 {
		t.Fatalf("offering interaction did not finish: quest=%+v item=%d use=%x", p.quest(eternalRestQuestID), s.countItems(p, eternalRestOfferingItemID), packets.last(smUseObject))
	}
	selectDialog(middle, 10255)
	if p.quest(eternalRestQuestID).Status != "REWARD" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 10, 0).Data) {
		t.Fatalf("middle NPC did not open reward state: quest=%+v dialog=%x", p.quest(eternalRestQuestID), packets.last(smDialogWindow))
	}
	c.showDialog(dialogRequest(cmShowDialog, start.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 5, eternalRestQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	selectDialog(start, 8)
	if quest := p.quest(eternalRestQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || p.Exp-beforeExperience != 1754700 {
		t.Fatalf("selectable reward failed: quest=%+v reward=%d experience=%d", quest, s.countItems(p, template.Rewards[0].SelectableItems[0].ID), p.Exp-beforeExperience)
	}
}
