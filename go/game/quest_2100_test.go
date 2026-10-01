package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestOrderOfTheCaptainZoneDialogAndLockedFollowUps(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 1
	p.Exp = d.ExpStart(1)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	c := &conn{s: s, player: p, tap: (&questPackets{}).tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2100, Kind: data.QuestCustom, StartNPC: 203516, EndNPC: 203516}
	npc := questCatalogNPC(s, p, 203516, 0x30001)
	other := questCatalogNPC(s, p, 203533, 0x30002)
	if c.orderOfTheCaptainEnterZone("OTHER") || p.quest(2100) != nil {
		t.Fatal("unrelated zone started quest")
	}
	if !c.orderOfTheCaptainEnterZone("ALDELLE_VILLAGE") || p.quest(2100).Status != "START" {
		t.Fatal("village entry did not start quest")
	}
	if c.orderOfTheCaptainEnterZone("ALDELLE_VILLAGE") {
		t.Fatal("repeated entry restarted quest")
	}
	c.orderOfTheCaptainDialog(other, script, 25)
	if p.quest(2100).Status != "START" {
		t.Fatal("wrong NPC advanced quest")
	}
	c.orderOfTheCaptainDialog(npc, script, 17)
	if p.quest(2001) != nil {
		t.Fatal("early reward locked follow-ups")
	}
	c.orderOfTheCaptainDialog(npc, script, 25)
	if q := p.quest(2100); q.Status != "REWARD" || q.Vars != 1 {
		t.Fatalf("dialog transition = %+v", q)
	}
	beforeExp := p.Exp
	c.orderOfTheCaptainDialog(npc, script, 17)
	if q := p.quest(2100); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != 10 {
		t.Fatalf("completion = %+v, exp = %d", q, p.Exp-beforeExp)
	}
	for id := int32(2001); id <= 2007; id++ {
		want := "LOCKED"
		if id == 2001 || id == 2002 {
			want = "START"
		}
		if q := p.quest(id); q == nil || q.Status != want || q.Vars != 0 {
			t.Fatalf("follow-up %d = %+v", id, q)
		}
	}
	c.orderOfTheCaptainDialog(npc, script, 17)
	if p.quest(2100).CompleteCount != 1 || p.Exp-beforeExp != 10 {
		t.Fatal("repeat reward changed completion")
	}
}

func TestOrderOfTheCaptainRejectsWrongRace(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	c := &conn{s: s, player: p}
	p.conn = c
	if c.orderOfTheCaptainEnterZone("ALDELLE_VILLAGE") || p.quest(2100) != nil {
		t.Fatal("wrong race started quest")
	}
}
