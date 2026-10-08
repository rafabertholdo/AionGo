package game

import (
	"bytes"
	"testing"

	"aionlightning/game/store"
)

func TestNochsanaGeneralQuestGateThenGeneralThenReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.level = "ELYOS", 25
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: 182400001, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := d.QuestScripts[3702]
	if script == nil {
		t.Fatal("quest 3702 has no handler")
	}
	nereus := questCatalogNPC(s, p, 278517, 0x31702)

	if c.nochsanaGeneralQuestDialog(nereus, script, -1) {
		t.Fatal("a plain click answered instead of the main menu")
	}
	if !c.nochsanaGeneralQuestDialog(nereus, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(nereus.id, 1011, 3702).Data) {
		t.Fatal("the start page did not show")
	}
	c.nochsanaGeneralQuestDialog(nereus, script, 1002)
	if q := p.quest(3702); q == nil || q.Status != "START" || s.countItems(p, 182202179) != 1 {
		t.Fatalf("accepting did not start the quest and hand over the siege weapon: %+v", p.quest(3702))
	}

	general := &object{worldID: nochsanaWorld, npc: d.Npcs[nochsanaGeneral]}
	c.nochsanaGeneralKill(3702, general)
	if p.quest(3702).Status != "START" {
		t.Fatal("the General counted before the gate fell")
	}
	gate := &object{worldID: nochsanaWorld, instance: 7, npc: d.Npcs[nochsanaGate]}
	p.WorldID, p.instance = nochsanaWorld, 8
	s.nochsanaGeneralGateCredit(gate)
	if questVar(p.quest(3702).Vars, 0) != 0 {
		t.Fatal("a gate in another run gave credit")
	}
	p.instance = 7
	s.nochsanaGeneralGateCredit(gate)
	if questVar(p.quest(3702).Vars, 0) != 1 {
		t.Fatal("the gate falling did not give its step")
	}
	c.nochsanaGeneralKill(3702, general)
	if p.quest(3702).Status != "REWARD" {
		t.Fatal("the General's death did not ready the reward")
	}
	if !c.nochsanaGeneralQuestDialog(nereus, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(nereus.id, 5, 3702).Data) {
		t.Fatal("Nereus did not offer the reward")
	}
	c.nochsanaGeneralQuestDialog(nereus, script, 8)
	if p.quest(3702).Status != "COMPLETE" {
		t.Fatalf("choosing a reward did not complete the quest: %+v", p.quest(3702))
	}
}
