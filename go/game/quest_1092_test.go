package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestJosnacksDilemmaProgressCollectionAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[josnackDilemmaQuestID], d.Quests[josnackDilemmaQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.EndNPC != josnackDilemmaAtroposNPCID || template.Race != "ELYOS" || template.MinLevel != 45 || template.NameID != 2204263 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: josnackDilemmaCollectItemID, Count: 6}) || len(template.QuestDrops) != 1 ||
		template.QuestDrops[0].NPCID != josnackDilemmaSpawnNPCID || template.QuestDrops[0].ItemID != josnackDilemmaCollectItemID ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 4226400 || len(template.Rewards[0].SelectableItems) != 2 {
		t.Fatalf("unexpected Josnack's Dilemma metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{josnackDilemmaAtroposNPCID, josnackDilemmaJosnackNPCID, josnackDilemmaGuideNPCID, josnackDilemmaScoutNPCID, josnackDilemmaHeraldNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	foundDrop := false
	for _, registered := range d.QuestDropsByNPC[josnackDilemmaSpawnNPCID] {
		foundDrop = foundDrop || registered.ID == josnackDilemmaQuestID
	}
	if !foundDrop {
		t.Fatalf("quest drop index is missing NPC %d", josnackDilemmaSpawnNPCID)
	}

	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 44
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.WorldID, p.instance = josnackDilemmaMapID, josnackDilemmaInstanceID
	p.quests = []store.Quest{{ID: josnackDilemmaQuestID, Status: "LOCKED"}, {ID: 1091, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.josnackDilemmaLevelUp() {
		t.Fatal("quest unlocked below level 45")
	}
	p.level = 45
	if c.josnackDilemmaLevelUp() {
		t.Fatal("quest unlocked before quest 1091 was complete")
	}
	p.quest(1091).Status = "COMPLETE"
	if !c.josnackDilemmaLevelUp() || p.quest(josnackDilemmaQuestID).Status != "START" || c.josnackDilemmaLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(josnackDilemmaQuestID))
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
	atropos := npc(josnackDilemmaAtroposNPCID, 0x31191)
	josnack := npc(josnackDilemmaJosnackNPCID, 0x31192)
	guide := npc(josnackDilemmaGuideNPCID, 0x31193)
	scout := npc(josnackDilemmaScoutNPCID, 0x31194)
	herald := npc(josnackDilemmaHeraldNPCID, 0x31195)
	selectDialog := func(target *object, dialogID int32) bool {
		return c.josnackDilemmaDialog(target, script, dialogID)
	}
	if selectDialog(atropos, -1) {
		t.Fatal("Java does not handle the plain click while the quest is START")
	}
	if !selectDialog(atropos, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 1011, josnackDilemmaQuestID).Data) {
		t.Fatalf("Atropos opening page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(atropos, 10000) || questVar(p.quest(josnackDilemmaQuestID).Vars, 0) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 10, 0).Data) {
		t.Fatalf("first acceptance did not advance to Josnack: quest=%+v dialog=%x", p.quest(josnackDilemmaQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(josnack, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(josnack.id, 1352, josnackDilemmaQuestID).Data) {
		t.Fatalf("Josnack page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(josnack, 10001) || questVar(p.quest(josnackDilemmaQuestID).Vars, 0) != 2 {
		t.Fatalf("Josnack did not advance the quest: %+v", p.quest(josnackDilemmaQuestID))
	}
	if !selectDialog(guide, -1) || questVar(p.quest(josnackDilemmaQuestID).Vars, 0) != 2 {
		t.Fatalf("guide should update status without advancing vars: %+v", p.quest(josnackDilemmaQuestID))
	}
	if !selectDialog(scout, -1) || questVar(p.quest(josnackDilemmaQuestID).Vars, 0) != 3 {
		t.Fatalf("scout did not advance the quest: %+v", p.quest(josnackDilemmaQuestID))
	}
	if !selectDialog(atropos, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 2034, josnackDilemmaQuestID).Data) {
		t.Fatalf("Atropos report page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(atropos, 10003) || questVar(p.quest(josnackDilemmaQuestID).Vars, 0) != 4 {
		t.Fatalf("Atropos did not advance to the item collection: %+v", p.quest(josnackDilemmaQuestID))
	}
	if !selectDialog(atropos, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 2375, josnackDilemmaQuestID).Data) {
		t.Fatalf("Atropos collection page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(atropos, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 10008, josnackDilemmaQuestID).Data) {
		t.Fatalf("missing-items page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(atropos, 10000) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 10008, josnackDilemmaQuestID).Data) {
		t.Fatalf("Java's var-four fallthrough from 10000 did not check the missing items: %x", packets.last(smDialogWindow))
	}

	herald.x, herald.y, herald.z, herald.heading = 1860, 900, 420, 17
	herald.interval = 60
	s.addObject(herald)
	if !selectDialog(herald, -1) {
		t.Fatal("herald interaction did not spawn the temporary creature")
	}
	var temporary *object
	for _, candidate := range s.byID {
		if candidate.npc != nil && candidate.npc.ID == josnackDilemmaSpawnNPCID && candidate.noRespawn {
			temporary = candidate
		}
	}
	if temporary == nil || temporary.worldID != josnackDilemmaMapID || temporary.instance != josnackDilemmaInstanceID || temporary.x != herald.x || temporary.y != herald.y || temporary.z != herald.z || temporary.heading != herald.heading {
		t.Fatalf("temporary creature spawn does not match Java: %+v", temporary)
	}
	if herald.respawn == nil {
		t.Fatal("the herald was not scheduled to respawn after the forced despawn")
	}
	herald.respawn.cancel()

	if !s.addItem(p, josnackDilemmaCollectItemID, 6) {
		t.Fatal("could not add six Josnack's creature samples")
	}
	if !selectDialog(atropos, 33) || p.quest(josnackDilemmaQuestID).Status != "REWARD" || s.countItems(p, josnackDilemmaCollectItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 10001, josnackDilemmaQuestID).Data) {
		t.Fatalf("collection did not consume the items and enable reward: quest=%+v count=%d page=%x", p.quest(josnackDilemmaQuestID), s.countItems(p, josnackDilemmaCollectItemID), packets.last(smDialogWindow))
	}
	if !selectDialog(atropos, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(atropos.id, 5, josnackDilemmaQuestID).Data) {
		t.Fatalf("reward-state click did not show the reward menu: %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	if !selectDialog(atropos, 8) || p.quest(josnackDilemmaQuestID).Status != "COMPLETE" || p.quest(josnackDilemmaQuestID).CompleteCount != 1 || p.Exp-beforeExperience != template.Rewards[0].Experience {
		t.Fatalf("Josnack's Dilemma did not finish: quest=%+v XP gained=%d", p.quest(josnackDilemmaQuestID), p.Exp-beforeExperience)
	}
	if count := s.countItems(p, template.Rewards[0].SelectableItems[0].ID); count != 1 {
		t.Fatalf("selected reward count = %d, want 1", count)
	}
}
