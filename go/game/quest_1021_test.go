package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestTrandilasEggsLevelUpDialogAndKill(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 10
	p.Exp = d.ExpStart(p.level)
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{
		{ID: trandilasEggsQuestID, Status: "LOCKED"},
		{ID: 1015, Status: "START"},
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: trandilasEggsQuestID, Kind: data.QuestCustom, StartNPC: trandilasEggsNPCID, EndNPC: trandilasEggsNPCID}
	pernos := questCatalogNPC(s, p, trandilasEggsNPCID, 0x31021)

	if c.trandilasEggsLevelUp() {
		t.Fatal("quest unlocked below level 11 or before Frillneck Hunt completion")
	}
	p.level = 11
	if c.trandilasEggsLevelUp() {
		t.Fatal("quest unlocked before Frillneck Hunt was complete")
	}
	p.quest(1015).Status = "COMPLETE"
	if !c.trandilasEggsLevelUp() || c.trandilasEggsLevelUp() || p.quest(trandilasEggsQuestID).Status != "START" {
		t.Fatalf("level-up transition = %+v", p.quest(trandilasEggsQuestID))
	}
	if c.trandilasEggsDialog(pernos, script, 9999) || c.trandilasEggsKill(questCatalogNPC(s, p, 210202, 0x31022)) {
		t.Fatal("quest advanced before Pernos's opening conversation")
	}
	if !c.trandilasEggsDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 1011, trandilasEggsQuestID).Data) {
		t.Fatalf("opening conversation = %x", packets.last(smDialogWindow))
	}
	if !c.trandilasEggsDialog(pernos, script, 10001) || c.trandilasEggsDialog(pernos, script, 10000) || questVar(p.quest(trandilasEggsQuestID).Vars, 0) != 1 {
		t.Fatalf("Pernos conversation state = %+v", p.quest(trandilasEggsQuestID))
	}
	if !c.trandilasEggsDialog(pernos, script, 25) || !bytes.Equal(packets.last(smPlayMovie), playMovie(27).Data) {
		t.Fatal("follow-up conversation did not play movie 27")
	}
	if !c.trandilasEggsDialog(pernos, script, 1012) || !bytes.Equal(packets.last(smPlayMovie), playMovie(27).Data) {
		t.Fatal("dialog 1012 did not play movie 27")
	}
	wrongMob := questCatalogNPC(s, p, trandilasEggsMobID+1, 0x31023)
	if c.trandilasEggsKill(wrongMob) || p.quest(trandilasEggsQuestID).Status != "START" {
		t.Fatal("wrong kill target advanced the quest")
	}
	mob := questCatalogNPC(s, p, trandilasEggsMobID, 0x31024)
	if !c.trandilasEggsKill(mob) || c.trandilasEggsKill(mob) || p.quest(trandilasEggsQuestID).Status != "REWARD" {
		t.Fatalf("kill completion state = %+v", p.quest(trandilasEggsQuestID))
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(mob.id, 10, 0).Data) {
		t.Fatalf("kill dialog close = %x", packets.last(smDialogWindow))
	}
}

func TestTrandilasEggsReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[trandilasEggsQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0].ID != 120000859 {
		t.Fatal("Trandila's Eggs reward is missing from quest metadata")
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.quests = []store.Quest{{ID: trandilasEggsQuestID, Status: "REWARD", Vars: 1}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: trandilasEggsQuestID, Kind: data.QuestCustom, EndNPC: trandilasEggsNPCID}
	pernos := questCatalogNPC(s, p, trandilasEggsNPCID, 0x32021)

	if !c.trandilasEggsShowDialog(pernos, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 5, trandilasEggsQuestID).Data) {
		t.Fatalf("reward preview = %x", packets.last(smDialogWindow))
	}
	if !c.trandilasEggsDialog(pernos, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 5, trandilasEggsQuestID).Data) {
		t.Fatalf("reward start dialog = %x", packets.last(smDialogWindow))
	}
	beforeExp := p.Exp
	if !c.trandilasEggsDialog(pernos, script, 17) {
		t.Fatal("fixed reward completion dialog was not handled")
	}
	if q := p.quest(trandilasEggsQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, 120000859) != 1 {
		t.Fatalf("reward mismatch: quest=%+v exp=%d item=%d", q, p.Exp-beforeExp, s.countItems(p, 120000859))
	}
	if c.trandilasEggsDialog(pernos, script, 17) || p.quest(trandilasEggsQuestID).CompleteCount != 1 || s.countItems(p, 120000859) != 1 {
		t.Fatal("repeated reward dialog granted a duplicate")
	}
}
