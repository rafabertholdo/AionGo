package game

import (
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSimpleThreeNPCQuestCatalog(t *testing.T) {
	d := staticDataOrSkip(t)
	ids := []int32{1131, 1218, 1314, 1324, 1452, 1527, 1528, 1609, 1628, 1909, 2222, 2651, 2693}
	for _, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			script, template := d.QuestScripts[id], d.Quests[id]
			if script == nil || template == nil || script.StartNPC == 0 || script.MiddleNPC == 0 || script.EndNPC == 0 || !script.NPCStart {
				t.Fatal("quest registration missing")
			}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
			if p.Race == "" {
				p.Race = "ELYOS"
			}
			p.Class = "GLADIATOR"
			p.level = max(template.MinLevel, 11)
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			for _, required := range template.FinishedQuestConditions {
				p.quests = append(p.quests, store.Quest{ID: required, Status: "COMPLETE", CompleteCount: 1})
			}
			start := questCatalogNPC(s, p, script.StartNPC, 0x31001)
			middle := questCatalogNPC(s, p, script.MiddleNPC, 0x31002)
			end := questCatalogNPC(s, p, script.EndNPC, 0x31003)
			c.simpleThreeNPCQuestDialog(middle, script, 10000)
			if p.quest(id) != nil {
				t.Fatal("middle NPC started quest")
			}
			c.simpleThreeNPCQuestDialog(start, script, 25)
			c.simpleThreeNPCQuestDialog(start, script, 1002)
			if q := p.quest(id); q == nil || q.Status != "START" || q.Vars != 0 {
				t.Fatalf("quest did not start: %+v", q)
			}
			c.simpleThreeNPCQuestDialog(middle, script, 25)
			c.simpleThreeNPCQuestDialog(middle, script, 10000)
			if q := p.quest(id); q.Vars != 1 || q.Status != "START" {
				t.Fatalf("middle NPC progress = %+v", q)
			}
			c.simpleThreeNPCQuestDialog(middle, script, 10000)
			if p.quest(id).Vars != 1 {
				t.Fatal("middle NPC advanced twice")
			}
			c.simpleThreeNPCQuestDialog(end, script, 25)
			c.simpleThreeNPCQuestDialog(end, script, 1009)
			wantVars := int32(2)
			if id == 2651 {
				wantVars = 3
			}
			if q := p.quest(id); q.Status != "REWARD" || q.Vars != wantVars {
				t.Fatalf("final NPC progress = %+v", q)
			}
			before := p.Exp
			rewardDialog := uint16(17)
			if len(template.Rewards[0].SelectableItems) > 0 || len(template.ClassRewards(p.Class)) > 0 {
				rewardDialog = 8
			}
			c.simpleThreeNPCQuestDialog(end, script, rewardDialog)
			if q := p.quest(id); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-before != template.Rewards[0].Experience {
				t.Fatalf("completion = %+v exp=%d", q, p.Exp-before)
			}
			c.simpleThreeNPCQuestDialog(end, script, rewardDialog)
			if p.quest(id).CompleteCount != 1 {
				t.Fatal("quest completed twice")
			}
		})
	}
}
