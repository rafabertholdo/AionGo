package game

import (
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestDelayedItemQuestCatalog(t *testing.T) {
	d := staticDataOrSkip(t)
	ids := []int32{1966, 1967, 1968, 1969, 1970, 2966, 2967, 2968, 2969, 2970, 2971, 1355, 2316, 2216, 2228, 1182, 3049}
	for index, id := range ids {
		script := d.QuestScripts[id]
		if script == nil || script.ItemUseDelay != 3000 || len(d.QuestItemUses[script.ItemID]) != 1 {
			t.Fatalf("quest %d registration = %+v", id, script)
		}
		for _, offered := range d.QuestStarts[script.EndNPC] {
			if offered.ID == id {
				t.Fatalf("item quest %d advertises an NPC start marker", id)
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
			t.Fatalf("quest %d item missing", id)
		}
		if index == 0 {
			if !c.delayedItemQuestUse(p.cube[0], script) {
				t.Fatal("delayed item use rejected")
			}
			time.Sleep(3200 * time.Millisecond)
			if packets.last(smDialogWindow) == nil {
				t.Fatal("delayed item omitted quest dialog")
			}
		}
		if !c.itemStartedQuestDialog(script, 1002) || p.quest(id) == nil || p.quest(id).Status != "START" {
			t.Fatalf("quest %d did not start", id)
		}
		npc := questCatalogNPC(s, p, script.EndNPC, 0x30000+id)
		c.delayedItemQuestDialog(npc, script, 25)
		if packets.last(smDialogWindow) == nil {
			t.Fatalf("quest %d NPC page missing", id)
		}
		c.delayedItemQuestDialog(npc, script, 1009)
		if p.quest(id).Status != "REWARD" || s.countItems(p, script.ItemID) != 0 {
			t.Fatalf("quest %d item turn-in", id)
		}
		beforeExp, beforeAP := p.Exp, p.abyss.AP
		reward := d.Quests[id].Rewards[0]
		c.delayedItemQuestDialog(npc, script, 17)
		if p.quest(id).Status != "COMPLETE" || p.Exp-beforeExp != reward.Experience || p.kinah.Count != reward.Kinah || p.abyss.AP-beforeAP != reward.AbyssPoints {
			t.Fatalf("quest %d reward = %+v exp=%d kinah=%d AP=%d", id, p.quest(id), p.Exp-beforeExp, p.kinah.Count, p.abyss.AP-beforeAP)
		}
	}
}
