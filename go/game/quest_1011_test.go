package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestDangerFromAboveUnlocksOnlyAfterCitadelSummons(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 10
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	p.quests = []store.Quest{{ID: 1011, Status: "LOCKED"}}
	if c.dangerFromAboveLevelUp() {
		t.Fatal("quest unlocked without completing Summons to the Citadel")
	}
	p.quests = append(p.quests, store.Quest{ID: 1130, Status: "COMPLETE", CompleteCount: 1})
	if !c.dangerFromAboveLevelUp() || p.quest(1011).Status != "START" || c.dangerFromAboveLevelUp() {
		t.Fatalf("unlock state = %+v", p.quest(1011))
	}
	if got := packets.last(smQuestAccepted); got == nil {
		t.Fatal("quest unlock packet was not sent")
	}
}

func TestDangerFromAboveDialogKillsAndClassReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script := &data.QuestScript{ID: 1011, Kind: data.QuestCustom, StartNPC: 203109, EndNPC: 203109}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 10
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	start := questCatalogNPC(s, p, 203109, 0x31001)
	middle := questCatalogNPC(s, p, 203122, 0x31002)
	other := questCatalogNPC(s, p, 203123, 0x31003)
	monster := &object{id: 0x31004, npc: &data.NpcTemplate{ID: 700091}}
	wrongMonster := &object{id: 0x31005, npc: &data.NpcTemplate{ID: 700092}}
	q := store.Quest{ID: 1011, Status: "START"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)

	if c.dangerFromAboveDialog(other, script, 25) || c.dangerFromAboveDialog(middle, script, 10001) {
		t.Fatal("invalid NPC or out-of-order dialog was accepted")
	}
	if !c.dangerFromAboveDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, 1011).Data) {
		t.Fatal("start dialog page missing")
	}
	if !c.dangerFromAboveDialog(start, script, 10000) || p.quest(1011).Vars != 1 || c.dangerFromAboveDialog(start, script, 10000) {
		t.Fatalf("start dialog did not advance once: %+v", p.quest(1011))
	}
	if !c.dangerFromAboveDialog(middle, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(middle.id, 1352, 1011).Data) {
		t.Fatal("middle dialog page missing")
	}
	if c.dangerFromAboveDialog(middle, script, 1353) || !bytes.Equal(packets.last(smPlayMovie), playMovie(24).Data) {
		t.Fatal("middle NPC movie was not sent with Java's event return behavior")
	}
	if !c.dangerFromAboveDialog(middle, script, 10001) || p.quest(1011).Vars != 2 || c.dangerFromAboveDialog(middle, script, 10001) {
		t.Fatalf("middle dialog did not advance once: %+v", p.quest(1011))
	}
	if c.dangerFromAboveKill(wrongMonster) || c.dangerFromAboveKill(nil) || p.quest(1011).Vars != 2 {
		t.Fatal("wrong kill target advanced quest")
	}
	for expected := int32(3); expected <= 4; expected++ {
		if !c.dangerFromAboveKill(monster) || p.quest(1011).Vars != expected || p.quest(1011).Status != "START" {
			t.Fatalf("kill progress %d = %+v", expected, p.quest(1011))
		}
	}
	if !c.dangerFromAboveKill(monster) || p.quest(1011).Status != "REWARD" || p.quest(1011).Vars != 4 {
		t.Fatalf("final kill did not open reward state: %+v", p.quest(1011))
	}
	if c.dangerFromAboveKill(monster) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(monster.id, 10, 0).Data) {
		t.Fatal("completed objective accepted a repeated kill or omitted its completion window")
	}
	if c.dangerFromAboveDialog(middle, script, -1) || !c.dangerFromAboveDialog(start, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1693, 1011).Data) {
		t.Fatal("reward preview was not restricted to the report NPC")
	}
	if !c.dangerFromAboveDialog(start, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 5, 1011).Data) {
		t.Fatal("reward choice page missing")
	}
	beforeExp := p.Exp
	if !c.dangerFromAboveDialog(start, script, 8) || p.quest(1011).Status != "COMPLETE" || p.Exp-beforeExp != 7340 || s.countItems(p, 162000024) != 4 || s.countItems(p, 169500008) != 1 {
		t.Fatalf("wrong class reward: quest=%+v exp=%d items=%d/%d", p.quest(1011), p.Exp-beforeExp, s.countItems(p, 162000024), s.countItems(p, 169500008))
	}
	if c.dangerFromAboveDialog(start, script, 8) || s.countItems(p, 162000024) != 4 || s.countItems(p, 169500008) != 1 {
		t.Fatal("repeated reward duplicated quest items")
	}
}
