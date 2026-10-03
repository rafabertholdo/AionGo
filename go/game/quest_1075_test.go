package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestNewWingsFlightSpawnAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[newWingsQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 32 || template.NameID != 2204249 ||
		len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1072 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 1301000 || template.Rewards[0].AbyssPoints != 2000 ||
		template.Rewards[0].Kinah != 10000 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 162000027, Count: 5}) {
		t.Fatalf("unexpected New Wings metadata: %+v", template)
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 31
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.WorldID, p.instance = newWingsInstanceID, newWingsInstanceIndex
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x31174, ItemID: data.Kinah, Owner: p.ID, Count: 1000}
	p.quests = []store.Quest{{ID: newWingsQuestID, Status: "LOCKED"}, {ID: 1072, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.newWingsLevelUp() {
		t.Fatal("quest unlocked below level 32")
	}
	p.level = 32
	p.Exp = d.ExpStart(p.level)
	if c.newWingsLevelUp() {
		t.Fatal("quest unlocked before quest 1072 was completed")
	}
	p.quest(1072).Status = "COMPLETE"
	if !c.newWingsLevelUp() || p.quest(newWingsQuestID).Status != "START" || c.newWingsLevelUp() {
		t.Fatalf("quest did not unlock exactly once after level and prerequisite: %+v", p.quest(newWingsQuestID))
	}

	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	first := npc(newWingsFirstNPCID, 0x31175)
	end := npc(newWingsEndNPCID, 0x31176)
	instance := npc(newWingsInstanceNPCID, 0x31177)
	script := &data.QuestScript{ID: newWingsQuestID, Kind: data.QuestCustom}
	selectDialog := func(o *object, dialogID int32) bool {
		return c.newWingsDialog(o, script, dialogID)
	}
	if !selectDialog(first, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(first.id, 1011, newWingsQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	if selectDialog(first, 1013) || !bytes.Equal(packets.last(smPlayMovie), movie(0, 272).Data) {
		t.Fatalf("movie branch did not preserve Java's unhandled response: movie=%x", packets.last(smPlayMovie))
	}
	if !selectDialog(first, 10000) || questVar(p.quest(newWingsQuestID).Vars, 0) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(first.id, 10, 0).Data) {
		t.Fatalf("first NPC did not advance: quest=%+v dialog=%x", p.quest(newWingsQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 1352, newWingsQuestID).Data) {
		t.Fatalf("flight NPC page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 10001) || questVar(p.quest(newWingsQuestID).Vars, 0) != 2 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 0, 0).Data) ||
		!bytes.Equal(packets.last(smEmotion), s.playerEmotionTo(p, emoteStartFlyTele, newWingsFlightPathID, 0, 0, 0, 0, 0).Data) {
		t.Fatalf("flight NPC did not start the flight path: quest=%+v dialog=%x emotion=%x", p.quest(newWingsQuestID), packets.last(smDialogWindow), packets.last(smEmotion))
	}
	if !selectDialog(instance, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(instance.id, 1693, newWingsQuestID).Data) {
		t.Fatalf("instance NPC first page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(instance, 10002) || questVar(p.quest(newWingsQuestID).Vars, 0) != 3 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(instance.id, 0, 0).Data) {
		t.Fatalf("instance NPC did not enter its second stage: quest=%+v dialog=%x", p.quest(newWingsQuestID), packets.last(smDialogWindow))
	}
	var spawned []*object
	for _, o := range s.byID {
		if o.npc != nil && o.npc.ID == newWingsBalaurNPCID {
			spawned = append(spawned, o)
		}
	}
	if len(spawned) != 2 {
		t.Fatalf("instance conversation spawned %d Balaur, want 2", len(spawned))
	}
	wantPositions := []struct {
		x, y, z float32
		heading byte
	}{{2344.32, 1789.96, 2258.88, 86}, {2344.51, 1786.01, 2258.88, 52}}
	for _, want := range wantPositions {
		found := false
		for _, got := range spawned {
			if got.worldID == newWingsInstanceID && got.instance == newWingsInstanceIndex && got.x == want.x && got.y == want.y && got.z == want.z && got.heading == want.heading && got.noRespawn {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("temporary Balaur spawn is missing at (%f,%f,%f) heading %d: %+v", want.x, want.y, want.z, want.heading, spawned)
		}
	}
	if !selectDialog(instance, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(instance.id, 2034, newWingsQuestID).Data) {
		t.Fatalf("instance NPC second page = %x", packets.last(smDialogWindow))
	}
	// Java's case 10002 falls through to case 10003 at var three.
	if !selectDialog(instance, 10002) || p.quest(newWingsQuestID).Status != "REWARD" || questVar(p.quest(newWingsQuestID).Vars, 0) != 12 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(instance.id, 10, 0).Data) {
		t.Fatalf("var-three fallthrough did not make the quest ready: quest=%+v dialog=%x", p.quest(newWingsQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10002, newWingsQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, newWingsQuestID).Data) {
		t.Fatalf("reward selection page = %x", packets.last(smDialogWindow))
	}
	beforeExperience, beforeAP, beforeKinah := p.Exp, p.abyss.AP, p.kinah.Count
	if !selectDialog(end, 8) || p.quest(newWingsQuestID).Status != "COMPLETE" || p.quest(newWingsQuestID).CompleteCount != 1 ||
		p.Exp-beforeExperience != 1301000 || p.abyss.AP-beforeAP != 2000 || p.kinah.Count-beforeKinah != 10000 || s.countItems(p, 162000027) != 5 {
		t.Fatalf("New Wings fixed reward failed: quest=%+v experience=%d AP=%d kinah=%d items=%d", p.quest(newWingsQuestID), p.Exp-beforeExperience, p.abyss.AP-beforeAP, p.kinah.Count-beforeKinah, s.countItems(p, 162000027))
	}
}

func TestNewWingsDirectFinalDialog(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "CLERIC"
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: newWingsQuestID, Status: "START", Vars: 3}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	o := questCatalogNPC(s, p, newWingsInstanceNPCID, 0x31178)
	o.npc = d.Npcs[newWingsInstanceNPCID]
	s.initNpc(o)
	script := &data.QuestScript{ID: newWingsQuestID, Kind: data.QuestCustom}
	if !c.newWingsDialog(o, script, 10003) || p.quest(newWingsQuestID).Status != "REWARD" || questVar(p.quest(newWingsQuestID).Vars, 0) != 12 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(o.id, 10, 0).Data) {
		t.Fatalf("case 10003 did not independently finish var three: quest=%+v dialog=%x", p.quest(newWingsQuestID), packets.last(smDialogWindow))
	}
}
