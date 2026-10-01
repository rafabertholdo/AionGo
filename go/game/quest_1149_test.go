package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestMissingPoppyStartAndEscort(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[missingPoppyQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 14 || len(template.Rewards) != 1 ||
		template.Rewards[0].Experience != 30400 || template.Rewards[0].Kinah != 1500 {
		t.Fatalf("unexpected Missing Poppy metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 14
	p.WorldID, p.X, p.Y, p.Z = 210030000, 1000, 1000, 144
	p.Exp = d.ExpStart(p.level)
	p.Exp = d.ExpStart(p.level)
	p.seen = map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: missingPoppyQuestID, Kind: data.QuestCustom, StartNPC: missingPoppyStartNPC,
		EndNPC: missingPoppyStartNPC, NPCStart: true, TalkNPCs: []int32{missingPoppyNPC}}
	client := questCatalogNPC(s, p, missingPoppyStartNPC, 0x61149)
	if !c.missingPoppyDialog(client, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(client.id, 1011, missingPoppyQuestID).Data) {
		t.Fatalf("quest offer = %x", packets.last(smDialogWindow))
	}
	if !c.missingPoppyDialog(client, script, 1002) || p.quest(missingPoppyQuestID) == nil || p.quest(missingPoppyQuestID).Status != "START" {
		t.Fatalf("quest did not start: %+v", p.quest(missingPoppyQuestID))
	}
	if c.missingPoppyDialog(client, script, 1002) {
		t.Fatal("repeated quest start was accepted")
	}
	poppy := &object{id: 0x61150, worldID: p.WorldID, instance: p.instance, x: 1002, y: 1000, z: 144,
		npc: d.Npcs[missingPoppyNPC]}
	if poppy.npc == nil {
		t.Fatal("Missing Poppy NPC template missing")
	}
	s.initNpc(poppy)
	s.byID[poppy.id] = poppy
	s.addObject(poppy)
	p.seen[poppy.id] = poppy
	if !c.missingPoppyDialog(poppy, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(poppy.id, 1352, missingPoppyQuestID).Data) {
		t.Fatalf("Poppy's opening page = %x", packets.last(smDialogWindow))
	}
	if c.missingPoppyDialog(client, script, 10000) {
		t.Fatal("quest starter advanced Poppy's escort")
	}
	if !c.missingPoppyDialog(poppy, script, 10000) || questVar(p.quest(missingPoppyQuestID).Vars, 0) != 1 ||
		!poppy.move.follow || poppy.targetID != p.ID || !poppy.move.scheduled() {
		t.Fatalf("escort did not start: quest=%+v move=%+v target=%d", p.quest(missingPoppyQuestID), poppy.move, poppy.targetID)
	}
	// Stop the simulated movement before placing Poppy at the Java destination.
	poppy.move.stop()
	poppy.x, poppy.y, poppy.z = 1255, 2223, 144
	if !c.missingPoppyShowDialog(poppy, script) || p.quest(missingPoppyQuestID).Status != "REWARD" ||
		!bytes.Equal(packets.last(smPlayMovie), missingPoppyMovie().Data) {
		t.Fatalf("Poppy rescue did not complete: quest=%+v movie=%x", p.quest(missingPoppyQuestID), packets.last(smPlayMovie))
	}
	if c.missingPoppyShowDialog(poppy, script) {
		t.Fatal("rescued Poppy accepted a repeated escort event")
	}
	beforeExp, beforeKinah := p.Exp, p.kinah.Count
	if !c.missingPoppyDialog(client, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(client.id, 5, missingPoppyQuestID).Data) {
		t.Fatal("reward page failed")
	}
	if !c.missingPoppyDialog(client, script, 8) || p.quest(missingPoppyQuestID).Status != "COMPLETE" ||
		p.Exp-beforeExp != template.Rewards[0].Experience || p.kinah.Count-beforeKinah != template.Rewards[0].Kinah {
		t.Fatalf("quest reward mismatch: %+v exp=%d kinah=%d", p.quest(missingPoppyQuestID), p.Exp-beforeExp, p.kinah.Count-beforeKinah)
	}
	if c.missingPoppyDialog(client, script, 8) || p.quest(missingPoppyQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward dialog completed twice")
	}
}
