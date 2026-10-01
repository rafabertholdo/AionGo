package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestRequestOfTheElimLockedAndConversation(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1002, Kind: data.QuestCustom, EndNPC: 203067}
	if c.requestOfTheElimLevelUp() {
		t.Fatal("missing locked quest was started")
	}
	p.quests = append(p.quests, store.Quest{ID: 1002, Status: "LOCKED"})
	if !c.requestOfTheElimLevelUp() || p.quest(1002).Status != "START" || c.requestOfTheElimLevelUp() {
		t.Fatalf("unlock state = %+v", p.quest(1002))
	}
	primer := questCatalogNPC(s, p, 203076, 0x30001)
	c.requestOfTheElimDialog(primer, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(primer.id, 1011, 1002).Data) {
		t.Fatalf("primer page = %x", got)
	}
	c.requestOfTheElimDialog(primer, script, 10000)
	if p.quest(1002).Vars != 1 {
		t.Fatalf("primer progress = %+v", p.quest(1002))
	}
	c.requestOfTheElimDialog(primer, script, 10000)
	if p.quest(1002).Vars != 1 {
		t.Fatal("repeated primer dialog advanced")
	}
	elim := questCatalogNPC(s, p, 730007, 0x30002)
	c.requestOfTheElimDialog(elim, script, 1353)
	if packets.last(smPlayMovie) == nil {
		t.Fatal("movie 20 was not sent")
	}
	c.requestOfTheElimDialog(elim, script, 10001)
	if p.quest(1002).Vars != 2 || s.countItems(p, 182200002) != 1 {
		t.Fatalf("Elim loan = %+v, items=%d", p.quest(1002), s.countItems(p, 182200002))
	}
	c.requestOfTheElimDialog(elim, script, 10001)
	if s.countItems(p, 182200002) != 1 {
		t.Fatal("repeated loan duplicated the item")
	}
}

func TestRequestOfTheElimCollectionAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1002, Kind: data.QuestCustom, EndNPC: 203067}
	p.quests = append(p.quests, store.Quest{ID: 1002, Status: "START", Vars: 6})
	elim := questCatalogNPC(s, p, 730007, 0x30002)
	c.requestOfTheElimDialog(elim, script, 33)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(elim.id, 2205, 1002).Data) {
		t.Fatalf("missing collect items page = %x", got)
	}
	if !s.addItem(p, 182200003, 6) {
		t.Fatal("could not add collected items")
	}
	c.requestOfTheElimDialog(elim, script, 33)
	if p.quest(1002).Vars != 12 || s.countItems(p, 182200003) != 0 {
		t.Fatalf("collection = %+v, items=%d", p.quest(1002), s.countItems(p, 182200003))
	}
	c.requestOfTheElimDialog(elim, script, 10003)
	if p.quest(1002).Vars != 13 {
		t.Fatalf("Elim exit = %+v", p.quest(1002))
	}
	portal := questCatalogNPC(s, p, 730008, 0x30003)
	c.requestOfTheElimDialog(portal, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(portal.id, 2375, 1002).Data) {
		t.Fatalf("portal page = %x", got)
	}
	p.quest(1002).Vars = 14
	c.requestOfTheElimDialog(portal, script, 10005)
	if p.quest(1002).Status != "REWARD" {
		t.Fatalf("reward status = %+v", p.quest(1002))
	}
	kalio := questCatalogNPC(s, p, 203067, 0x30004)
	c.requestOfTheElimDialog(kalio, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 2716, 1002).Data) {
		t.Fatalf("reward preview = %x", got)
	}
	c.requestOfTheElimDialog(kalio, script, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 5, 1002).Data) {
		t.Fatalf("reward choice page = %x", got)
	}
	before := p.Exp
	c.requestOfTheElimDialog(kalio, script, 8)
	if p.quest(1002).Status != "COMPLETE" || p.Exp-before != 2830 || s.countItems(p, 100200613) != 1 {
		t.Fatalf("reward = %+v, exp=%d, item=%d", p.quest(1002), p.Exp-before, s.countItems(p, 100200613))
	}
	c.requestOfTheElimDialog(kalio, script, 8)
	if s.countItems(p, 100200613) != 1 {
		t.Fatal("reward duplicated")
	}
}

func TestRequestOfTheElimSaplingsAndMorph(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1002, Kind: data.QuestCustom}
	p.quests = append(p.quests, store.Quest{ID: 1002, Status: "START", Vars: 2})
	if c.requestOfTheElimEnterWorld() {
		t.Fatal("morphed outside trial instance")
	}
	p.WorldID = 310010000
	if !c.requestOfTheElimEnterWorld() || packets.last(smAscensionMorph) == nil {
		t.Fatal("trial entry omitted morph")
	}
	for _, want := range []int32{4, 5} {
		sapling := questCatalogNPC(s, p, 730010, 0x30000+want)
		c.requestOfTheElimDialog(sapling, script, 25)
		if p.quest(1002).Vars != want || !sapling.dead {
			t.Fatalf("sapling progress = %+v, dead=%v", p.quest(1002), sapling.dead)
		}
		c.requestOfTheElimDialog(sapling, script, 25)
		if p.quest(1002).Vars != want {
			t.Fatal("dead sapling advanced again")
		}
	}
	elim := questCatalogNPC(s, p, 730007, 0x30010)
	if !s.addItem(p, 182200002, 1) {
		t.Fatal("could not add loan item")
	}
	c.requestOfTheElimDialog(elim, script, 25)
	if s.countItems(p, 182200002) != 0 {
		t.Fatal("Elim did not reclaim loan item")
	}
	c.requestOfTheElimDialog(elim, script, 10002)
	if p.quest(1002).Vars != 6 {
		t.Fatalf("Elim collection state = %+v", p.quest(1002))
	}
}
