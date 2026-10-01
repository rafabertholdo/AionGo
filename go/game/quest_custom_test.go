package game

import (
	"bytes"
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestCustomNPCQuestChains(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		id, middleNPC, startVarItem int32
	}{
		{2125, 203514, 0},
		{2135, 203531, 182203131},
	} {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			script := d.QuestScripts[tc.id]
			template := d.Quests[tc.id]
			if script == nil || script.Kind != data.QuestCustom || template == nil {
				t.Fatalf("custom quest not registered: %+v", script)
			}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
			p.level = max(1, template.MinLevel)
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
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
			selectQuestDialog(c, start, 25, script.ID)
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(start.id, 1011, script.ID).Data) {
				t.Fatalf("custom quest offer = %x", got)
			}
			selectQuestDialog(c, start, 1002, script.ID)
			if q := p.quest(script.ID); q == nil || q.Status != "START" || q.Vars != 0 {
				t.Fatalf("custom quest did not start: %+v", q)
			}
			if tc.startVarItem != 0 && s.countItems(p, tc.startVarItem) != 1 {
				t.Fatal("starter item was not given")
			}
			selectQuestDialog(c, middle, 25, script.ID)
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(middle.id, 1352, script.ID).Data) {
				t.Fatalf("middle NPC dialog = %x", got)
			}
			selectQuestDialog(c, middle, 10000, script.ID)
			if q := p.quest(script.ID); q.Status != "START" || q.Vars != 1 {
				t.Fatalf("middle NPC did not advance quest: %+v", q)
			}
			if tc.startVarItem != 0 && s.countItems(p, tc.startVarItem) != 0 {
				t.Fatal("loaned item was not returned")
			}
			selectQuestDialog(c, end, 25, script.ID)
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(end.id, 2375, script.ID).Data) {
				t.Fatalf("turn-in dialog = %x", got)
			}
			selectQuestDialog(c, end, 1009, script.ID)
			if q := p.quest(script.ID); q.Status != "REWARD" || q.Vars != 2 {
				t.Fatalf("custom quest did not reach reward: %+v", q)
			}
			selectQuestDialog(c, end, 17, script.ID)
			if q := p.quest(script.ID); q.Status != "COMPLETE" || q.CompleteCount != 1 {
				t.Fatalf("custom quest did not complete: %+v", q)
			}
		})
	}
}

func TestInsomniaMedicineChoosesRewardRecord(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[1111]
	dropCount := 0
	for _, dropScript := range d.QuestDropsByNPC[210261] {
		if dropScript.ID == 1111 {
			dropCount++
		}
	}
	if script == nil || script.Kind != data.QuestCustom || dropCount != 1 {
		t.Fatalf("Insomnia Medicine or its quest drop is not registered: %+v", script)
	}
	for _, tc := range []struct {
		choice uint16
		vars   int32
		item   int32
		xp     int64
		kinah  int64
	}{
		{10000, 2, 182200222, 1450, 250},
		{10001, 3, 182200221, 1150, 450},
	} {
		t.Run(fmt.Sprint(tc.choice), func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.level = 3
			p.Exp = d.ExpStart(3)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			start := questCatalogNPC(s, p, script.StartNPC, 0x30001)
			middle := questCatalogNPC(s, p, 203061, 0x30002)
			selectQuestDialog(c, start, 1002, script.ID)
			selectQuestDialog(c, middle, 33, script.ID)
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(middle.id, 1693, script.ID).Data) {
				t.Fatalf("missing ingredients dialog = %x", got)
			}
			if !s.addItem(p, 182200223, 5) {
				t.Fatal("could not add ingredients")
			}
			selectQuestDialog(c, middle, 33, script.ID)
			if q := p.quest(script.ID); q.Status != "START" || q.Vars != 1 || s.countItems(p, 182200223) != 0 {
				t.Fatalf("ingredients were not collected: %+v", q)
			}
			selectQuestDialog(c, middle, tc.choice, script.ID)
			if q := p.quest(script.ID); q.Status != "REWARD" || q.Vars != tc.vars || s.countItems(p, tc.item) != 1 {
				t.Fatalf("reward branch did not start: %+v", q)
			}
			c.showDialog(dialogRequest(cmShowDialog, start.id, 0, 0))
			if s.countItems(p, tc.item) != 0 {
				t.Fatal("temporary potion was not removed")
			}
			selectQuestDialog(c, start, 1009, script.ID)
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(start.id, uint16(tc.vars+3), script.ID).Data) {
				t.Fatalf("branch reward dialog = %x", got)
			}
			beforeExp := p.Exp
			selectQuestDialog(c, start, 17, script.ID)
			if q := p.quest(script.ID); q.Status != "COMPLETE" || p.Exp-beforeExp != tc.xp || p.kinah.Count != tc.kinah {
				t.Fatalf("wrong reward record: q=%+v exp=%d kinah=%d", q, p.Exp-beforeExp, p.kinah.Count)
			}
		})
	}
}

func TestPernosRobeThreeRewardBranches(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[1122]
	if script == nil || script.Kind != data.QuestCustom {
		t.Fatalf("Pernos's Robe is not registered: %+v", script)
	}
	for index, turnIn := range []int32{182200218, 182200219, 182200220} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.level = 7
			p.Exp = d.ExpStart(7)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.quests = []store.Quest{{ID: 1116, Status: "COMPLETE", CompleteCount: 1}}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			start := questCatalogNPC(s, p, script.StartNPC, 0x30001)
			end := questCatalogNPC(s, p, script.EndNPC, 0x30002)
			selectQuestDialog(c, start, 1002, script.ID)
			if q := p.quest(script.ID); q == nil || q.Status != "START" || s.countItems(p, script.ItemID) != 1 {
				t.Fatalf("quest or robe was not granted: %+v", q)
			}
			selectQuestDialog(c, end, uint16(10000+index), script.ID)
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(end.id, 1608, script.ID).Data) {
				t.Fatalf("missing robe dialog = %x", got)
			}
			if !s.addItem(p, turnIn, 1) {
				t.Fatalf("could not add turn-in item %d", turnIn)
			}
			selectQuestDialog(c, end, uint16(10000+index), script.ID)
			if q := p.quest(script.ID); q.Status != "REWARD" || q.Vars != int32(index+1) || s.countItems(p, turnIn) != 0 || s.countItems(p, script.ItemID) != 0 {
				t.Fatalf("branch did not reach reward: %+v", q)
			}
			c.showDialog(dialogRequest(cmShowDialog, end.id, 0, 0))
			if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(end.id, uint16(5+index), script.ID).Data) {
				t.Fatalf("reward branch dialog = %x", got)
			}
			beforeExp := p.Exp
			selectQuestDialog(c, end, 17, script.ID)
			reward := d.Quests[script.ID].Rewards[index]
			if q := p.quest(script.ID); q.Status != "COMPLETE" || p.Exp-beforeExp != reward.Experience || s.countItems(p, reward.Items[0].ID) != reward.Items[0].Count {
				t.Fatalf("wrong reward record: q=%+v exp=%d items=%+v", q, p.Exp-beforeExp, p.cube)
			}
		})
	}
}
