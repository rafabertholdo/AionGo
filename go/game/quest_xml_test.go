package game

import (
	"fmt"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAllSpecializedXMLQuestsComplete(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		id, middleNPC int32
	}{
		{1115, 203072},
		{1127, 700001},
		{1963, 203851},
		{1964, 203776},
		{3913, 203752},
	} {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			script := d.QuestScripts[tc.id]
			template := d.Quests[tc.id]
			if script == nil || script.Kind != data.QuestXML || template == nil || len(script.TalkEvents) == 0 {
				t.Fatalf("missing XML quest data: %+v %+v", script, template)
			}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
			p.level = max(1, template.MinLevel)
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.spawned = true
			for _, prerequisite := range template.FinishedQuestConditions {
				p.quests = append(p.quests, store.Quest{ID: prerequisite, Status: "COMPLETE", CompleteCount: 1})
			}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			start := questCatalogNPC(s, p, script.StartNPC, 0x30001)
			middle := questCatalogNPC(s, p, tc.middleNPC, 0x30002)
			end := questCatalogNPC(s, p, script.EndNPC, 0x30003)
			if script.StartNPC == script.EndNPC {
				end = start
			}
			selectQuestDialog(c, start, 1002, tc.id)
			if q := p.quest(tc.id); q == nil || q.Status != "START" {
				t.Fatalf("XML quest did not start: %+v", q)
			}
			if tc.id == 1127 {
				c.showDialog(dialogRequest(cmShowDialog, middle.id, 0, 0))
				time.Sleep(3200 * time.Millisecond)
				if p.quest(tc.id).Vars != 1 || s.countItems(p, 182200215) != 1 || packets.last(smUseObject) == nil {
					t.Fatalf("quest object use failed: %+v cube=%+v", p.quest(tc.id), p.cube)
				}
				selectQuestDialog(c, end, 33, tc.id)
			} else {
				selectQuestDialog(c, middle, 25, tc.id)
				if got := packets.last(smDialogWindow); got == nil {
					t.Fatal("middle NPC did not open dialog")
				}
				selectQuestDialog(c, middle, 10000, tc.id)
				if tc.id == 1115 {
					if p.quest(tc.id).Vars != 1 {
						t.Fatal("first Elim dialog did not advance quest variable")
					}
					selectQuestDialog(c, end, 25, tc.id)
					selectQuestDialog(c, end, 1009, tc.id)
				}
			}
			if q := p.quest(tc.id); q.Status != "REWARD" {
				t.Fatalf("XML quest did not reach reward: %+v", q)
			}
			if tc.id == 1127 && s.countItems(p, 182200215) != 0 {
				t.Fatal("collected cube item was not consumed")
			}
			choice := uint16(17)
			if len(template.Rewards[0].SelectableItems) > 0 || len(template.ClassRewards(p.Class)) > 0 {
				choice = 8
			}
			selectQuestDialog(c, end, choice, tc.id)
			if q := p.quest(tc.id); q.Status != "COMPLETE" {
				t.Fatalf("XML quest did not complete: %+v", q)
			}
		})
	}
}
