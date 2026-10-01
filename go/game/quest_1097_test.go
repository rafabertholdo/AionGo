package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSwordOfTranscendenceNPCStart(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 50
	p.cube = nil
	p.quests = []store.Quest{{ID: 1096, Status: "COMPLETE"}}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: swordTranscendenceQuestID, Kind: data.QuestCustom, StartNPC: swordTranscendencePernos, EndNPC: swordTranscendencePernos}
	pernos := questCatalogNPC(s, p, swordTranscendencePernos, 0x31096)

	if !c.swordTranscendenceDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 1011, swordTranscendenceQuestID).Data) {
		t.Fatal("NPC did not offer Sword of Transcendence")
	}
	if !c.swordTranscendenceDialog(pernos, script, 1002) || p.quest(swordTranscendenceQuestID) == nil || p.quest(swordTranscendenceQuestID).Status != "START" {
		t.Fatal("NPC acceptance did not start the quest")
	}
	if c.swordTranscendenceDialog(pernos, script, 1002) || p.quest(swordTranscendenceQuestID).Vars != 0 {
		t.Fatal("repeated acceptance changed the active quest")
	}
}

func TestSwordOfTranscendenceLevelUpAndNPCProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[swordTranscendenceQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 50 || len(template.CollectItems) != 3 || len(template.Rewards) != 1 {
		t.Fatalf("unexpected Sword of Transcendence metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{
		{ID: swordTranscendenceQuestID, Status: "LOCKED"},
		{ID: 1096, Status: "START"},
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: swordTranscendenceQuestID, Kind: data.QuestCustom, StartNPC: swordTranscendencePernos, EndNPC: swordTranscendencePernos}
	pernos := questCatalogNPC(s, p, swordTranscendencePernos, 0x31097)
	anusis := questCatalogNPC(s, p, swordTranscendenceAnusis, 0x31098)
	baoninerk := questCatalogNPC(s, p, swordTranscendenceShugo, 0x31099)

	if c.swordTranscendenceLevelUp() {
		t.Fatal("quest unlocked before its prerequisite completed")
	}
	p.quest(1096).Status = "COMPLETE"
	if !c.swordTranscendenceLevelUp() || p.quest(swordTranscendenceQuestID).Status != "START" || c.swordTranscendenceLevelUp() {
		t.Fatalf("prerequisite level-up transition failed or repeated: %+v", p.quest(swordTranscendenceQuestID))
	}
	if c.swordTranscendenceDialog(anusis, script, 25) || c.swordTranscendenceDialog(pernos, script, 999) {
		t.Fatal("wrong NPC or dialog advanced the quest")
	}
	if !c.swordTranscendenceDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 1011, swordTranscendenceQuestID).Data) {
		t.Fatal("Pernos's opening dialog did not show")
	}
	if !c.swordTranscendenceDialog(pernos, script, 10000) || questVar(p.quest(swordTranscendenceQuestID).Vars, 0) != 1 || c.swordTranscendenceDialog(pernos, script, 10000) {
		t.Fatal("Pernos's report did not advance exactly once")
	}
	if !c.swordTranscendenceDialog(anusis, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(anusis.id, 1352, swordTranscendenceQuestID).Data) || !c.swordTranscendenceDialog(anusis, script, 10001) || questVar(p.quest(swordTranscendenceQuestID).Vars, 0) != 2 {
		t.Fatal("Anusis's report did not advance")
	}
	if !c.swordTranscendenceDialog(baoninerk, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(baoninerk.id, 1693, swordTranscendenceQuestID).Data) {
		t.Fatal("Baoninerk's dialog did not show")
	}
	if !c.swordTranscendenceDialog(baoninerk, script, 33) || questVar(p.quest(swordTranscendenceQuestID).Vars, 0) != 3 || s.countItems(p, swordTranscendenceItemID) != 1 {
		t.Fatal("Baoninerk did not grant the quest work item")
	}
	if c.swordTranscendenceDialog(baoninerk, script, 33) || s.countItems(p, swordTranscendenceItemID) != 1 {
		t.Fatal("repeated Baoninerk dialog granted a duplicate item")
	}
	if !c.swordTranscendenceDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 10002, swordTranscendenceQuestID).Data) {
		t.Fatal("Pernos's final conversation did not show")
	}
	if !c.swordTranscendenceDialog(pernos, script, 1009) || p.quest(swordTranscendenceQuestID).Status != "REWARD" {
		t.Fatal("final conversation did not unlock reward")
	}
	if !c.swordTranscendenceDialog(pernos, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 5, swordTranscendenceQuestID).Data) {
		t.Fatal("reward dialog did not show")
	}
	beforeExp := p.Exp
	if !c.swordTranscendenceDialog(pernos, script, 17) || p.quest(swordTranscendenceQuestID).Status != "COMPLETE" || p.Exp-beforeExp != template.Rewards[0].Experience {
		t.Fatalf("quest reward mismatch: quest=%+v gained experience=%d", p.quest(swordTranscendenceQuestID), p.Exp-beforeExp)
	}
	if c.swordTranscendenceDialog(pernos, script, 17) || p.quest(swordTranscendenceQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward selection completed the quest again")
	}
}

func TestSwordOfTranscendenceWorkItemInventoryGuard(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 50
	p.cube = nil
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: swordTranscendenceQuestID, Status: "START", Vars: 2}}
	c := &conn{s: s, player: p}
	shugo := questCatalogNPC(s, p, swordTranscendenceShugo, 0x31100)
	script := &data.QuestScript{ID: swordTranscendenceQuestID, Kind: data.QuestCustom, StartNPC: swordTranscendencePernos, EndNPC: swordTranscendencePernos}

	// Fill all cube slots with distinct non-stackable items so the work item
	// grant cannot fit. The quest must stay at variable 2 until capacity exists.
	for itemID := int32(100000001); len(p.cube) < p.cubeLimit(); itemID++ {
		p.cube = append(p.cube, &store.Item{UniqueID: int32(len(p.cube) + 1), ItemID: itemID, Count: 1, Owner: p.ID})
	}
	if !c.swordTranscendenceDialog(shugo, script, 33) || questVar(p.quest(swordTranscendenceQuestID).Vars, 0) != 2 || s.countItems(p, swordTranscendenceItemID) != 0 {
		t.Fatal("full inventory advanced the quest or granted the item")
	}
}
