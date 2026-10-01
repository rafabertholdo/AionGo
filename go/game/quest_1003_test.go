package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestIllegalLoggingProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.level = 3
	p.Race = "ELYOS"
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1003, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1003, Kind: data.QuestCustom, EndNPC: 203081}
	npc := questCatalogNPC(s, p, 203081, 0x31003)
	if c.illegalLoggingKill(210096) {
		t.Fatal("locked quest accepted kill")
	}
	p.level = 2
	if c.illegalLoggingLevelUp() {
		t.Fatal("unlocked below minimum level")
	}
	p.level = 3
	if !c.illegalLoggingLevelUp() || c.illegalLoggingLevelUp() {
		t.Fatal("level transition failed or repeated")
	}
	c.illegalLoggingDialog(npc, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1011, 1003).Data) {
		t.Fatalf("initial dialog = %x", got)
	}
	if c.illegalLoggingKill(210160) || c.illegalLoggingKill(210096) {
		t.Fatal("kill advanced before initial dialog")
	}
	c.illegalLoggingDialog(npc, script, 10000)
	if got := questVar(p.quest(1003).Vars, 0); got != 1 {
		t.Fatalf("first dialog var = %d", got)
	}
	c.illegalLoggingDialog(npc, script, 10000)
	if got := questVar(p.quest(1003).Vars, 0); got != 1 {
		t.Fatal("repeated dialog advanced")
	}
	if c.illegalLoggingKill(210160) || c.illegalLoggingKill(210001) {
		t.Fatal("wrong first-phase kill advanced")
	}
	mobs := []int32{210096, 210149, 210145, 210146, 210150, 210151, 210092, 210154}
	for i := 0; i < 12; i++ {
		if !c.illegalLoggingKill(mobs[i%len(mobs)]) {
			t.Fatalf("first-phase kill %d failed", i)
		}
	}
	if got := questVar(p.quest(1003).Vars, 0); got != 13 || c.illegalLoggingKill(210096) {
		t.Fatalf("first-phase boundary = %d", got)
	}
	c.illegalLoggingDialog(npc, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1352, 1003).Data) {
		t.Fatalf("second dialog = %x", got)
	}
	c.illegalLoggingDialog(npc, script, 10001)
	if got := questVar(p.quest(1003).Vars, 0); got != 14 {
		t.Fatalf("second-phase var = %d", got)
	}
	if c.illegalLoggingKill(210096) {
		t.Fatal("first-phase mob advanced second phase")
	}
	for i := 0; i < 3; i++ {
		if !c.illegalLoggingKill(210160) {
			t.Fatalf("second-phase kill %d failed", i)
		}
	}
	if q := p.quest(1003); q.Status != "REWARD" || questVar(q.Vars, 0) != 16 || c.illegalLoggingKill(210160) {
		t.Fatalf("reward transition = %+v", q)
	}
}

func TestIllegalLoggingReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 3
	p.Exp = d.ExpStart(3)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1003, Status: "REWARD", Vars: 16}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1003, Kind: data.QuestCustom, EndNPC: 203081}
	npc := questCatalogNPC(s, p, 203081, 0x31003)
	c.illegalLoggingDialog(npc, script, -1)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 5, 1003).Data) {
		t.Fatalf("reward dialog = %x", got)
	}
	beforeExp := p.Exp
	c.illegalLoggingDialog(npc, script, 8)
	if q := p.quest(1003); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != 4160 || s.countItems(p, 114100807) != 1 {
		t.Fatalf("completion = %+v, experience=%d, item=%d", q, p.Exp-beforeExp, s.countItems(p, 114100807))
	}
	c.illegalLoggingDialog(npc, script, 8)
	if p.quest(1003).CompleteCount != 1 || s.countItems(p, 114100807) != 1 {
		t.Fatal("repeated finish granted reward")
	}
}
