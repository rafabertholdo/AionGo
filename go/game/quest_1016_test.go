package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSourcePollutionLevelUpEligibility(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.level = 11
	p.quests = []store.Quest{{ID: sourcePollutionQuestID, Status: "LOCKED"}}
	c := &conn{s: s, player: p, tap: (&questPackets{}).tap}
	p.conn = c
	if c.sourcePollutionLevelUp() {
		t.Fatal("unlocked below the minimum level")
	}
	p.level = 12
	if !c.sourcePollutionLevelUp() || p.quest(sourcePollutionQuestID).Status != "START" || c.sourcePollutionLevelUp() {
		t.Fatalf("level-up transition failed or repeated: %+v", p.quest(sourcePollutionQuestID))
	}
}

func TestSourcePollutionQuestFlowAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[sourcePollutionQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 12 || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected quest metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 12
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: sourcePollutionQuestID, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: sourcePollutionQuestID, Kind: data.QuestCustom, EndNPC: 203098}

	conversation := []struct {
		npcID int32
		page  uint16
		step  int32
	}{
		{203149, 1011, 0},
		{203148, 1352, 1},
		{203149, 1693, 2},
		{203832, 2034, 3},
		{203705, 2375, 4},
		{203822, 2716, 5},
		{203761, 3057, 6},
		{203149, 3398, 7},
	}
	for _, stage := range conversation {
		npc := questCatalogNPC(s, p, stage.npcID, 0x61000+stage.step)
		if !c.sourcePollutionDialog(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, stage.page, sourcePollutionQuestID).Data) {
			t.Fatalf("NPC %d stage %d conversation page = %x", stage.npcID, stage.step, packets.last(smDialogWindow))
		}
		if c.sourcePollutionDialog(npc, script, 9999) || questVar(p.quest(sourcePollutionQuestID).Vars, 0) != stage.step {
			t.Fatalf("invalid dialog advanced stage %d", stage.step)
		}
		if stage.step == 1 && !c.s.addItem(p, sourcePollutionWater, 1) {
			t.Fatal("could not add contaminated water prerequisite")
		}
		if stage.step == 5 && s.countItems(p, sourcePollutionWater) == 0 {
			t.Fatal("contaminated water was not given by the previous NPC")
		}
		dialogID := int32(10000 + stage.step)
		if stage.step == 7 {
			dialogID = 10007
		}
		if !c.sourcePollutionDialog(npc, script, dialogID) || questVar(p.quest(sourcePollutionQuestID).Vars, 0) != stage.step+1 {
			t.Fatalf("NPC %d failed stage %d: quest=%+v", stage.npcID, stage.step, p.quest(sourcePollutionQuestID))
		}
		if c.sourcePollutionDialog(npc, script, dialogID) {
			t.Fatalf("repeated dialog advanced stage %d", stage.step)
		}
	}
	if s.countItems(p, sourcePollutionGuide) != 0 || s.countItems(p, sourcePollutionTear) != 0 || s.countItems(p, sourcePollutionMedicine) != 2 {
		t.Fatalf("stage 7 item exchange failed: guide=%d tear=%d medicine=%d", s.countItems(p, sourcePollutionGuide), s.countItems(p, sourcePollutionTear), s.countItems(p, sourcePollutionMedicine))
	}
	principal := questCatalogNPC(s, p, 203149, 0x61020)
	if !c.sourcePollutionDialog(principal, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(principal.id, 3569, sourcePollutionQuestID).Data) {
		t.Fatalf("medicine status page = %x", packets.last(smDialogWindow))
	}
	if !c.sourcePollutionDialog(principal, script, 10008) || s.countItems(p, sourcePollutionMedicine) != 4 || questVar(p.quest(sourcePollutionQuestID).Vars, 0) != 8 {
		t.Fatal("stage 8 medicine refill did not grant two without advancing")
	}
	if !c.sourcePollutionDialog(principal, script, 3400) || !bytes.Equal(packets.last(smPlayMovie), sourcePollutionMovie().Data) {
		t.Fatal("expected the researcher movie")
	}

	monster := questCatalogNPC(s, p, 210318, 0x61021)
	wrongMonster := questCatalogNPC(s, p, 210319, 0x61023)
	if c.sourcePollutionKill(wrongMonster) || questVar(p.quest(sourcePollutionQuestID).Vars, 0) != 8 {
		t.Fatal("wrong monster advanced the kill stage")
	}
	if !c.sourcePollutionKill(monster) || questVar(p.quest(sourcePollutionQuestID).Vars, 0) != 9 || c.sourcePollutionKill(monster) {
		t.Fatalf("kill stage failed or repeated: %+v", p.quest(sourcePollutionQuestID))
	}
	time.Sleep(5200 * time.Millisecond)
	var researcher *object
	for _, spawned := range s.byID {
		if spawned.npc != nil && spawned.npc.ID == 203195 {
			researcher = spawned
			break
		}
	}
	if researcher == nil {
		t.Fatal("kill did not spawn the temporary researcher")
	}
	p.seen[researcher.id] = researcher
	s.removeItemsByID(p, sourcePollutionMedicine, s.countItems(p, sourcePollutionMedicine))
	if !c.sourcePollutionDialog(researcher, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(researcher.id, 3739, sourcePollutionQuestID).Data) {
		t.Fatalf("researcher page = %x", packets.last(smDialogWindow))
	}
	if c.sourcePollutionDialog(researcher, script, 10008) || p.quest(sourcePollutionQuestID).Status != "START" {
		t.Fatal("researcher accepted turn-in without its medicine")
	}
	if !s.addItem(p, sourcePollutionMedicine, 1) {
		t.Fatal("could not add medicine for researcher")
	}
	if !c.sourcePollutionDialog(researcher, script, 10008) || p.quest(sourcePollutionQuestID).Status != "REWARD" || s.countItems(p, sourcePollutionMedicine) != 0 {
		t.Fatalf("research turn-in failed: quest=%+v medicine=%d", p.quest(sourcePollutionQuestID), s.countItems(p, sourcePollutionMedicine))
	}
	if s.countItems(p, sourcePollutionDiary) != 1 {
		t.Fatal("research diary was not granted")
	}
	end := questCatalogNPC(s, p, 203098, 0x61022)
	if !c.sourcePollutionShowDialog(end, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 4080, sourcePollutionQuestID).Data) {
		t.Fatal("reward preview failed")
	}
	if c.sourcePollutionDialog(end, script, 1009) == false || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, sourcePollutionQuestID).Data) {
		t.Fatal("reward confirmation page failed")
	}
	choice := template.Rewards[0].SelectableItems[2]
	beforeExp := p.Exp
	if !c.sourcePollutionDialog(end, script, 10) || p.quest(sourcePollutionQuestID).Status != "COMPLETE" || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count {
		t.Fatalf("selectable reward failed: quest=%+v item %d=%d", p.quest(sourcePollutionQuestID), choice.ID, s.countItems(p, choice.ID))
	}
	if c.sourcePollutionDialog(end, script, 10) || p.quest(sourcePollutionQuestID).CompleteCount != 1 || s.countItems(p, choice.ID) != choice.Count {
		t.Fatal("repeated reward duplicated quest completion or items")
	}
}
