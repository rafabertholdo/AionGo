package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestBelbuasTreasureOfferAndBarrelReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[belbuasTreasureQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 11 || len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1143 || len(template.Rewards) != 1 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 160000003, Count: 2}) {
		t.Fatalf("unexpected Belbua's Treasure metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Class = "SORCERER"
	p.Race = "ELYOS"
	p.level = template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1143, Status: "COMPLETE", CompleteCount: 1}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: belbuasTreasureQuestID, Kind: data.QuestCustom, StartNPC: belbuasTreasureNola, EndNPC: belbuasTreasureBarrel, NPCStart: true}
	nola := questCatalogNPC(s, p, belbuasTreasureNola, 0x31141)
	barrel := questCatalogNPC(s, p, belbuasTreasureBarrel, 0x31142)

	if !c.belbuasTreasureDialog(nola, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(nola.id, 1011, belbuasTreasureQuestID).Data) {
		t.Fatal("Nola did not offer the quest")
	}
	if !c.belbuasTreasureDialog(nola, script, 1002) || p.quest(belbuasTreasureQuestID) == nil || p.quest(belbuasTreasureQuestID).Status != "START" {
		t.Fatal("quest acceptance did not start Belbua's Treasure")
	}
	if c.belbuasTreasureDialog(questCatalogNPC(s, p, 730002, 0x31143), script, -1) || c.belbuasTreasureDialog(barrel, script, 25) {
		t.Fatal("wrong NPC or wrong barrel dialog advanced the quest")
	}
	if !c.belbuasTreasureDialog(barrel, script, -1) || p.quest(belbuasTreasureQuestID).Status != "REWARD" || p.quest(belbuasTreasureQuestID).Vars != 2 {
		t.Fatal("barrel discovery did not unlock the reward")
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(barrel.id, 5, belbuasTreasureQuestID).Data) {
		t.Fatalf("barrel reward dialog = %x", packets.last(smDialogWindow))
	}
	if !c.belbuasTreasureDialog(barrel, script, -1) || p.quest(belbuasTreasureQuestID).Vars != 2 {
		t.Fatal("repeated barrel inspection changed the quest")
	}
	beforeExp := p.Exp
	if !c.belbuasTreasureDialog(barrel, script, 17) {
		t.Fatal("reward selection did not complete the quest")
	}
	if q := p.quest(belbuasTreasureQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, 160000003) != 2 {
		t.Fatalf("reward mismatch: quest=%+v gained experience=%d reward items=%d", q, p.Exp-beforeExp, s.countItems(p, 160000003))
	}
	if c.belbuasTreasureDialog(barrel, script, 17) || p.quest(belbuasTreasureQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward selection completed the quest twice")
	}
}

func TestBelbuasTreasureRequiresPrerequisite(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Class = "SORCERER"
	p.Race = "ELYOS"
	p.level = 11
	p.seen = map[int32]*object{}
	c := &conn{s: s, player: p}
	script := &data.QuestScript{ID: belbuasTreasureQuestID, Kind: data.QuestCustom, StartNPC: belbuasTreasureNola, EndNPC: belbuasTreasureBarrel}
	nola := questCatalogNPC(s, p, belbuasTreasureNola, 0x32141)
	if c.belbuasTreasureDialog(nola, script, 25) || p.quest(belbuasTreasureQuestID) != nil {
		t.Fatal("quest was offered before Nola's Request was complete")
	}
}
