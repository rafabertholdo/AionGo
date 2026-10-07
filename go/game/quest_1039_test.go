package game

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSomethingInTheWaterBottleKillsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		script, template := d.QuestScripts[somethingInTheWaterQuestID], d.Quests[somethingInTheWaterQuestID]
		if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 29 || template.NameID != 2204117 ||
			len(template.FinishedQuestConditions) != 2 || len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: somethingInTheWaterFullBottle, Count: 1}) ||
			len(template.Rewards) != 1 || template.Rewards[0].Experience != 633400 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 121000771, Count: 1}) {
			t.Fatalf("unexpected Something in the Water metadata: script=%+v template=%+v", script, template)
		}
		if len(d.QuestItemUses[somethingInTheWaterBottle]) != 1 || d.QuestItemUses[somethingInTheWaterBottle][0].ID != somethingInTheWaterQuestID {
			t.Fatalf("empty bottle use is not registered: %+v", d.QuestItemUses[somethingInTheWaterBottle])
		}
		for _, npcID := range []int32{somethingInTheWaterNPCID, somethingInTheWaterJumentis} {
			if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
				t.Fatalf("quest talk index is missing NPC %d", npcID)
			}
		}
		for _, npcID := range []int32{210946, 210947} {
			found := false
			for _, registered := range d.QuestKills[npcID] {
				found = found || registered.ID == somethingInTheWaterQuestID
			}
			if !found {
				t.Fatalf("kill index is missing NPC %d", npcID)
			}
		}

		s := testServer(d)
		saver := &recordedQuests{}
		s.quests = saver
		p := wrathchild(s)
		p.Race, p.Class, p.level = "ELYOS", "CLERIC", 28
		p.Exp = d.ExpStart(p.level)
		p.cube, p.seen = []*store.Item{}, map[int32]*object{}
		p.quests = []store.Quest{{ID: somethingInTheWaterQuestID, Status: "LOCKED"}, {ID: 1035, Status: "COMPLETE"}, {ID: 1016, Status: "COMPLETE"}}
		packets := &questPackets{}
		c := &conn{s: s, player: p, tap: packets.tap}
		p.conn = c
		s.spawned[p.ID] = p
		if c.somethingInTheWaterLevelUp() {
			t.Fatal("quest unlocked below level 29")
		}
		p.level = 29
		p.quest(1016).Status = "START"
		if c.somethingInTheWaterLevelUp() {
			t.Fatal("quest unlocked without all prerequisite quests complete")
		}
		p.quest(1016).Status = "COMPLETE"
		if !c.somethingInTheWaterLevelUp() || p.quest(somethingInTheWaterQuestID).Status != "START" || c.somethingInTheWaterLevelUp() {
			t.Fatalf("quest did not unlock exactly once: %+v", p.quest(somethingInTheWaterQuestID))
		}

		npc := func(id, objectID int32) *object {
			object := questCatalogNPC(s, p, id, objectID)
			object.npc = d.Npcs[id]
			if object.npc == nil {
				t.Fatalf("NPC template %d is missing", id)
			}
			return object
		}
		selectDialog := func(object *object, dialog uint16) {
			c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, somethingInTheWaterQuestID))
		}
		asclepius := npc(somethingInTheWaterNPCID, 0x31090)
		jumentis := npc(somethingInTheWaterJumentis, 0x31091)
		c.showDialog(dialogRequest(cmShowDialog, asclepius.id, 0, 0))
		selectDialog(asclepius, 25)
		if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(asclepius.id, 1011, somethingInTheWaterQuestID).Data) {
			t.Fatalf("opening quest page = %x", packets.last(smDialogWindow))
		}
		selectDialog(asclepius, 10000)
		if questVar(p.quest(somethingInTheWaterQuestID).Vars, 0) != 1 || s.countItems(p, somethingInTheWaterBottle) != 1 {
			t.Fatalf("Asclepius did not give the empty bottle: quest=%+v bottle=%d", p.quest(somethingInTheWaterQuestID), s.countItems(p, somethingInTheWaterBottle))
		}
		item := p.cubeItem(p.cube[0].UniqueID)
		p.zone = &data.Zone{Name: "PUTRID_MIRE", MapID: 210020000}
		s.visMu.Lock()
		c.somethingInTheWaterItemUse(item)
		s.visMu.Unlock()
		if packets.last(smItemUsageAnimation) != nil {
			t.Fatal("empty bottle started outside the Mystic Spring")
		}
		p.zone = &data.Zone{Name: somethingInTheWaterZone, MapID: 210020000}
		s.visMu.Lock()
		c.somethingInTheWaterItemUse(item)
		if !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0).Data) {
			t.Fatalf("bottle use animation = %x", packets.last(smItemUsageAnimation))
		}
		s.visMu.Unlock()
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if questVar(p.quest(somethingInTheWaterQuestID).Vars, 0) != 2 || s.countItems(p, somethingInTheWaterBottle) != 0 || s.countItems(p, somethingInTheWaterFullBottle) != 1 ||
			!bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0).Data) {
			t.Fatalf("bottle use did not produce the full bottle: quest=%+v empty=%d full=%d animation=%x", p.quest(somethingInTheWaterQuestID), s.countItems(p, somethingInTheWaterBottle), s.countItems(p, somethingInTheWaterFullBottle), packets.last(smItemUsageAnimation))
		}
		selectDialog(jumentis, 25)
		if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(jumentis.id, 1352, somethingInTheWaterQuestID).Data) {
			t.Fatalf("Jumentis page = %x", packets.last(smDialogWindow))
		}
		selectDialog(jumentis, 10001)
		if questVar(p.quest(somethingInTheWaterQuestID).Vars, 0) != 3 || s.countItems(p, somethingInTheWaterFullBottle) != 0 {
			t.Fatalf("Jumentis did not accept the full bottle: quest=%+v full=%d", p.quest(somethingInTheWaterQuestID), s.countItems(p, somethingInTheWaterFullBottle))
		}
		selectDialog(asclepius, 25)
		if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(asclepius.id, 1693, somethingInTheWaterQuestID).Data) {
			t.Fatalf("second Asclepius page = %x", packets.last(smDialogWindow))
		}
		selectDialog(asclepius, 10002)
		if questVar(p.quest(somethingInTheWaterQuestID).Vars, 0) != 4 {
			t.Fatalf("Asclepius did not start the spring investigation: %+v", p.quest(somethingInTheWaterQuestID))
		}
		for range 3 {
			if !c.somethingInTheWaterKill(210946) {
				t.Fatal("scout Vaegir kill did not progress")
			}
		}
		for range 3 {
			if !c.somethingInTheWaterKill(210947) {
				t.Fatal("scout Lepharist kill did not progress")
			}
		}
		quest := p.quest(somethingInTheWaterQuestID)
		if quest.Status != "REWARD" || questVar(quest.Vars, 1) != 3 || questVar(quest.Vars, 2) != 3 {
			t.Fatalf("six kills did not unlock reward: %+v", quest)
		}
		c.showDialog(dialogRequest(cmShowDialog, asclepius.id, 0, 0))
		selectDialog(asclepius, 17)
		if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, 121000771) != 1 {
			t.Fatalf("fixed reward did not complete the quest: quest=%+v reward=%d", quest, s.countItems(p, 121000771))
		}
	})
}
