package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestFrillneckHuntLevelUpAndProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 9
	p.Exp = d.ExpStart(p.level)
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: frillneckHuntQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: frillneckHuntQuestID, EndNPC: 203129}
	leto := questCatalogNPC(s, p, 203129, 0x31015)

	if c.frillneckHuntLevelUp() {
		t.Fatal("quest unlocked below its minimum level")
	}
	p.level = 10
	if !c.frillneckHuntLevelUp() || c.frillneckHuntLevelUp() || p.quest(frillneckHuntQuestID).Status != "START" {
		t.Fatalf("level up transition = %+v", p.quest(frillneckHuntQuestID))
	}
	if c.frillneckHuntKill(210126) {
		t.Fatal("kill advanced before the first conversation")
	}
	c.frillneckHuntDialog(leto, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(leto.id, 1011, frillneckHuntQuestID).Data) {
		t.Fatalf("opening conversation = %x", got)
	}
	c.frillneckHuntDialog(leto, script, 10000)
	c.frillneckHuntDialog(leto, script, 10000)
	if questVar(p.quest(frillneckHuntQuestID).Vars, 0) != 1 {
		t.Fatal("first conversation did not start the hunt")
	}
	c.frillneckHuntDialog(leto, script, 1012)
	if got := packets.last(smPlayMovie); !bytes.Equal(got, frillneckHuntMovie().Data) {
		t.Fatalf("Leto's movie = %x", got)
	}
	if c.frillneckHuntKill(210200) || c.frillneckHuntKill(210126+1) {
		t.Fatal("wrong mob advanced the first hunt phase")
	}
	for kill := 0; kill < 7; kill++ {
		if !c.frillneckHuntKill(210126) {
			t.Fatalf("Rakeclaw kill %d did not advance", kill+1)
		}
	}
	if got := questVar(p.quest(frillneckHuntQuestID).Vars, 0); got != 8 || c.frillneckHuntKill(210126) {
		t.Fatalf("first hunt boundary = %d", got)
	}
	c.frillneckHuntDialog(leto, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(leto.id, 1352, frillneckHuntQuestID).Data) {
		t.Fatalf("second conversation = %x", got)
	}
	c.frillneckHuntDialog(leto, script, 10001)
	if questVar(p.quest(frillneckHuntQuestID).Vars, 0) != 9 {
		t.Fatalf("second hunt state = %+v", p.quest(frillneckHuntQuestID))
	}
	for kill := 0; kill < 12; kill++ {
		npcID := int32(210200)
		if kill%2 == 1 {
			npcID = 210201
		}
		if !c.frillneckHuntKill(npcID) {
			t.Fatalf("Giant Rakeclaw kill %d did not advance", kill+1)
		}
	}
	if q := p.quest(frillneckHuntQuestID); q.Status != "REWARD" || questVar(q.Vars, 0) != 20 || c.frillneckHuntKill(210200) {
		t.Fatalf("second hunt completion = %+v", q)
	}
}

func TestFrillneckHuntRewardChoices(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[frillneckHuntQuestID]
	if template == nil || len(template.Rewards) == 0 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatal("Frillneck Hunt reward choices are missing from quest metadata")
	}
	for index, choice := range template.Rewards[0].SelectableItems {
		t.Run(choiceName(index), func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.Race = "ELYOS"
			p.Class = "SORCERER" // Advanced class lets the reward experience exceed the level-10 cap.
			p.level = template.MinLevel
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.quests = []store.Quest{{ID: frillneckHuntQuestID, Status: "REWARD", Vars: 20}}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			script := &data.QuestScript{ID: frillneckHuntQuestID, EndNPC: 203129}
			leto := questCatalogNPC(s, p, 203129, 0x32015)

			if !c.frillneckHuntShowDialog(leto, script) {
				t.Fatal("reward preview did not handle Leto's show-dialog event")
			}
			if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(leto.id, 1693, frillneckHuntQuestID).Data) {
				t.Fatalf("reward preview = %x", got)
			}
			beforeExp := p.Exp
			c.frillneckHuntDialog(leto, script, int32(8+index))
			if q := p.quest(frillneckHuntQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count {
				t.Fatalf("reward choice %d completion = %+v, exp=%d, item=%d", index, q, p.Exp-beforeExp, s.countItems(p, choice.ID))
			}
			c.frillneckHuntDialog(leto, script, int32(8+index))
			if q := p.quest(frillneckHuntQuestID); q.CompleteCount != 1 || s.countItems(p, choice.ID) != choice.Count {
				t.Fatal("repeated reward dialog granted a duplicate")
			}
		})
	}
}

func choiceName(index int) string {
	return string(rune('A' + index))
}
