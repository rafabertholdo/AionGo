package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestKeeperOfTheKaidanKeyUnlockKillItemAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[keeperKaidanKeyQuestID], d.Quests[keeperKaidanKeyQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 35 || template.NameID != 2204123 ||
		len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1040 || len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: keeperKaidanKeyItemID, Count: 1}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 2108600 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 122000895, Count: 1}) {
		t.Fatalf("unexpected Keeper of the Kaidan Key metadata: script=%+v template=%+v", script, template)
	}
	if len(d.QuestItemUses[keeperKaidanKeyItemID]) != 1 || d.QuestItemUses[keeperKaidanKeyItemID][0].ID != keeperKaidanKeyQuestID {
		t.Fatalf("Kaidan key use is not registered: %+v", d.QuestItemUses[keeperKaidanKeyItemID])
	}
	for _, npcID := range []int32{212029, 212033} {
		found := false
		for _, registered := range d.QuestKills[npcID] {
			found = found || registered.ID == keeperKaidanKeyQuestID
		}
		if !found {
			t.Fatalf("kill index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 34
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.quests = []store.Quest{{ID: keeperKaidanKeyQuestID, Status: "LOCKED"}, {ID: 1040, Status: "COMPLETE"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.keeperKaidanKeyLevelUp() {
		t.Fatal("quest unlocked below level 35")
	}
	p.level = 35
	p.quest(1040).Status = "START"
	if c.keeperKaidanKeyLevelUp() {
		t.Fatal("quest unlocked before quest 1040 was complete")
	}
	p.quest(1040).Status = "COMPLETE"
	if !c.keeperKaidanKeyLevelUp() || p.quest(keeperKaidanKeyQuestID).Status != "START" || c.keeperKaidanKeyLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(keeperKaidanKeyQuestID))
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
		c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, keeperKaidanKeyQuestID))
	}
	startNPC := npc(keeperKaidanKeyStartNPC, 0x31107)
	endNPC := npc(keeperKaidanKeyEndNPC, 0x31108)
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, keeperKaidanKeyQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 1012)
	if !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(185).Data) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1012, keeperKaidanKeyQuestID).Data) {
		t.Fatalf("start NPC movie did not echo its page: movie=%x dialog=%x", packets.last(smPlayMovie), packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10000)
	if questVar(p.quest(keeperKaidanKeyQuestID).Vars, 0) != 1 {
		t.Fatalf("start conversation did not advance: %+v", p.quest(keeperKaidanKeyQuestID))
	}
	selectDialog(endNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 1438, keeperKaidanKeyQuestID).Data) {
		t.Fatalf("Java's mismatched stage fallthrough should request the missing key, got %x", packets.last(smDialogWindow))
	}
	if !c.keeperKaidanKeyKill(212029) || questVar(p.quest(keeperKaidanKeyQuestID).Vars, 0) != 2 || c.keeperKaidanKeyKill(212033) {
		t.Fatalf("Kaidan kill did not set the key stage once: %+v", p.quest(keeperKaidanKeyQuestID))
	}
	quest := p.quest(keeperKaidanKeyQuestID)
	if !c.customQuestProgress(keeperKaidanKeyQuestID, setQuestVar(quest.Vars, 0, 1), "") || !c.keeperKaidanKeyKill(212033) || questVar(quest.Vars, 0) != 2 {
		t.Fatalf("Crack Kaidan Captain kill did not set the key stage: %+v", quest)
	}
	if !s.addItem(p, keeperKaidanKeyItemID, 1) {
		t.Fatal("could not add the Kaidan key for the item-use route")
	}
	item := p.cube[0]
	use := wire.Packet(cmUseItem)
	use.D(item.UniqueID)
	use.C(0)
	c.useItem(wire.NewReader(use.Data[1:]))
	if questVar(p.quest(keeperKaidanKeyQuestID).Vars, 0) != 2 || s.countItems(p, keeperKaidanKeyItemID) != 1 || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, keeperKaidanKeyItemID, 20, 1, 0).Data) {
		t.Fatalf("key use did not preserve the item and set its stage: quest=%+v item=%d animation=%x", p.quest(keeperKaidanKeyQuestID), s.countItems(p, keeperKaidanKeyItemID), packets.last(smItemUsageAnimation))
	}
	selectDialog(endNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 1352, keeperKaidanKeyQuestID).Data) {
		t.Fatalf("end NPC page = %x", packets.last(smDialogWindow))
	}
	selectDialog(endNPC, 33)
	if p.quest(keeperKaidanKeyQuestID).Status != "REWARD" || s.countItems(p, keeperKaidanKeyItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, keeperKaidanKeyQuestID).Data) {
		t.Fatalf("key turn-in did not open reward: quest=%+v key=%d page=%x", p.quest(keeperKaidanKeyQuestID), s.countItems(p, keeperKaidanKeyItemID), packets.last(smDialogWindow))
	}
	c.showDialog(dialogRequest(cmShowDialog, endNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 10002, keeperKaidanKeyQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	selectDialog(endNPC, 17)
	if quest := p.quest(keeperKaidanKeyQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, 122000895) != 1 {
		t.Fatalf("fixed reward did not complete the quest: quest=%+v reward=%d", quest, s.countItems(p, 122000895))
	}
}
