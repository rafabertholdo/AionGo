package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestWheresTuttyZoneAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script := &data.QuestScript{ID: 1123, Kind: data.QuestCustom, StartNPC: 790001, EndNPC: 790001}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 7
	p.Exp = d.ExpStart(7)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	pernos := questCatalogNPC(s, p, 790001, 0x30001)

	c.wheresTuttyDialog(pernos, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(pernos.id, 1011, 1123).Data) {
		t.Fatalf("offer = %x", got)
	}
	c.wheresTuttyDialog(pernos, script, 1002)
	if q := p.quest(1123); q == nil || q.Status != "START" {
		t.Fatalf("start = %+v", q)
	}
	if c.wheresTuttyEnterZone("OTHER") || p.quest(1123).Status != "START" {
		t.Fatal("unrelated zone advanced quest")
	}
	if !c.wheresTuttyEnterZone("Q1123") {
		t.Fatal("Q1123 did not advance quest")
	}
	if q := p.quest(1123); q.Status != "REWARD" || q.Vars != 0 {
		t.Fatalf("zone state = %+v", q)
	}
	if got := packets.last(smPlayMovie); !bytes.Equal(got, wheresTuttyMovie().Data) {
		t.Fatalf("zone movie = %x", got)
	}
	if c.wheresTuttyEnterZone("Q1123") {
		t.Fatal("repeated entry replayed movie")
	}
	c.wheresTuttyDialog(pernos, script, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(pernos.id, 5, 1123).Data) {
		t.Fatalf("reward dialog = %x", got)
	}
	beforeExp := p.Exp
	c.wheresTuttyDialog(pernos, script, 17)
	if q := p.quest(1123); q.Status != "COMPLETE" || q.CompleteCount != 1 {
		t.Fatalf("completion = %+v", q)
	}
	if p.Exp-beforeExp != 4150 || s.countItems(p, 162000017) != 2 {
		t.Fatalf("reward = exp %d, items %d", p.Exp-beforeExp, s.countItems(p, 162000017))
	}
	c.wheresTuttyDialog(pernos, script, 17)
	if p.quest(1123).CompleteCount != 1 || s.countItems(p, 162000017) != 2 {
		t.Fatal("repeated completion paid reward again")
	}
}

func TestWheresTuttyRejectsWrongNPCAndInactiveZone(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.seen = map[int32]*object{}
	c := &conn{s: s, player: p}
	p.conn = c
	script := &data.QuestScript{ID: 1123, Kind: data.QuestCustom, StartNPC: 790001, EndNPC: 790001}
	other := questCatalogNPC(s, p, 203067, 0x30002)
	c.wheresTuttyDialog(other, script, 1002)
	if p.quest(1123) != nil || c.wheresTuttyEnterZone("Q1123") {
		t.Fatal("inactive quest advanced")
	}
}
