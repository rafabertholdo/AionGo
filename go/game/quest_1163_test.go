package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestArachnaAntidoteDialogueAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[arachnaAntidoteQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 15 || len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1016 ||
		len(template.QuestWorkItems) != 1 || template.QuestWorkItems[0] != (data.QuestItem{ID: 182200564, Count: 1}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 37680 || template.Rewards[0].Kinah != 3000 ||
		len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 162000022, Count: 4}) {
		t.Fatalf("unexpected Arachna Antidote metadata: %+v", template)
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Class = "SORCERER"
	p.Race, p.level, p.Exp = "ELYOS", 15, d.ExpStart(15)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.quests = []store.Quest{{ID: 1016, Status: "COMPLETE", CompleteCount: 1}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: arachnaAntidoteQuestID, Kind: data.QuestCustom, StartNPC: arachnaAntidoteStartNPCID, EndNPC: arachnaAntidoteEndNPCID}
	start := questCatalogNPC(s, p, arachnaAntidoteStartNPCID, 0x31163)
	middle := questCatalogNPC(s, p, arachnaAntidoteMiddleNPCID, 0x31164)
	end := questCatalogNPC(s, p, arachnaAntidoteEndNPCID, 0x31165)
	dialog := func(o *object, id int32) bool { return c.arachnaAntidoteDialog(o, script, id) }

	if !dialog(start, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, arachnaAntidoteQuestID).Data) {
		t.Fatalf("start offer = %x", packets.last(smDialogWindow))
	}
	if !dialog(start, 1007) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 4, arachnaAntidoteQuestID).Data) {
		t.Fatalf("start-side 1007 page = %x", packets.last(smDialogWindow))
	}
	if !dialog(start, 1003) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1004, arachnaAntidoteQuestID).Data) {
		t.Fatalf("start-side 1003 page = %x", packets.last(smDialogWindow))
	}
	if !dialog(start, 1002) || p.quest(arachnaAntidoteQuestID) == nil || p.quest(arachnaAntidoteQuestID).Status != "START" {
		t.Fatalf("quest did not start: %+v", p.quest(arachnaAntidoteQuestID))
	}
	if dialog(start, 1002) {
		t.Fatal("repeated start dialog was unexpectedly handled")
	}
	if dialog(middle, -1) || dialog(middle, 10001) || questVar(p.quest(arachnaAntidoteQuestID).Vars, 0) != 0 {
		t.Fatal("middle NPC accepted an unsupported dialog")
	}
	if !dialog(middle, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 1352, arachnaAntidoteQuestID).Data) {
		t.Fatalf("middle page = %x", packets.last(smDialogWindow))
	}
	if !dialog(middle, 10000) || questVar(p.quest(arachnaAntidoteQuestID).Vars, 0) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 10, 0).Data) {
		t.Fatalf("middle accept = %x, quest=%+v", packets.last(smDialogWindow), p.quest(arachnaAntidoteQuestID))
	}
	if !dialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2375, arachnaAntidoteQuestID).Data) {
		t.Fatalf("end page = %x", packets.last(smDialogWindow))
	}
	if dialog(end, 1008) || p.quest(arachnaAntidoteQuestID).Status != "START" {
		t.Fatal("end NPC accepted an unsupported turn-in")
	}
	if !dialog(end, 1009) || p.quest(arachnaAntidoteQuestID).Status != "REWARD" || questVar(p.quest(arachnaAntidoteQuestID).Vars, 0) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10, 0).Data) {
		t.Fatalf("end turn-in = %x, quest=%+v", packets.last(smDialogWindow), p.quest(arachnaAntidoteQuestID))
	}
	if !dialog(end, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, arachnaAntidoteQuestID).Data) ||
		!dialog(end, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, arachnaAntidoteQuestID).Data) {
		t.Fatalf("reward-ready/click pages are wrong: %x", packets.last(smDialogWindow))
	}
	beforeExperience, beforeKinah := p.Exp, p.kinah.Count
	if !dialog(end, 8) || p.quest(arachnaAntidoteQuestID).Status != "COMPLETE" || p.quest(arachnaAntidoteQuestID).CompleteCount != 1 ||
		p.Exp-beforeExperience != template.Rewards[0].Experience || p.kinah.Count-beforeKinah != template.Rewards[0].Kinah || s.countItems(p, 162000022) != 4 {
		t.Fatalf("fixed reward did not complete quest: quest=%+v xp=%d kinah=%d item=%d", p.quest(arachnaAntidoteQuestID), p.Exp-beforeExperience, p.kinah.Count-beforeKinah, s.countItems(p, 162000022))
	}
}
