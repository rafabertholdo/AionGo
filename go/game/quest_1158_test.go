package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestVillageSealFoundSearchAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[villageSealFoundQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 14 || len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1157 || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Village Seal Found metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.WorldID = 210030000
	p.level = template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1157, Status: "COMPLETE", CompleteCount: 1}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: villageSealFoundQuestID, Kind: data.QuestCustom, StartNPC: villageSealFoundStart, EndNPC: villageSealFoundEnd}
	start := questCatalogNPC(s, p, villageSealFoundStart, 0x31158)
	search := questCatalogNPC(s, p, villageSealFoundSearch, 0x31159)
	end := questCatalogNPC(s, p, villageSealFoundEnd, 0x31160)

	if !c.villageSealFoundDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, villageSealFoundQuestID).Data) {
		t.Fatal("quest offer did not show")
	}
	if !c.villageSealFoundDialog(start, script, 1002) || p.quest(villageSealFoundQuestID) == nil || p.quest(villageSealFoundQuestID).Status != "START" {
		t.Fatal("quest acceptance did not start the quest")
	}
	if c.villageSealFoundDialog(end, script, 25) || c.villageSealFoundDialog(search, script, 10001) {
		t.Fatal("wrong NPC or dialog advanced the quest")
	}
	if !c.villageSealFoundDialog(search, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(search.id, 1352, villageSealFoundQuestID).Data) {
		t.Fatal("search conversation did not show")
	}
	p.spawned = true
	if !c.villageSealFoundDialog(search, script, 1353) || search.useTask == nil || c.villageSealFoundDialog(search, script, 1353) {
		t.Fatal("search animation did not start exactly once")
	}
	time.Sleep(3100 * time.Millisecond)
	if search.useTask != nil || p.quest(villageSealFoundQuestID).Status != "START" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(search.id, 1353, villageSealFoundQuestID).Data) {
		t.Fatal("delayed search did not restore the dialog")
	}
	if !c.villageSealFoundDialog(search, script, 10000) || p.quest(villageSealFoundQuestID).Status != "REWARD" || questVar(p.quest(villageSealFoundQuestID).Vars, 0) != 1 || s.countItems(p, villageSealFoundItemID) != 1 {
		t.Fatal("seal handoff did not grant the item and unlock reward")
	}
	if c.villageSealFoundDialog(search, script, 10000) || s.countItems(p, villageSealFoundItemID) != 1 {
		t.Fatal("repeated seal handoff granted a duplicate")
	}
	if c.villageSealFoundDialog(start, script, -1) {
		t.Fatal("non-end NPC showed the reward")
	}
	if !c.villageSealFoundDialog(end, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2375, villageSealFoundQuestID).Data) {
		t.Fatal("reward preview did not show")
	}
	if !c.villageSealFoundDialog(end, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, villageSealFoundQuestID).Data) {
		t.Fatal("reward page did not show")
	}
	choice := template.Rewards[0].SelectableItems[0]
	beforeExp := p.Exp
	if !c.villageSealFoundDialog(end, script, 8) {
		t.Fatal("valid reward selection was rejected")
	}
	if q := p.quest(villageSealFoundQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count || s.countItems(p, villageSealFoundItemID) != 0 {
		t.Fatalf("reward mismatch: quest=%+v exp=%d choice=%d workitem=%d", q, p.Exp-beforeExp, s.countItems(p, choice.ID), s.countItems(p, villageSealFoundItemID))
	}
	if c.villageSealFoundDialog(end, script, 8) || p.quest(villageSealFoundQuestID).CompleteCount != 1 {
		t.Fatal("duplicate reward was accepted")
	}
}

func TestVillageSealFoundSelectableRewards(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[villageSealFoundQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatal("Village Seal Found reward choices are missing")
	}
	for index, choice := range template.Rewards[0].SelectableItems {
		t.Run(choiceName(index), func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.Race, p.Class = "ELYOS", "SORCERER"
			p.level = template.MinLevel
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.quests = []store.Quest{{ID: villageSealFoundQuestID, Status: "REWARD", Vars: 1}}
			c := &conn{s: s, player: p}
			p.conn = c
			s.spawned[p.ID] = p
			if !s.addItem(p, villageSealFoundItemID, 1) {
				t.Fatal("could not add seal work item")
			}
			script := &data.QuestScript{ID: villageSealFoundQuestID, Kind: data.QuestCustom, EndNPC: villageSealFoundEnd}
			end := questCatalogNPC(s, p, villageSealFoundEnd, 0x32158)
			beforeExp := p.Exp
			if !c.villageSealFoundDialog(end, script, int32(8+index)) {
				t.Fatal("valid reward choice was rejected")
			}
			if q := p.quest(villageSealFoundQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count || s.countItems(p, villageSealFoundItemID) != 0 {
				t.Fatalf("wrong reward choice %d: quest=%+v exp=%d selected=%d seal=%d", index, q, p.Exp-beforeExp, s.countItems(p, choice.ID), s.countItems(p, villageSealFoundItemID))
			}
		})
	}
}

func TestVillageSealFoundSealInventoryGuard(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 14
	p.cube = nil
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: villageSealFoundQuestID, Status: "START"}}
	c := &conn{s: s, player: p}
	search := questCatalogNPC(s, p, villageSealFoundSearch, 0x33158)
	script := &data.QuestScript{ID: villageSealFoundQuestID, Kind: data.QuestCustom, EndNPC: villageSealFoundEnd}
	for itemID := int32(100000001); len(p.cube) < p.cubeLimit(); itemID++ {
		p.cube = append(p.cube, &store.Item{UniqueID: int32(len(p.cube) + 1), ItemID: itemID, Count: 1, Owner: p.ID})
	}
	if !c.villageSealFoundDialog(search, script, 10000) || p.quest(villageSealFoundQuestID).Status != "START" || s.countItems(p, villageSealFoundItemID) != 0 {
		t.Fatal("full inventory advanced the quest or lost the seal")
	}
}
