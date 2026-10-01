package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestGaphyrksLoveAttackMovieAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[gaphyrksLoveQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 14 || len(template.FinishedQuestConditions) != 1 ||
		template.FinishedQuestConditions[0] != 1156 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 15500 {
		t.Fatalf("unexpected Gaphyrk's Love metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 14
	p.WorldID, p.X, p.Y, p.Z = 210030000, 892, 2024, 166
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.quests = []store.Quest{{ID: 1156, Status: "COMPLETE"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: gaphyrksLoveQuestID, Kind: data.QuestCustom, StartNPC: 798003, EndNPC: 798003, NPCStart: true}
	gaphyrk := questCatalogNPC(s, p, 798003, 0x61157)
	if !c.gaphyrksLoveDialog(gaphyrk, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(gaphyrk.id, 1011, gaphyrksLoveQuestID).Data) {
		t.Fatalf("quest offer = %x", packets.last(smDialogWindow))
	}
	if !c.gaphyrksLoveDialog(gaphyrk, script, 1002) || p.quest(gaphyrksLoveQuestID) == nil || p.quest(gaphyrksLoveQuestID).Status != "START" {
		t.Fatalf("quest did not start: %+v", p.quest(gaphyrksLoveQuestID))
	}
	if c.gaphyrksLoveDialog(gaphyrk, script, 1002) {
		t.Fatal("quest start repeated")
	}

	npcTemplate := d.Npcs[210319]
	if npcTemplate == nil {
		t.Fatal("Gaphyrk's Love scout template missing")
	}
	scout := &object{id: 0x61159, worldID: p.WorldID, instance: p.instance, x: 1000, y: 2024, z: 166,
		homeX: 892, homeY: 2024, homeZ: 166, interval: 60, npc: npcTemplate, watchers: map[int32]*player{p.ID: p}}
	s.initNpc(scout)
	p.seen[scout.id] = scout
	s.byID[scout.id] = scout
	s.addObject(scout)
	if c.gaphyrksLoveAttack(scout) || p.quest(gaphyrksLoveQuestID).Status != "START" {
		t.Fatal("out-of-range scout attack triggered the event")
	}
	scout.x = 892
	if !c.gaphyrksLoveAttack(scout) || !bytes.Equal(packets.last(smPlayMovie), gaphyrksLoveMovie(17).Data) || p.quest(gaphyrksLoveQuestID).Status != "START" {
		t.Fatalf("marked scout attack result: quest=%+v movie=%x", p.quest(gaphyrksLoveQuestID), packets.last(smPlayMovie))
	}
	if c.gaphyrksLoveAttack(scout) {
		t.Fatal("repeated attack event retriggered while scout was despawned")
	}
	if c.gaphyrksLoveMovieEnd(16) || p.quest(gaphyrksLoveQuestID).Status != "START" {
		t.Fatal("unrelated movie ended the quest")
	}
	if !c.gaphyrksLoveMovieEnd(17) || p.quest(gaphyrksLoveQuestID).Status != "REWARD" || c.gaphyrksLoveMovieEnd(17) {
		t.Fatalf("movie completion did not transition once: %+v", p.quest(gaphyrksLoveQuestID))
	}
	if !c.gaphyrksLoveShowDialog(gaphyrk, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(gaphyrk.id, 2375, gaphyrksLoveQuestID).Data) {
		t.Fatal("reward preview failed")
	}
	if !c.gaphyrksLoveDialog(gaphyrk, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(gaphyrk.id, 5, gaphyrksLoveQuestID).Data) {
		t.Fatal("reward confirmation page failed")
	}
	beforeExp := p.Exp
	if !c.gaphyrksLoveDialog(gaphyrk, script, 17) || p.quest(gaphyrksLoveQuestID).Status != "COMPLETE" || p.Exp-beforeExp != template.Rewards[0].Experience {
		t.Fatalf("reward mismatch: %+v exp=%d", p.quest(gaphyrksLoveQuestID), p.Exp-beforeExp)
	}
	if c.gaphyrksLoveDialog(gaphyrk, script, 17) || p.quest(gaphyrksLoveQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward dialog completed the quest twice")
	}
}
