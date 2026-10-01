package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestFlyingReconnaissanceProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[flyingReconnaissanceQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 14 || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Flying Reconnaissance metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.WorldID = 210030000
	p.level = 13
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: flyingReconnaissanceQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: flyingReconnaissanceQuestID, Kind: data.QuestCustom, StartNPC: flyingReconnaissanceNPC, EndNPC: flyingReconnaissanceTursin}
	starter := questCatalogNPC(s, p, flyingReconnaissanceNPC, 0x31019)
	tursin := questCatalogNPC(s, p, flyingReconnaissanceTursin, 0x31020)
	guide := questCatalogNPC(s, p, flyingReconnaissanceGuide, 0x31021)

	if c.flyingReconnaissanceLevelUp() {
		t.Fatal("quest unlocked below its minimum level")
	}
	p.level = 14
	if !c.flyingReconnaissanceLevelUp() || p.quest(flyingReconnaissanceQuestID).Status != "START" || c.flyingReconnaissanceLevelUp() {
		t.Fatalf("level-up transition failed or repeated: %+v", p.quest(flyingReconnaissanceQuestID))
	}
	if c.flyingReconnaissanceDialog(tursin, script, 25) || c.flyingReconnaissanceDialog(starter, script, 99) {
		t.Fatal("wrong NPC or dialog advanced the quest")
	}
	if !c.flyingReconnaissanceDialog(starter, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(starter.id, 1011, flyingReconnaissanceQuestID).Data) {
		t.Fatal("starter conversation page did not show")
	}
	if !c.flyingReconnaissanceDialog(starter, script, 10000) || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 1 || s.countItems(p, flyingReconnaissancePassID) != 1 {
		t.Fatal("starter did not grant the pass and advance")
	}
	if c.flyingReconnaissanceEnterZone("WRONG_ZONE") || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 1 {
		t.Fatal("unrelated zone advanced the quest")
	}
	if !c.flyingReconnaissanceEnterZone("TURSIN_OUTPOST_ENTRANCE") || !bytes.Equal(packets.last(smPlayMovie), playMovie(18).Data) || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 1 {
		t.Fatal("entrance movie or separate progression did not match")
	}
	if !c.flyingReconnaissanceEnterZone("TURSIN_OUTPOST") || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 2 || c.flyingReconnaissanceEnterZone("TURSIN_OUTPOST") {
		t.Fatal("outpost entry did not advance exactly once")
	}
	if !c.flyingReconnaissanceDialog(tursin, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(tursin.id, 1352, flyingReconnaissanceQuestID).Data) || !c.flyingReconnaissanceDialog(tursin, script, 10001) || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 3 {
		t.Fatal("outpost report did not advance")
	}
	if !c.flyingReconnaissanceDialog(guide, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guide.id, 1438, flyingReconnaissanceQuestID).Data) || !c.flyingReconnaissanceDialog(guide, script, 10002) || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 4 {
		t.Fatal("guide report did not advance")
	}
	scout := questCatalogNPC(s, p, flyingReconnaissanceScout, 0x31022)
	scout.x, scout.y, scout.z = 1558.74, 1160.36, 114
	if c.flyingReconnaissanceAttack(scout) {
		t.Fatal("scout outside the marked location was accepted")
	}
	scout.x = 1552.74
	if !c.flyingReconnaissanceAttack(scout) || !scout.dead || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 5 || !bytes.Equal(packets.last(smPlayMovie), playMovie(13).Data) {
		t.Fatal("scout attack event did not kill, play the movie, and advance")
	}
	if c.flyingReconnaissanceAttack(scout) {
		t.Fatal("repeated attack event was accepted")
	}
	if !c.flyingReconnaissanceDialog(guide, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guide.id, 1693, flyingReconnaissanceQuestID).Data) || !c.flyingReconnaissanceDialog(guide, script, 10003) {
		t.Fatal("guide did not provide the totem item")
	}
	if q := p.quest(flyingReconnaissanceQuestID); questVar(q.Vars, 0) != 6 || s.countItems(p, flyingReconnaissanceItemID) != 1 {
		t.Fatalf("totem item branch failed: %+v", q)
	}

	// The three object variables share one delayed action; start at its last
	// step here to keep the test focused on callback guards and completion.
	q := p.quest(flyingReconnaissanceQuestID)
	if !c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(q.Vars, 0, 8), "") {
		t.Fatal("could not prepare the final totem step")
	}
	totem := questCatalogNPC(s, p, flyingReconnaissanceTotem, 0x31023)
	p.spawned = true
	p.targetID = totem.id
	if !c.flyingReconnaissanceDialog(totem, script, -1) || totem.useTask == nil || c.flyingReconnaissanceDialog(totem, script, -1) {
		t.Fatal("totem use did not start exactly once")
	}
	time.Sleep(3500 * time.Millisecond)
	if !totem.dead || totem.useTask != nil || questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 9 {
		t.Fatalf("delayed totem use did not consume the object and advance: quest=%+v dead=%v task=%v seen=%v target=%d", p.quest(flyingReconnaissanceQuestID), totem.dead, totem.useTask != nil, p.seen[totem.id] == totem, p.targetID)
	}
	p.zone = &data.Zone{Name: "TURSIN_TOTEM_POLE", MapID: 210030000}
	var totemItem *store.Item
	for _, item := range p.cube {
		if item.ItemID == flyingReconnaissanceItemID {
			totemItem = item
			break
		}
	}
	if !c.flyingReconnaissanceItemUse(totemItem) {
		t.Fatal("totem item-use animation did not start")
	}
	time.Sleep(3500 * time.Millisecond)
	if questVar(p.quest(flyingReconnaissanceQuestID).Vars, 0) != 10 || s.countItems(p, flyingReconnaissanceItemID) != 0 {
		t.Fatalf("delayed item use did not consume the item and advance: quest=%+v itemCount=%d stillInCube=%v zone=%+v item=%+v", p.quest(flyingReconnaissanceQuestID), s.countItems(p, flyingReconnaissanceItemID), p.cubeItem(totemItem.UniqueID) == totemItem, p.zone, totemItem)
	}
	if c.flyingReconnaissanceItemUse(totemItem) {
		t.Fatal("repeated item use was accepted after turn-in")
	}
	if c.flyingReconnaissanceKill(210696) || p.quest(flyingReconnaissanceQuestID).Status != "START" || !c.flyingReconnaissanceKill(flyingReconnaissanceBoss) || p.quest(flyingReconnaissanceQuestID).Status != "REWARD" {
		t.Fatal("final kill did not gate and unlock the reward")
	}
	if c.flyingReconnaissanceKill(flyingReconnaissanceBoss) {
		t.Fatal("repeated final kill was accepted")
	}
	if !c.flyingReconnaissanceDialog(tursin, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(tursin.id, 2034, flyingReconnaissanceQuestID).Data) {
		t.Fatal("reward preview did not show")
	}
	beforeExp := p.Exp
	choice := template.Rewards[0].SelectableItems[0]
	if !c.flyingReconnaissanceDialog(tursin, script, 8) {
		t.Fatal("first reward choice was rejected")
	}
	if q := p.quest(flyingReconnaissanceQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count {
		t.Fatalf("reward mismatch: quest=%+v exp=%d items=%d", q, p.Exp-beforeExp, s.countItems(p, choice.ID))
	}
	if c.flyingReconnaissanceDialog(tursin, script, 8) || p.quest(flyingReconnaissanceQuestID).CompleteCount != 1 {
		t.Fatal("duplicate reward completion was accepted")
	}
}

