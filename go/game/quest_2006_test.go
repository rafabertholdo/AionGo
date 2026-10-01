package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestHitThemWhereItHurtsUnlockCollectReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.level, p.Exp = "ASMODIANS", 4, d.ExpStart(4)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	p.quests = []store.Quest{{ID: 2006, Status: "LOCKED"}}
	script := d.QuestScripts[2006]
	if script == nil {
		t.Fatal("quest 2006 unregistered")
	}
	if !c.hitThemWhereItHurtsLevelUp() || p.quest(2006).Status != "START" || c.hitThemWhereItHurtsLevelUp() {
		t.Fatal("level unlock")
	}
	npc := questCatalogNPC(s, p, 203540, 0x30001)
	end := questCatalogNPC(s, p, 203516, 0x30002)
	if !c.hitThemWhereItHurtsDialog(npc, script, 25) || !c.hitThemWhereItHurtsDialog(npc, script, 10000) || p.quest(2006).Vars != 1 {
		t.Fatal("dialog progress")
	}
	if !c.hitThemWhereItHurtsDialog(npc, script, 33) || p.quest(2006).Status != "START" {
		t.Fatal("missing-item branch")
	}
	if !s.addItem(p, 182203008, 7) {
		t.Fatal("could not grant collectible")
	}
	if !c.hitThemWhereItHurtsDialog(npc, script, 33) || p.quest(2006).Status != "REWARD" || s.countItems(p, 182203008) != 0 {
		t.Fatal("collection turn-in")
	}
	if !c.hitThemWhereItHurtsDialog(end, script, -1) {
		t.Fatal("reward preview")
	}
	before := p.Exp
	if !c.hitThemWhereItHurtsDialog(end, script, 8) || p.quest(2006).Status != "COMPLETE" || p.Exp-before != 5200 || s.countItems(p, 114100795) != 1 {
		t.Fatal("reward completion")
	}
	c.hitThemWhereItHurtsDialog(end, script, 8)
	if s.countItems(p, 114100795) != 1 {
		t.Fatal("repeat reward")
	}
}
