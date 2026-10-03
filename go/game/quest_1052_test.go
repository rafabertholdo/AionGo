package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestRootOfTheRotLevelUpCollectionAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[rootOfRotQuestID], d.Quests[rootOfRotQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != rootOfRotStartNPC || script.EndNPC != rootOfRotEndNPC || template.Race != "ELYOS" || template.MinLevel != 30 || template.NameID != 2204203 || len(template.CollectItems) != 2 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 633400 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0].ID != 125001838 {
		t.Fatalf("unexpected Root of the Rot metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{rootOfRotStartNPC, rootOfRotMiddleNPC, rootOfRotEndNPC} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	for _, tc := range []struct{ npcID, itemID int32 }{{211078, rootOfRotFirstItem}, {211087, rootOfRotFirstItem}, {211079, rootOfRotSecondItem}, {211089, rootOfRotSecondItem}} {
		found := false
		for _, drop := range d.QuestDropsByNPC[tc.npcID] {
			if drop.ID == rootOfRotQuestID {
				found = true
			}
		}
		if !found {
			t.Errorf("quest drop index is missing NPC %d for item %d", tc.npcID, tc.itemID)
		}
	}
	for _, itemID := range []int32{rootOfRotFirstItem, rootOfRotSecondItem} {
		found := false
		for _, registered := range d.QuestItemUses[itemID] {
			found = found || registered.ID == rootOfRotQuestID
		}
		if !found {
			t.Errorf("quest item-use index is missing item %d", itemID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 29
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.WorldID, p.X, p.Y, p.Z = 210060000, 2500, 780, 409
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: rootOfRotQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	s.addPlayerCell(p)
	if c.rootOfRotLevelUp() {
		t.Fatal("quest unlocked below its minimum level")
	}
	p.level = 30
	p.Exp = d.ExpStart(p.level)
	if c.rootOfRotLevelUp() {
		t.Fatal("quest unlocked before quest 1500 was complete")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.rootOfRotLevelUp() || p.quest(rootOfRotQuestID).Status != "START" || c.rootOfRotLevelUp() {
		t.Fatalf("quest did not unlock exactly once after level 30 and quest 1500: %+v", p.quest(rootOfRotQuestID))
	}

	npc := func(id, objectID int32) *object {
		object := questCatalogNPC(s, p, id, objectID)
		object.npc = d.Npcs[id]
		if object.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(object)
		return object
	}
	selectDialog := func(object *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, rootOfRotQuestID))
	}
	startNPC := npc(rootOfRotStartNPC, 0x31120)
	middleNPC := npc(rootOfRotMiddleNPC, 0x31121)
	endNPC := npc(rootOfRotEndNPC, 0x31122)
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, rootOfRotQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10000)
	if questVar(p.quest(rootOfRotQuestID).Vars, 0) != 1 {
		t.Fatalf("opening dialog did not advance to one: %+v", p.quest(rootOfRotQuestID))
	}
	selectDialog(startNPC, 33)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 10001, rootOfRotQuestID).Data) {
		t.Fatalf("missing items page = %x", packets.last(smDialogWindow))
	}
	if !s.addItem(p, rootOfRotFirstItem, 3) || !s.addItem(p, rootOfRotSecondItem, 3) || s.countItems(p, rootOfRotFirstItem) != 3 || s.countItems(p, rootOfRotSecondItem) != 3 {
		t.Fatalf("could not add collection items: %d/%d", s.countItems(p, rootOfRotFirstItem), s.countItems(p, rootOfRotSecondItem))
	}
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1352, rootOfRotQuestID).Data) {
		t.Fatalf("collection stage reminder page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 33)
	if questVar(p.quest(rootOfRotQuestID).Vars, 0) != 2 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 10000, rootOfRotQuestID).Data) {
		t.Fatalf("collection did not advance the quest: quest=%+v items=%d/%d requirements=%+v page=%x", p.quest(rootOfRotQuestID), s.countItems(p, rootOfRotFirstItem), s.countItems(p, rootOfRotSecondItem), template.CollectItems, packets.last(smDialogWindow))
	}
	selectDialog(middleNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middleNPC.id, 1693, rootOfRotQuestID).Data) {
		t.Fatalf("middle NPC page = %x", packets.last(smDialogWindow))
	}
	selectDialog(middleNPC, 10255)
	if p.quest(rootOfRotQuestID).Status != "REWARD" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middleNPC.id, 10, 0).Data) {
		t.Fatalf("middle NPC did not open reward state: quest=%+v page=%x", p.quest(rootOfRotQuestID), packets.last(smDialogWindow))
	}
	c.showDialog(dialogRequest(cmShowDialog, endNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, rootOfRotQuestID).Data) || s.countItems(p, rootOfRotFirstItem) != 2 || s.countItems(p, rootOfRotSecondItem) != 2 {
		t.Fatalf("end click did not show reward and remove one of each item: page=%x items=%d/%d", packets.last(smDialogWindow), s.countItems(p, rootOfRotFirstItem), s.countItems(p, rootOfRotSecondItem))
	}
	selectDialog(endNPC, 17) // Java: 8-16 index a selectable reward this quest does not have (and throw)
	if p.quest(rootOfRotQuestID).Status != "COMPLETE" || p.quest(rootOfRotQuestID).CompleteCount != 1 || s.countItems(p, rootOfRotFirstItem) != 1 || s.countItems(p, rootOfRotSecondItem) != 1 || s.countItems(p, 125001838) != 1 || p.Exp < d.ExpStart(30)+633400 {
		t.Fatalf("quest completion/rewards mismatch: quest=%+v exp=%d items=%d/%d reward=%d", p.quest(rootOfRotQuestID), p.Exp, s.countItems(p, rootOfRotFirstItem), s.countItems(p, rootOfRotSecondItem), s.countItems(p, 125001838))
	}
}
