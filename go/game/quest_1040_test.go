package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestScoutingTheScoutsLevelUpKillRouteAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[scoutingTheScoutsQuestID], d.Quests[scoutingTheScoutsQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 31 || template.NameID != 2204119 ||
		len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1036 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 987100 ||
		len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 120000848, Count: 1}) {
		t.Fatalf("unexpected Scouting the Scouts metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{scoutingTheScoutsNPCID, scoutingScoutsReportNPC, scoutingScoutsGuideNPC, scoutingScoutsScoutNPC} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	for _, npcID := range []int32{212010, 204046} {
		found := false
		for _, registered := range d.QuestKills[npcID] {
			found = found || registered.ID == scoutingTheScoutsQuestID
		}
		if !found {
			t.Fatalf("kill index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 30
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: scoutingTheScoutsQuestID, Status: "LOCKED"}, {ID: 1036, Status: "COMPLETE"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.scoutingTheScoutsLevelUp() {
		t.Fatal("quest unlocked below level 31")
	}
	p.level = 31
	p.quest(1036).Status = "START"
	if c.scoutingTheScoutsLevelUp() {
		t.Fatal("quest unlocked before its prerequisite was complete")
	}
	p.quest(1036).Status = "COMPLETE"
	if !c.scoutingTheScoutsLevelUp() || p.quest(scoutingTheScoutsQuestID).Status != "START" || c.scoutingTheScoutsLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(scoutingTheScoutsQuestID))
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
		c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, scoutingTheScoutsQuestID))
	}
	asclepius := npc(scoutingTheScoutsNPCID, 0x31092)
	reportNPC := npc(scoutingScoutsReportNPC, 0x31093)
	guideNPC := npc(scoutingScoutsGuideNPC, 0x31094)
	scoutNPC := npc(scoutingScoutsScoutNPC, 0x31095)
	refreshNPCs := func() {
		p.spawned = true
		s.spawned[p.ID] = p
		s.addPlayerCell(p)
		p.seen = map[int32]*object{}
		for _, object := range []*object{asclepius, reportNPC, guideNPC, scoutNPC} {
			object.worldID, object.x, object.y, object.z = p.WorldID, p.X+2, p.Y, p.Z
			object.watchers = map[int32]*player{p.ID: p}
			p.seen[object.id] = object
		}
	}
	selectDialog(asclepius, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(asclepius.id, 1011, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("opening quest page = %x", packets.last(smDialogWindow))
	}
	selectDialog(asclepius, 1013)
	if !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(183).Data) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(asclepius.id, 1013, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("movie page did not preserve Java's movie-then-echo response: movie=%x dialog=%x", packets.last(smPlayMovie), packets.last(smDialogWindow))
	}
	selectDialog(asclepius, 10000)
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(asclepius.id, 10, 0).Data) {
		t.Fatalf("initial report did not begin the scout search: quest=%+v window=%x", p.quest(scoutingTheScoutsQuestID), packets.last(smDialogWindow))
	}
	for range 3 {
		if !c.scoutingTheScoutsKill(212010) {
			t.Fatal("scout Vaegir kill did not progress")
		}
	}
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 4 || c.scoutingTheScoutsKill(212010) {
		t.Fatalf("scout Vaegir hunt did not stop at three kills: %+v", p.quest(scoutingTheScoutsQuestID))
	}
	selectDialog(asclepius, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(asclepius.id, 1352, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("post-kill Asclepius page = %x", packets.last(smDialogWindow))
	}
	selectDialog(asclepius, 10001)
	selectDialog(reportNPC, 25)
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 5 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 1693, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("report NPC did not take over stage five: quest=%+v window=%x", p.quest(scoutingTheScoutsQuestID), packets.last(smDialogWindow))
	}
	selectDialog(reportNPC, 10002)
	selectDialog(guideNPC, 25)
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 6 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideNPC.id, 2034, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("guide NPC did not open stage six: quest=%+v window=%x", p.quest(scoutingTheScoutsQuestID), packets.last(smDialogWindow))
	}
	selectDialog(guideNPC, 10003)
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 7 || p.WorldID != scoutingScoutsWorldID || p.X != 2211 || p.Y != 811 || p.Z != 513 {
		t.Fatalf("first scout-zone teleport failed: quest=%+v location=%d %.0f,%.0f,%.0f", p.quest(scoutingTheScoutsQuestID), p.WorldID, p.X, p.Y, p.Z)
	}
	refreshNPCs()
	selectDialog(scoutNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scoutNPC.id, 2375, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("scout briefing page = %x", packets.last(smDialogWindow))
	}
	selectDialog(scoutNPC, 10004)
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 8 {
		t.Fatalf("briefing did not advance to scout leader: %+v", p.quest(scoutingTheScoutsQuestID))
	}
	if !c.scoutingTheScoutsKill(204046) || questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 9 || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(36).Data) || c.scoutingTheScoutsKill(204046) {
		t.Fatalf("scout leader kill or movie progression failed: quest=%+v movie=%x", p.quest(scoutingTheScoutsQuestID), packets.last(smPlayMovie))
	}
	selectDialog(scoutNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scoutNPC.id, 2716, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("scout report page = %x", packets.last(smDialogWindow))
	}
	selectDialog(scoutNPC, 10004) // Java falls through from 10004 to 10005 when var is nine.
	if questVar(p.quest(scoutingTheScoutsQuestID).Vars, 0) != 10 || p.WorldID != scoutingScoutsWorldID || p.X != 1606 || p.Y != 1529 || p.Z != 318 {
		t.Fatalf("second scout-zone teleport/fallthrough failed: quest=%+v location=%d %.0f,%.0f,%.0f", p.quest(scoutingTheScoutsQuestID), p.WorldID, p.X, p.Y, p.Z)
	}
	refreshNPCs()
	selectDialog(guideNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideNPC.id, 3057, scoutingTheScoutsQuestID).Data) {
		t.Fatalf("final guide page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guideNPC, 10003) // Java falls through from 10003 to 10006 when var is ten.
	quest := p.quest(scoutingTheScoutsQuestID)
	if quest.Status != "REWARD" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guideNPC.id, 10, 0).Data) {
		t.Fatalf("final report did not open reward: quest=%+v window=%x", quest, packets.last(smDialogWindow))
	}
	c.showDialog(dialogRequest(cmShowDialog, asclepius.id, 0, 0))
	selectDialog(asclepius, 17)
	if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, 120000848) != 1 {
		t.Fatalf("fixed reward did not complete the quest: quest=%+v reward=%d", quest, s.countItems(p, 120000848))
	}
}
