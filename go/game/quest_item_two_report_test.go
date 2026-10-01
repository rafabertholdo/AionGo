package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestItemTwoReportQuestCatalog(t *testing.T) {
	d := staticDataOrSkip(t)
	ids := []int32{2578, 2846, 2847, 2848, 4053}
	for _, id := range ids {
		script := d.QuestScripts[id]
		if script == nil || script.Kind != data.QuestCustom || script.ItemUseDelay != 3000 || script.MiddleNPC == 0 || script.MiddleNPC2 == 0 || script.EndNPC == 0 || script.ItemID == 0 || script.NPCStart {
			t.Fatalf("quest %d registration = %+v", id, script)
		}
		if len(d.QuestItemUses[script.ItemID]) == 0 {
			t.Fatalf("quest %d item use is not routed", id)
		}
		for _, npcID := range []int32{script.MiddleNPC, script.MiddleNPC2, script.EndNPC} {
			found := false
			for _, indexed := range d.QuestEnds[npcID] {
				if indexed.ID == id {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("quest %d NPC %d is not routed", id, npcID)
			}
		}
		s := testServer(d)
		p := wrathchild(s)
		p.Race = d.Quests[id].Race
		p.Class = "GLADIATOR"
		p.level = d.Quests[id].MinLevel
		p.Exp = d.ExpStart(p.level)
		p.cube = nil
		p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
		p.seen = map[int32]*object{}
		packets := &questPackets{}
		c := &conn{s: s, player: p, tap: packets.tap}
		p.conn = c
		s.spawned[p.ID] = p
		if !s.addItem(p, script.ItemID, 1) {
			t.Fatalf("quest %d start item missing", id)
		}
		if !c.itemStartedQuestDialog(script, 1002) || p.quest(id) == nil || p.quest(id).Status != "START" {
			t.Fatalf("quest %d did not start", id)
		}
		first := questCatalogNPC(s, p, script.MiddleNPC, 0x33000+id)
		second := questCatalogNPC(s, p, script.MiddleNPC2, 0x34000+id)
		end := questCatalogNPC(s, p, script.EndNPC, 0x35000+id)
		c.itemTwoReportQuestDialog(second, script, 10001)
		if p.quest(id).Vars != 0 {
			t.Fatalf("quest %d accepted out-of-order second report", id)
		}
		c.itemTwoReportQuestDialog(first, script, 25)
		if packets.last(smDialogWindow) == nil {
			t.Fatalf("quest %d first report page missing", id)
		}
		c.itemTwoReportQuestDialog(first, script, 10000)
		if p.quest(id).Vars != 1 {
			t.Fatalf("quest %d first report vars = %d", id, p.quest(id).Vars)
		}
		c.itemTwoReportQuestDialog(second, script, 25)
		c.itemTwoReportQuestDialog(second, script, 10001)
		if p.quest(id).Vars != 2 {
			t.Fatalf("quest %d second report vars = %d", id, p.quest(id).Vars)
		}
		c.itemTwoReportQuestDialog(end, script, 25)
		c.itemTwoReportQuestDialog(end, script, 1009)
		if p.quest(id).Status != "REWARD" || p.quest(id).Vars != 1 {
			t.Fatalf("quest %d final turn-in state = %+v", id, p.quest(id))
		}
		remaining := int64(0)
		if id == 2846 {
			remaining = 1 // Java removes 182207048 instead of the start item.
		}
		if got := s.countItems(p, script.ItemID); got != remaining {
			t.Fatalf("quest %d start item remaining = %d, want %d", id, got, remaining)
		}
		beforeExp := p.Exp
		c.itemTwoReportQuestDialog(end, script, 17)
		if p.quest(id).Status != "COMPLETE" || p.Exp-beforeExp != d.Quests[id].Rewards[0].Experience {
			t.Fatalf("quest %d completion = %+v exp=%d", id, p.quest(id), p.Exp-beforeExp)
		}
	}
}
