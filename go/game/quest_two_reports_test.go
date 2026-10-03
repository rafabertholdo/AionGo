package game

import (
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestTwoReportsQuestCatalog(t *testing.T) {
	d := staticDataOrSkip(t)
	cases := []struct {
		id, start, first, second, final, finalVar int32
	}{
		{1553, 203786, 730051, 204500, 204584, 2},
		{1620, 204519, 790000, 730001, 203125, 3},
		{1578, 730025, 730024, 204560, 204579, 3},
		{1605, 204576, 204530, 204501, 204577, 3},
		{1483, 798126, 203940, 203944, 798127, 3},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			template := d.Quests[tc.id]
			if template == nil {
				t.Fatal("quest metadata missing")
			}
			script := &data.QuestScript{ID: tc.id, StartNPC: tc.start, EndNPC: tc.final, NPCStart: true}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
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
			start := questCatalogNPC(s, p, tc.start, 0x31001)
			first := questCatalogNPC(s, p, tc.first, 0x31002)
			second := questCatalogNPC(s, p, tc.second, 0x31003)
			end := questCatalogNPC(s, p, tc.final, 0x31004)
			c.twoReportsQuestDialog(first, script, 10000)
			if p.quest(tc.id) != nil {
				t.Fatal("accepted report before quest start")
			}
			c.twoReportsQuestDialog(start, script, 25)
			c.twoReportsQuestDialog(start, script, 1002)
			if q := p.quest(tc.id); q == nil || q.Status != "START" {
				t.Fatalf("quest did not start: %+v", q)
			}
			c.twoReportsQuestDialog(second, script, 10000)
			if p.quest(tc.id).Vars != 0 {
				t.Fatal("second report accepted out of order")
			}
			c.twoReportsQuestDialog(first, script, 25)
			c.twoReportsQuestDialog(first, script, 10000)
			c.twoReportsQuestDialog(first, script, 10000)
			if p.quest(tc.id).Vars != 1 {
				t.Fatal("first report did not advance exactly once")
			}
			c.twoReportsQuestDialog(second, script, 25)
			c.twoReportsQuestDialog(second, script, 10000)
			if p.quest(tc.id).Vars != 2 {
				t.Fatal("second report did not advance")
			}
			c.twoReportsQuestDialog(end, script, 25)
			c.twoReportsQuestDialog(end, script, 1009)
			if q := p.quest(tc.id); q.Status != "REWARD" || q.Vars != tc.finalVar {
				t.Fatalf("reward transition = %+v", q)
			}
			before := p.Exp
			rewardDialog := int32(17)
			if len(template.Rewards[0].SelectableItems) != 0 {
				rewardDialog = 8
			}
			c.twoReportsQuestDialog(end, script, rewardDialog)
			if q := p.quest(tc.id); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-before != template.Rewards[0].Experience {
				t.Fatalf("completion = %+v exp=%d", q, p.Exp-before)
			}
			c.twoReportsQuestDialog(end, script, rewardDialog)
			if p.quest(tc.id).CompleteCount != 1 {
				t.Fatal("duplicate completion")
			}
		})
	}
}
