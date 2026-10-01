package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestNeutralizingOdiumProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 3
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: 1004, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1004, Kind: data.QuestCustom, EndNPC: 203067}
	start := questCatalogNPC(s, p, 203082, 0x41001)
	odium := questCatalogNPC(s, p, 700030, 0x41002)
	worker := questCatalogNPC(s, p, 790001, 0x41003)
	if c.neutralizingOdiumLevelUp() {
		t.Fatal("unlocked below level 4")
	}
	p.level = 4
	if !c.neutralizingOdiumLevelUp() || c.neutralizingOdiumLevelUp() {
		t.Fatal("level unlock failed or repeated")
	}
	c.neutralizingOdiumDialog(start, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(start.id, 1011, 1004).Data) {
		t.Fatalf("opening page = %x", got)
	}
	c.neutralizingOdiumDialog(start, script, 1013)
	if got := packets.last(smPlayMovie); !bytes.Equal(got, neutralizingOdiumMovie().Data) {
		t.Fatalf("movie = %x", got)
	}
	c.neutralizingOdiumDialog(start, script, 10000)
	c.neutralizingOdiumDialog(start, script, 10000)
	if p.quest(1004).Vars != 1 {
		t.Fatalf("start variable = %d", p.quest(1004).Vars)
	}
	p.targetID = odium.id
	c.neutralizingOdiumDialog(odium, script, -1)
	if got := packets.last(smUseObject); !bytes.Equal(got, useObject(p.ID, odium.id, 1).Data) {
		t.Fatalf("odium use = %x", got)
	}
	c.neutralizingOdiumDialog(odium, script, -1)
	time.Sleep(3200 * time.Millisecond)
	if p.quest(1004).Vars != 2 || s.countItems(p, 182200005) != 1 {
		t.Fatalf("first use: quest=%+v item=%d", p.quest(1004), s.countItems(p, 182200005))
	}
	c.neutralizingOdiumDialog(worker, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(worker.id, 1352, 1004).Data) {
		t.Fatalf("worker page = %x", got)
	}
	c.neutralizingOdiumDialog(worker, script, 10001)
	if p.quest(1004).Vars != 3 {
		t.Fatalf("worker variable = %d", p.quest(1004).Vars)
	}
	c.neutralizingOdiumDialog(worker, script, 33)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(worker.id, 1779, 1004).Data) {
		t.Fatalf("missing items page = %x", got)
	}
	if !s.addItem(p, 182200004, 3) {
		t.Fatal("could not add collected items")
	}
	c.neutralizingOdiumDialog(worker, script, 33)
	if p.quest(1004).Vars != 11 || s.countItems(p, 182200004) != 0 {
		t.Fatalf("collection: quest=%+v item=%d", p.quest(1004), s.countItems(p, 182200004))
	}
	c.neutralizingOdiumDialog(worker, script, 10002)
	if p.quest(1004).Vars != 4 || s.countItems(p, 182200005) != 0 || s.countItems(p, 182200006) != 1 {
		t.Fatalf("exchange: quest=%+v old=%d new=%d", p.quest(1004), s.countItems(p, 182200005), s.countItems(p, 182200006))
	}
	c.neutralizingOdiumDialog(odium, script, -1)
	time.Sleep(3200 * time.Millisecond)
	if p.quest(1004).Vars != 5 || s.countItems(p, 182200006) != 0 {
		t.Fatalf("second use: quest=%+v item=%d", p.quest(1004), s.countItems(p, 182200006))
	}
	c.neutralizingOdiumDialog(start, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(start.id, 2034, 1004).Data) {
		t.Fatalf("return page = %x", got)
	}
	c.neutralizingOdiumDialog(start, script, 10002)
	if p.quest(1004).Status != "REWARD" {
		t.Fatalf("reward state = %+v", p.quest(1004))
	}
}

func TestNeutralizingOdiumReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x41005, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1004, Status: "REWARD", Vars: 5}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1004, Kind: data.QuestCustom, EndNPC: 203067}
	end := questCatalogNPC(s, p, 203067, 0x41004)
	c.neutralizingOdiumDialog(end, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(end.id, 5, 1004).Data) {
		t.Fatalf("reward page = %x", got)
	}
	before := p.Exp
	c.neutralizingOdiumDialog(end, script, 8)
	if q := p.quest(1004); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-before != 6970 || s.countItems(p, 113100781) != 1 {
		t.Fatalf("reward: quest=%+v exp=%d item=%d", q, p.Exp-before, s.countItems(p, 113100781))
	}
	c.neutralizingOdiumDialog(end, script, 8)
	if s.countItems(p, 113100781) != 1 {
		t.Fatal("reward duplicated")
	}
}
