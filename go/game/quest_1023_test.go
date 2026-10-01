package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestANestOfLepharistsLevelUp(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.level = 14
	p.quests = []store.Quest{{ID: aNestOfLepharistsQuestID, Status: "LOCKED"}}
	c := &conn{s: s, player: p, tap: (&questPackets{}).tap}
	p.conn = c
	if c.aNestOfLepharistsLevelUp() {
		t.Fatal("unlocked below level 15")
	}
	p.level = 15
	if !c.aNestOfLepharistsLevelUp() || p.quest(aNestOfLepharistsQuestID).Status != "START" || c.aNestOfLepharistsLevelUp() {
		t.Fatalf("level-up transition failed or repeated: %+v", p.quest(aNestOfLepharistsQuestID))
	}
}

func TestANestOfLepharistsZoneProofAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[aNestOfLepharistsQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 15 || len(template.CollectItems) != 1 ||
		template.CollectItems[0].ID != aNestLepharistProofID || template.CollectItems[0].Count != 1 || len(template.Rewards) != 1 ||
		template.Rewards[0].Experience != 124600 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected A Nest of Lepharists metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 15
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.quests = []store.Quest{{ID: aNestOfLepharistsQuestID, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: aNestOfLepharistsQuestID, Kind: data.QuestCustom, StartNPC: 203098, EndNPC: 203098,
		TalkNPCs: []int32{203183}}
	latinus := questCatalogNPC(s, p, 203098, 0x61031)
	shipwreckOfficer := questCatalogNPC(s, p, 203183, 0x61032)

	if !c.aNestOfLepharistsDialog(latinus, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(latinus.id, 1011, aNestOfLepharistsQuestID).Data) {
		t.Fatalf("Latius opening page = %x", packets.last(smDialogWindow))
	}
	if c.aNestOfLepharistsDialog(latinus, script, 10001) || questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 0 {
		t.Fatal("invalid starter dialog advanced the quest")
	}
	if !c.aNestOfLepharistsDialog(latinus, script, 10000) || questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 1 ||
		c.aNestOfLepharistsDialog(latinus, script, 10000) {
		t.Fatalf("starter dialog failed or repeated: %+v", p.quest(aNestOfLepharistsQuestID))
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(shipwreckOfficer.id, 1011, aNestOfLepharistsQuestID).Data) {
		t.Fatalf("shipwreck officer page = %x", packets.last(smDialogWindow))
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 1012) || !bytes.Equal(packets.last(smPlayMovie), aNestOfLepharistsMovie(30).Data) {
		t.Fatal("shipwreck introduction movie failed")
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 10000) || questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 2 {
		t.Fatalf("report did not unlock the zone stage: %+v", p.quest(aNestOfLepharistsQuestID))
	}
	if c.aNestOfLepharistsEnterZone("WRONG_ZONE") || c.aNestOfLepharistsEnterZone("MYSTERIOUS_SHIPWRECK") == false {
		t.Fatal("zone transition rejected the correct stage or accepted the wrong zone")
	}
	if questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 3 || c.aNestOfLepharistsEnterZone("MYSTERIOUS_SHIPWRECK") {
		t.Fatalf("zone event did not advance exactly once: %+v", p.quest(aNestOfLepharistsQuestID))
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 10000) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(shipwreckOfficer.id, 1352, aNestOfLepharistsQuestID).Data) ||
		questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 3 {
		t.Fatal("var-3 report page did not preserve the stage")
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 10001) || questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 4 {
		t.Fatalf("second report did not advance: %+v", p.quest(aNestOfLepharistsQuestID))
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 10001) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(shipwreckOfficer.id, 1779, aNestOfLepharistsQuestID).Data) ||
		p.quest(aNestOfLepharistsQuestID).Status != "START" {
		t.Fatal("missing proof item did not block completion")
	}
	if !s.addItem(p, aNestMaskItemID, 1) || !s.addItem(p, aNestLepharistProofID, 1) {
		t.Fatal("could not add turn-in items")
	}
	if !c.aNestOfLepharistsDialog(shipwreckOfficer, script, 10001) || p.quest(aNestOfLepharistsQuestID).Status != "REWARD" ||
		questVar(p.quest(aNestOfLepharistsQuestID).Vars, 0) != 5 {
		t.Fatalf("proof turn-in failed: %+v", p.quest(aNestOfLepharistsQuestID))
	}
	if s.countItems(p, aNestMaskItemID) != 0 || s.countItems(p, aNestLepharistProofID) != 1 ||
		!bytes.Equal(packets.last(smPlayMovie), aNestOfLepharistsMovie(23).Data) {
		t.Fatalf("unexpected final turn-in cleanup or movie: mask=%d proof=%d", s.countItems(p, aNestMaskItemID), s.countItems(p, aNestLepharistProofID))
	}
	if c.aNestOfLepharistsDialog(shipwreckOfficer, script, 10001) {
		t.Fatal("repeated proof turn-in was accepted")
	}
	if !c.aNestOfLepharistsShowDialog(latinus, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(latinus.id, 5, aNestOfLepharistsQuestID).Data) {
		t.Fatal("reward preview failed")
	}
	if !c.aNestOfLepharistsDialog(latinus, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(latinus.id, 5, aNestOfLepharistsQuestID).Data) {
		t.Fatal("reward confirmation page failed")
	}
	choice := template.Rewards[0].SelectableItems[2]
	beforeExp := p.Exp
	if !c.aNestOfLepharistsDialog(latinus, script, 10) || p.quest(aNestOfLepharistsQuestID).Status != "COMPLETE" ||
		p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count {
		t.Fatalf("selectable reward failed: %+v item=%d", p.quest(aNestOfLepharistsQuestID), s.countItems(p, choice.ID))
	}
	if c.aNestOfLepharistsDialog(latinus, script, 10) || p.quest(aNestOfLepharistsQuestID).CompleteCount != 1 || s.countItems(p, choice.ID) != choice.Count {
		t.Fatal("repeated reward duplicated completion or item")
	}
}
