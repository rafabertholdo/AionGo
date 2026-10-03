package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestDangerousArtifactLevelUpObjectsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[dangerousArtifactQuestID], d.Quests[dangerousArtifactQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 33 || template.NameID != 2204121 ||
		len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1034 || len(template.CollectItems) != 0 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 1496700 || template.Rewards[0].TitleID != 11 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected A Dangerous Artifact metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{dangerousArtifactStartNPC, dangerousArtifactEngineer, dangerousArtifactBeacon, dangerousArtifactXenophon, dangerousArtifactYuditio, dangerousArtifactLaigas, dangerousArtifactRelic} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 32
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.WorldID, p.X, p.Y, p.Z = dangerousArtifactWorld, 2200, 2300, 278
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: dangerousArtifactQuestID, Status: "LOCKED"}, {ID: disappearingAetherQuestID, Status: "COMPLETE"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	s.addPlayerCell(p)
	if c.dangerousArtifactLevelUp() {
		t.Fatal("quest unlocked below level 33")
	}
	p.level = 33
	p.quest(disappearingAetherQuestID).Status = "START"
	if c.dangerousArtifactLevelUp() {
		t.Fatal("quest unlocked before Disappearing Aether was complete")
	}
	p.quest(disappearingAetherQuestID).Status = "COMPLETE"
	if !c.dangerousArtifactLevelUp() || p.quest(dangerousArtifactQuestID).Status != "START" || c.dangerousArtifactLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(dangerousArtifactQuestID))
	}

	npc := func(id, objectID int32) *object {
		object := questCatalogNPC(s, p, id, objectID)
		object.npc = d.Npcs[id]
		if object.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(object)
		return object
	}
	selectDialog := func(object *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, dangerousArtifactQuestID))
	}
	telemachus := npc(dangerousArtifactStartNPC, 0x31101)
	engineer := npc(dangerousArtifactEngineer, 0x31102)
	xenophon := npc(dangerousArtifactXenophon, 0x31103)
	yuditio := npc(dangerousArtifactYuditio, 0x31104)
	laigas := npc(dangerousArtifactLaigas, 0x31105)
	selectDialog(telemachus, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(telemachus.id, 1011, dangerousArtifactQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(telemachus, 10000)
	selectDialog(engineer, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(engineer.id, 1352, dangerousArtifactQuestID).Data) {
		t.Fatalf("civil engineer page = %x", packets.last(smDialogWindow))
	}
	selectDialog(engineer, 10001)
	if questVar(p.quest(dangerousArtifactQuestID).Vars, 0) != 2 {
		t.Fatalf("engineer did not start the exit search: %+v", p.quest(dangerousArtifactQuestID))
	}
	beacons := s.spawnedDangerousArtifactObjectsForTest()
	if len(beacons) != 2 {
		t.Fatalf("engineer spawned %d beacon exits, want two", len(beacons))
	}
	expectedPositions := [][3]float32{{2265.621, 2357.8164, 277.8047}, {1827.1799, 2537.9143, 267.5}}
	matchedPositions := make([]bool, len(expectedPositions))
	for _, beacon := range beacons {
		if beacon.npc.ID != dangerousArtifactBeacon || beacon.worldID != dangerousArtifactWorld || beacon.instance != p.instance {
			t.Fatalf("unexpected temporary beacon identity: %+v", beacon)
		}
		for index, expected := range expectedPositions {
			if beacon.x == expected[0] && beacon.y == expected[1] && beacon.z == expected[2] {
				matchedPositions[index] = true
			}
		}
	}
	for index, matched := range matchedPositions {
		if !matched {
			t.Fatalf("temporary beacon position %v was not spawned: %+v", expectedPositions[index], beacons)
		}
	}
	beacon := beacons[0]
	p.seen[beacon.id] = beacon
	p.targetID = beacon.id
	c.customQuestDialogID(beacon, script, -1)
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, beacon.id, 1).Data) || questVar(p.quest(dangerousArtifactQuestID).Vars, 0) != 2 {
		t.Fatalf("beacon use did not start: quest=%+v packet=%x", p.quest(dangerousArtifactQuestID), packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(dangerousArtifactQuestID).Vars, 0) != 3 || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, beacon.id, 0).Data) || !bytes.Equal(packets.last(smEmotion), c.s.playerEmotionTo(p, emoteStartLoot, 0, beacon.id, 0, 0, 0, 0).Data) {
		t.Fatalf("beacon search did not complete: quest=%+v use=%x emotion=%x", p.quest(dangerousArtifactQuestID), packets.last(smUseObject), packets.last(smEmotion))
	}
	selectDialog(telemachus, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(telemachus.id, 1693, dangerousArtifactQuestID).Data) {
		t.Fatalf("second Telemachus page = %x", packets.last(smDialogWindow))
	}
	selectDialog(telemachus, 10002)
	selectDialog(xenophon, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(xenophon.id, 2034, dangerousArtifactQuestID).Data) {
		t.Fatalf("Xenophon page = %x", packets.last(smDialogWindow))
	}
	selectDialog(xenophon, 10003)
	selectDialog(yuditio, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(yuditio.id, 2375, dangerousArtifactQuestID).Data) {
		t.Fatalf("Yuditio page = %x", packets.last(smDialogWindow))
	}
	selectDialog(yuditio, 10004)
	selectDialog(telemachus, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(telemachus.id, 2716, dangerousArtifactQuestID).Data) {
		t.Fatalf("third Telemachus page = %x", packets.last(smDialogWindow))
	}
	selectDialog(telemachus, 10005)
	selectDialog(laigas, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(laigas.id, 3057, dangerousArtifactQuestID).Data) {
		t.Fatalf("Laigas first page = %x", packets.last(smDialogWindow))
	}
	selectDialog(laigas, 10006)
	if questVar(p.quest(dangerousArtifactQuestID).Vars, 0) != 8 || s.countItems(p, dangerousArtifactReportItem) != 1 || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(37).Data) {
		t.Fatalf("Laigas did not grant its report item and movie: quest=%+v item=%d movie=%x", p.quest(dangerousArtifactQuestID), s.countItems(p, dangerousArtifactReportItem), packets.last(smPlayMovie))
	}
	if !s.addItem(p, dangerousArtifactProof, 1) {
		t.Fatal("could not add the artifact proof for its timed interaction")
	}
	relic := npc(dangerousArtifactRelic, 0x31106)
	p.targetID = relic.id
	c.customQuestDialogID(relic, script, -1)
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(dangerousArtifactQuestID).Vars, 0) != 9 || s.countItems(p, dangerousArtifactProof) != 0 || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, relic.id, 0).Data) {
		t.Fatalf("stolen artifact interaction failed: quest=%+v proof=%d use=%x", p.quest(dangerousArtifactQuestID), s.countItems(p, dangerousArtifactProof), packets.last(smUseObject))
	}
	selectDialog(laigas, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(laigas.id, 3398, dangerousArtifactQuestID).Data) {
		t.Fatalf("Laigas final page = %x", packets.last(smDialogWindow))
	}
	selectDialog(laigas, 10007)
	if quest := p.quest(dangerousArtifactQuestID); quest.Status != "REWARD" || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(38).Data) {
		t.Fatalf("final report did not open reward after movie 38: quest=%+v movie=%x", quest, packets.last(smPlayMovie))
	}
	// Java takes the reward at Telemachus (REWARD -> defaultQuestEndDialog there only).
	c.showDialog(dialogRequest(cmShowDialog, telemachus.id, 0, 0))
	selectDialog(telemachus, 8)
	if quest := p.quest(dangerousArtifactQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || !bytes.Equal(packets.last(smTitleList), titleList(p).Data) {
		t.Fatalf("selectable reward did not complete the quest: quest=%+v item=%d title=%x", quest, s.countItems(p, template.Rewards[0].SelectableItems[0].ID), packets.last(smTitleList))
	}
}

func (s *Server) spawnedDangerousArtifactObjectsForTest() []*object {
	var objects []*object
	for _, o := range s.byID {
		if o.npc != nil && o.npc.ID == dangerousArtifactBeacon && o.x != 1573 {
			objects = append(objects, o)
		}
	}
	return objects
}
