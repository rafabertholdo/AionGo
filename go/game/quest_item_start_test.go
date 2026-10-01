package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestCustomItemStartedQuests(t *testing.T) {
	for _, tc := range []struct {
		id, itemID, middleNPC, endNPC int32
		race                          string
		exp, kinah                    int64
	}{
		{1107, 182200501, 0, 203075, "ELYOS", 420, 400},
		{2107, 182203107, 203516, 203512, "ASMODIANS", 750, 130},
	} {
		t.Run(tc.race, func(t *testing.T) {
			d := staticDataOrSkip(t)
			s := testServer(d)
			p := wrathchild(s)
			p.Race, p.level, p.Exp = tc.race, 3, d.ExpStart(3)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			script := d.QuestScripts[tc.id]
			if script == nil || len(d.QuestItemUses[tc.itemID]) != 1 {
				t.Fatal("item use index missing")
			}
			if c.itemStartedQuestDialog(script, 1002) || p.quest(tc.id) != nil {
				t.Fatal("quest started without its item")
			}
			if !s.addItem(p, tc.itemID, 1) || !c.questStartItemUse(p.cube[0], script) || packets.last(smDialogWindow) == nil {
				t.Fatal("item did not open quest")
			}
			if !c.itemStartedQuestDialog(script, 1002) || p.quest(tc.id) == nil || p.quest(tc.id).Status != "START" {
				t.Fatal("item did not start quest")
			}
			if tc.middleNPC != 0 {
				middle := questCatalogNPC(s, p, tc.middleNPC, 0x30001)
				c.returnToSenderDialog(middle, script, 25)
				c.returnToSenderDialog(middle, script, 10000)
				if p.quest(tc.id).Vars != 1 {
					t.Fatal("middle NPC did not advance")
				}
			}
			end := questCatalogNPC(s, p, tc.endNPC, 0x30002)
			if tc.id == 1107 {
				c.lostAxeDialog(end, script, 25)
				c.lostAxeDialog(end, script, 1009)
			} else {
				c.returnToSenderDialog(end, script, 25)
				c.returnToSenderDialog(end, script, 1009)
			}
			if p.quest(tc.id).Status != "REWARD" || s.countItems(p, tc.itemID) != 0 {
				t.Fatal("item turn-in")
			}
			before := p.Exp
			if tc.id == 1107 {
				c.lostAxeDialog(end, script, 17)
				c.lostAxeDialog(end, script, 17)
			} else {
				c.returnToSenderDialog(end, script, 17)
				c.returnToSenderDialog(end, script, 17)
			}
			if p.quest(tc.id).Status != "COMPLETE" || p.Exp-before != tc.exp || p.kinah.Count != tc.kinah || p.quest(tc.id).CompleteCount != 1 {
				t.Fatalf("reward = %+v, exp=%d, kinah=%d", p.quest(tc.id), p.Exp-before, p.kinah.Count)
			}
		})
	}
}
