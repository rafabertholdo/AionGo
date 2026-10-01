package game

import (
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestThreeNPCQuestCatalog(t *testing.T) {
	d := staticDataOrSkip(t)
	ids := []int32{2569, 2553, 2512, 2953, 2928, 2954, 2914, 2773, 4036, 4020, 3091, 3020, 3037, 3081, 2514, 2583, 2965, 2913, 2917, 2767, 4001, 3083, 3076, 3035, 3023, 2611, 2501, 2515, 2523, 2539, 2692, 2912, 4052, 4101}
	for _, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			script, template := d.QuestScripts[id], d.Quests[id]
			if script == nil || template == nil || script.MiddleNPC == 0 || !script.NPCStart {
				t.Fatal("quest registration missing")
			}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
			if p.Race == "" {
				p.Race = "ASMODIANS"
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
			start := questCatalogNPC(s, p, script.StartNPC, 0x30001)
			middle := questCatalogNPC(s, p, script.MiddleNPC, 0x30002)
			c.threeNPCQuestDialog(start, script, 25)
			c.threeNPCQuestDialog(start, script, 1002)
			if q := p.quest(id); q == nil || q.Status != "START" {
				t.Fatalf("quest did not start: %+v", q)
			}
			c.threeNPCQuestDialog(middle, script, 25)
			c.threeNPCQuestDialog(middle, script, 10000)
			if p.quest(id).Vars != 1 {
				t.Fatal("middle NPC did not advance")
			}
			if script.MiddleNPC2 != 0 {
				second := questCatalogNPC(s, p, script.MiddleNPC2, 0x30004)
				c.threeNPCQuestDialog(second, script, 25)
				c.threeNPCQuestDialog(second, script, 10001)
				if p.quest(id).Vars != 2 {
					t.Fatal("second middle NPC did not advance")
				}
			}
			if script.MiddleNPC3 != 0 {
				third := questCatalogNPC(s, p, script.MiddleNPC3, 0x30005)
				c.threeNPCQuestDialog(third, script, 25)
				c.threeNPCQuestDialog(third, script, 10002)
				if p.quest(id).Vars != 3 {
					t.Fatal("third middle NPC did not advance")
				}
			}
			end := start
			if script.FinalNPC != 0 {
				end = questCatalogNPC(s, p, script.FinalNPC, 0x30003)
			}
			c.threeNPCQuestDialog(end, script, 1009)
			wantVars := int32(1)
			if script.MiddleNPC2 != 0 {
				wantVars = 2
			}
			if script.MiddleNPC3 != 0 {
				wantVars = 3
			}
			if script.FinalNPC != 0 {
				wantVars = 3
			}
			if q := p.quest(id); q.Status != "REWARD" || q.Vars != wantVars {
				t.Fatalf("reward transition = %+v", q)
			}
			before := p.Exp
			c.threeNPCQuestDialog(end, script, 17)
			if q := p.quest(id); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-before != template.Rewards[0].Experience {
				t.Fatalf("completion = %+v exp=%d", q, p.Exp-before)
			}
			c.threeNPCQuestDialog(end, script, 17)
			if p.quest(id).CompleteCount != 1 {
				t.Fatal("duplicate completion")
			}
		})
	}
}
