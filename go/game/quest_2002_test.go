package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestWheresRaeDialogKillsAndCollection(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2002, Kind: data.QuestCustom, EndNPC: 203516}
	makeNPC := func(id int32) *object { return questCatalogNPC(s, p, id, id+100000) }
	start, elder, rae, stone := makeNPC(203519), makeNPC(203534), makeNPC(790002), makeNPC(700045)
	killA, killB := makeNPC(210377), makeNPC(210378)
	if c.wheresRaeLevelUp() {
		t.Fatal("uncreated quest unlocked")
	}
	q := store.Quest{ID: 2002, Status: "LOCKED"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.wheresRaeEvent(start, script, 25, false) || !c.wheresRaeLevelUp() || c.wheresRaeLevelUp() {
		t.Fatal("level-up")
	}
	if !c.wheresRaeEvent(start, script, 25, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, 2002).Data) {
		t.Fatal("start page")
	}
	if !c.wheresRaeEvent(start, script, 10000, false) || !c.wheresRaeEvent(elder, script, 25, false) {
		t.Fatal("first handoff")
	}
	c.wheresRaeEvent(elder, script, 1353, false)
	if !bytes.Equal(packets.last(smPlayMovie), wheresRaeMovie(52).Data) {
		t.Fatal("movie 52")
	}
	if !c.wheresRaeEvent(elder, script, 10001, false) || !c.wheresRaeEvent(rae, script, 25, false) || !c.wheresRaeEvent(rae, script, 10002, false) {
		t.Fatal("second handoff")
	}
	if c.wheresRaeEvent(rae, script, 10002, false) || c.wheresRaeEvent(start, script, 10000, false) {
		t.Fatal("repeat advanced")
	}
	for i := 0; i < 7; i++ {
		target := killA
		if i%2 != 0 {
			target = killB
		}
		if !c.wheresRaeEvent(target, script, 0, true) {
			t.Fatalf("kill %d", i)
		}
	}
	if p.quest(2002).Vars != 10 || c.wheresRaeEvent(killA, script, 0, true) {
		t.Fatal("kill cap")
	}
	if !c.wheresRaeEvent(rae, script, 25, false) || !c.wheresRaeEvent(rae, script, 10003, false) {
		t.Fatal("third handoff")
	}
	if !c.wheresRaeEvent(rae, script, 33, false) || p.quest(2002).Vars != 11 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(rae.id, 2376, 2002).Data) {
		t.Fatal("missing item")
	}
	if !c.wheresRaeEvent(stone, script, -1, false) || packets.last(smUseObject) == nil {
		t.Fatal("object use")
	}
	if !s.addItem(p, 182203003, 1) {
		t.Fatal("add collection item")
	}
	if !c.wheresRaeEvent(rae, script, 33, false) || p.quest(2002).Vars != 12 || s.countItems(p, 182203003) != 0 {
		t.Fatal("collect")
	}
	if c.wheresRaeEvent(rae, script, 33, false) {
		t.Fatal("repeat collection")
	}
}

func TestWheresRaeFinalDialogAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 3
	p.Exp = d.ExpStart(3)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2002, Kind: data.QuestCustom, EndNPC: 203516}
	q := store.Quest{ID: 2002, Status: "START", Vars: 14}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	dead := questCatalogNPC(s, p, 203537, 0x30001)
	s.initNpc(dead)
	newRae := questCatalogNPC(s, p, 203553, 0x30002)
	s.initNpc(newRae)
	end := questCatalogNPC(s, p, 203516, 0x30003)
	if !c.wheresRaeEvent(dead, script, -1, false) || p.quest(2002).Vars != 15 || !bytes.Equal(packets.last(smPlayMovie), wheresRaeMovie(256).Data) {
		t.Fatal("Rae reveal")
	}
	if c.wheresRaeEvent(dead, script, -1, false) || !c.wheresRaeEvent(newRae, script, 25, false) {
		t.Fatal("reveal repeat or dialog")
	}
	if !c.wheresRaeEvent(newRae, script, 10006, false) || p.quest(2002).Status != "REWARD" {
		t.Fatal("reward state")
	}
	if !c.wheresRaeEvent(end, script, -1, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 3398, 2002).Data) {
		t.Fatal("reward page")
	}
	if !c.wheresRaeEvent(end, script, 10007, false) {
		t.Fatal("reward choice page")
	}
	before := p.Exp
	if !c.wheresRaeEvent(end, script, 8, false) || p.quest(2002).Status != "COMPLETE" || p.Exp-before != 4450 || s.countItems(p, 100200604) != 1 {
		t.Fatal("reward completion")
	}
	if c.wheresRaeEvent(end, script, 8, false) || s.countItems(p, 100200604) != 1 {
		t.Fatal("reward repeat")
	}
}
