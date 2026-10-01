package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestOdiumDukakiSettlementProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 9
	p.Exp = d.ExpStart(p.level)
	p.WorldID, p.X, p.Y, p.Z = 210030000, 800, 2490, 217
	p.zone = &data.Zone{Name: "ODIUM_REFINING_CAULDRON"}
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: 1014, Status: "LOCKED"}}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1014, Kind: data.QuestCustom, StartNPC: 203129, EndNPC: 203098,
		MonsterInfos: []data.QuestMonster{{NPCID: 210145, VarID: 0, MaxKill: 10}}}

	if c.dukakiOdiumLevelUp() {
		t.Fatal("quest unlocked below level 10")
	}
	p.level = 10
	if !c.dukakiOdiumLevelUp() || c.dukakiOdiumLevelUp() || p.quest(1014).Status != "START" {
		t.Fatalf("level-up unlock failed: %+v", p.quest(1014))
	}

	pernos := questCatalogNPC(s, p, 203129, 0x31401)
	hyan := questCatalogNPC(s, p, 730020, 0x31402)
	cauldron := questCatalogNPC(s, p, 700090, 0x31403)
	end := questCatalogNPC(s, p, 203098, 0x31404)
	if !c.dukakiOdiumDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 1011, 1014).Data) {
		t.Fatalf("initial dialog = %x", packets.last(smDialogWindow))
	}
	if !c.dukakiOdiumDialog(pernos, script, 10000) || questVar(p.quest(1014).Vars, 0) != 1 {
		t.Fatalf("Pernos did not advance: %+v", p.quest(1014))
	}
	if !c.dukakiOdiumDialog(hyan, script, 25) || !c.dukakiOdiumDialog(hyan, script, 10001) || questVar(p.quest(1014).Vars, 0) != 2 {
		t.Fatalf("Hyan did not advance: %+v", p.quest(1014))
	}
	s.data.QuestKills[210145] = []*data.QuestScript{script}
	for range 8 {
		s.recordQuestKill(&object{npc: &data.NpcTemplate{ID: 210145}}, p)
	}
	if questVar(p.quest(1014).Vars, 0) != 10 {
		t.Fatalf("Dukaki kills left variable at %d", questVar(p.quest(1014).Vars, 0))
	}
	if !c.dukakiOdiumDialog(pernos, script, 10001) || questVar(p.quest(1014).Vars, 0) != 11 {
		t.Fatalf("cauldron stage was not unlocked: %+v", p.quest(1014))
	}
	if !s.addItem(p, dukakiCrystalItemID, 1) {
		t.Fatal("could not add odium refining crystal")
	}
	if !c.dukakiOdiumDialog(cauldron, script, -1) {
		t.Fatal("cauldron did not begin its use animation")
	}
	p.targetID = cauldron.id
	time.Sleep(3100 * time.Millisecond)
	var spawned *object
	for _, object := range s.byID {
		if object.npc != nil && object.npc.ID == 210739 {
			spawned = object
			break
		}
	}
	if spawned == nil {
		t.Fatal("cauldron did not spawn the Dukaki")
	}
	if !s.addItem(p, dukakiOdiumItemID, 1) {
		t.Fatal("could not add concentrated odium")
	}
	usedItem := p.cube[len(p.cube)-1]
	if !c.dukakiOdiumItemUse(usedItem, script) {
		t.Fatal("concentrated odium did not start inside the cauldron zone")
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(1014).Vars, 0) != 14 || s.countItems(p, dukakiOdiumItemID) != 0 || s.countItems(p, dukakiCrystalItemID) != 0 || packets.last(smPlayMovie) == nil {
		t.Fatalf("refining failed: quest=%+v odium=%d crystal=%d", p.quest(1014), s.countItems(p, dukakiOdiumItemID), s.countItems(p, dukakiCrystalItemID))
	}
	if !c.dukakiOdiumDialog(pernos, script, 10002) || p.quest(1014).Status != "REWARD" {
		t.Fatalf("quest did not reach reward: %+v", p.quest(1014))
	}
	beforeExp := p.Exp
	if !c.dukakiOdiumDialog(end, script, -1) {
		t.Fatal("reward NPC did not show the reward window")
	}
	c.dukakiOdiumDialog(end, script, 8)
	if q := p.quest(1014); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != d.Quests[1014].Rewards[0].Experience {
		t.Fatalf("quest reward mismatch: %+v exp=%d", q, p.Exp-beforeExp)
	}
}

func TestOdiumDukakiItemRequiresCorrectQuestStateAndZone(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.zone = &data.Zone{Name: "DUKAKI_SETTLEMENT"}
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	script := &data.QuestScript{ID: 1014, Kind: data.QuestCustom}
	p.quests = []store.Quest{{ID: 1014, Status: "START", Vars: setQuestVar(0, 0, 11)}}
	if !s.addItem(p, dukakiOdiumItemID, 1) || !s.addItem(p, dukakiCrystalItemID, 1) {
		t.Fatal("could not add quest items")
	}
	item := p.cube[0]
	if c.dukakiOdiumItemUse(item, script) {
		t.Fatal("item was usable outside the cauldron zone")
	}
	p.zone = &data.Zone{Name: "ODIUM_REFINING_CAULDRON"}
	p.quests[0].Vars = setQuestVar(0, 0, 10)
	if c.dukakiOdiumItemUse(item, script) {
		t.Fatal("item was usable before the cauldron stage")
	}
}
