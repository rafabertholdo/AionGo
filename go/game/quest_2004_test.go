package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestACharmedCubeLevelUp(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 3
	c := &conn{s: s, player: p}
	q := store.Quest{ID: 2004, Status: "LOCKED"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.aCharmedCubeLevelUp() {
		t.Fatal("unlocked without prerequisite")
	}
	p.quests = append(p.quests, store.Quest{ID: 2003, Status: "COMPLETE"})
	p.level = 2
	if c.aCharmedCubeLevelUp() {
		t.Fatal("unlocked below minimum level")
	}
	p.level = 3
	if !c.aCharmedCubeLevelUp() || p.quest(2004).Status != "START" || c.aCharmedCubeLevelUp() {
		t.Fatal("unlock or duplicate")
	}
}

func TestACharmedCubeDialogKillsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 3
	p.Exp = d.ExpStart(3)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 2004, Kind: data.QuestCustom, EndNPC: 203539}
	starter := questCatalogNPC(s, p, 203539, 0x32001)
	turnIn := questCatalogNPC(s, p, 203550, 0x32002)
	kill := questCatalogNPC(s, p, 210402, 0x32003)
	otherKill := questCatalogNPC(s, p, 210403, 0x32004)
	wrong := questCatalogNPC(s, p, 203540, 0x32005)
	q := store.Quest{ID: 2004, Status: "START"}
	if err := s.quests.SaveQuest(p.ID, q); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, q)
	if c.aCharmedCubeEvent(wrong, script, 25, false) || c.aCharmedCubeEvent(starter, nil, 25, false) || c.aCharmedCubeEvent(kill, script, 0, true) {
		t.Fatal("invalid event")
	}
	if !c.aCharmedCubeEvent(starter, script, 25, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(starter.id, 1011, 2004).Data) {
		t.Fatal("start page")
	}
	if !c.aCharmedCubeEvent(starter, script, 10000, false) || questVar(p.quest(2004).Vars, 0) != 1 {
		t.Fatal("first conversation")
	}
	if !c.aCharmedCubeEvent(starter, script, 25, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(starter.id, 1352, 2004).Data) {
		t.Fatal("second page")
	}
	if !c.aCharmedCubeEvent(starter, script, 33, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(starter.id, 1353, 2004).Data) {
		t.Fatal("missing item page")
	}
	if !s.addItem(p, 182203005, 1) {
		t.Fatal("item")
	}
	if !c.aCharmedCubeEvent(starter, script, 33, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(starter.id, 1438, 2004).Data) {
		t.Fatal("item page")
	}
	if !c.aCharmedCubeEvent(starter, script, 10001, false) || questVar(p.quest(2004).Vars, 0) != 2 {
		t.Fatal("second conversation")
	}
	if !c.aCharmedCubeEvent(turnIn, script, 25, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(turnIn.id, 1693, 2004).Data) {
		t.Fatal("turn-in page")
	}
	if !c.aCharmedCubeEvent(turnIn, script, 10002, false) || s.countItems(p, 182203005) != 0 || questVar(p.quest(2004).Vars, 0) != 3 {
		t.Fatal("turn-in")
	}
	if c.aCharmedCubeEvent(turnIn, script, 10002, false) {
		t.Fatal("duplicate turn-in")
	}
	for variable := int32(3); variable < 8; variable++ {
		target := kill
		if variable%2 == 0 {
			target = otherKill
		}
		if !c.aCharmedCubeEvent(target, script, 0, true) || questVar(p.quest(2004).Vars, 0) != variable+1 {
			t.Fatalf("kill at %d", variable)
		}
	}
	if !c.aCharmedCubeEvent(kill, script, 0, true) || p.quest(2004).Status != "REWARD" || c.aCharmedCubeEvent(kill, script, 0, true) {
		t.Fatal("final kill")
	}
	if !c.aCharmedCubeEvent(starter, script, -1, false) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(starter.id, 2375, 2004).Data) {
		t.Fatal("reward page")
	}
	before := p.Exp
	if !c.aCharmedCubeEvent(starter, script, 17, false) || p.quest(2004).Status != "COMPLETE" || p.Exp-before != 5160 || s.countItems(p, 122000869) != 1 {
		t.Fatal("reward")
	}
	if c.aCharmedCubeEvent(starter, script, 17, false) {
		t.Fatal("duplicate reward")
	}
}

func TestACharmedCubeObjectUseGuard(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.seen = map[int32]*object{}
	c := &conn{s: s, player: p}
	p.conn = c
	script := &data.QuestScript{ID: 2004, Kind: data.QuestCustom}
	object := questCatalogNPC(s, p, 700047, 0x32006)
	p.quests = append(p.quests, store.Quest{ID: 2004, Status: "START", Vars: 1})
	if c.aCharmedCubeEvent(object, script, 25, false) || c.aCharmedCubeEvent(object, script, -1, true) {
		t.Fatal("invalid object event")
	}
	if !c.aCharmedCubeEvent(object, script, -1, false) || object.useTask == nil {
		t.Fatal("object use did not start")
	}
	if c.aCharmedCubeEvent(object, script, -1, false) {
		t.Fatal("duplicate object use")
	}
	p.spawned = true
	p.targetID = object.id
	time.Sleep(3100 * time.Millisecond)
	if !object.dead || object.useTask != nil {
		t.Fatal("object was not consumed after use")
	}
	found := false
	for _, spawned := range s.byID {
		if spawned.npc != nil && spawned.npc.ID == 211755 {
			found = true
		}
	}
	if !found {
		t.Fatal("charmed cube creature was not spawned")
	}
}