func TestFlyingReconnaissanceSelectableRewards(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[flyingReconnaissanceQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatal("Flying Reconnaissance selectable reward metadata is missing")
	}
	for index, choice := range template.Rewards[0].SelectableItems {
		t.Run(choiceName(index), func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.Race, p.Class = "ELYOS", "SORCERER"
			p.level = template.MinLevel
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.quests = []store.Quest{{ID: flyingReconnaissanceQuestID, Status: "REWARD", Vars: 10}}
			c := &conn{s: s, player: p}
			p.conn = c
			s.spawned[p.ID] = p
			script := &data.QuestScript{ID: flyingReconnaissanceQuestID, Kind: data.QuestCustom, EndNPC: flyingReconnaissanceTursin}
			npc := questCatalogNPC(s, p, flyingReconnaissanceTursin, 0x32019)
			beforeExp := p.Exp
			if !c.flyingReconnaissanceDialog(npc, script, int32(8+index)) {
				t.Fatal("valid selectable reward was rejected")
			}
			if q := p.quest(flyingReconnaissanceQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, choice.ID) != choice.Count {
				t.Fatalf("wrong reward choice %d: quest=%+v gained experience=%d item count=%d", index, q, p.Exp-beforeExp, s.countItems(p, choice.ID))
			}
		})
	}
}
