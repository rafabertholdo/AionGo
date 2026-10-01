package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAbyssTrainingConversationMoviesAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[abyssTrainingQuestID], d.Quests[abyssTrainingQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != abyssTrainingStartNPCID || script.EndNPC != abyssTrainingEndNPCID || script.LevelUpNPC != abyssTrainingStartNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 25 || template.NameID != 2204243 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 15000 || template.Rewards[0].AbyssPoints != 500 ||
		len(template.Rewards[0].Items) != 2 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 169000005, Count: 100}) || template.Rewards[0].Items[1] != (data.QuestItem{ID: 184000017, Count: 1}) {
		t.Fatalf("unexpected Abyss Training metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{abyssTrainingStartNPCID, abyssTrainingSecondNPCID, abyssTrainingThirdNPCID, abyssTrainingFourthNPCID, abyssTrainingFifthNPCID, abyssTrainingSixthNPCID, abyssTrainingSeventhNPCID, abyssTrainingEndNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 24
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: abyssTrainingQuestID, Status: "LOCKED"}, {ID: abyssTrainingPrerequisite, Status: "COMPLETE"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	// The Java level-up handler checks only quest 1701; it does not enforce the XML min level here.
	if !c.abyssTrainingLevelUp() || p.quest(abyssTrainingQuestID).Status != "START" || c.abyssTrainingLevelUp() {
		t.Fatalf("quest did not unlock exactly once after quest 1701: %+v", p.quest(abyssTrainingQuestID))
	}

	makeNPC := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	start := makeNPC(abyssTrainingStartNPCID, 0x31171)
	second := makeNPC(abyssTrainingSecondNPCID, 0x31172)
	third := makeNPC(abyssTrainingThirdNPCID, 0x31173)
	fourth := makeNPC(abyssTrainingFourthNPCID, 0x31174)
	fifth := makeNPC(abyssTrainingFifthNPCID, 0x31175)
	sixth := makeNPC(abyssTrainingSixthNPCID, 0x31176)
	seventh := makeNPC(abyssTrainingSeventhNPCID, 0x31177)
	end := makeNPC(abyssTrainingEndNPCID, 0x31178)
	selectDialog := func(o *object, dialogID int32) bool {
		return c.abyssTrainingDialog(o, script, dialogID)
	}
	expectPage := func(o *object, page uint16) {
		t.Helper()
		if !selectDialog(o, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(o.id, page, abyssTrainingQuestID).Data) {
			t.Fatalf("NPC %d page = %x, want %d", o.npc.ID, packets.last(smDialogWindow), page)
		}
	}
	expectMovie := func(o *object, dialogID int32, movieID uint16) {
		t.Helper()
		framesBefore := len(packets.frames)
		if selectDialog(o, dialogID) || len(packets.frames) == framesBefore || !bytes.Equal(packets.last(smPlayMovie), playMovie(movieID).Data) {
			t.Fatalf("NPC %d movie dialog %d = %x, want movie %d", o.npc.ID, dialogID, packets.last(smPlayMovie), movieID)
		}
	}
	expectAdvance := func(o *object, dialogID int32, wantVariable int32) {
		t.Helper()
		if !selectDialog(o, dialogID) || questVar(p.quest(abyssTrainingQuestID).Vars, 0) != wantVariable || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(o.id, 10, 0).Data) {
			t.Fatalf("NPC %d dialog %d did not advance to %d: quest=%+v window=%x", o.npc.ID, dialogID, wantVariable, p.quest(abyssTrainingQuestID), packets.last(smDialogWindow))
		}
	}

	expectPage(start, 1011)
	expectMovie(start, 1013, 262)
	expectAdvance(start, 10000, 1)
	// Java's unmatched case 25 falls through into the movie case even after var 0 has advanced.
	expectMovie(start, 25, 262)
	expectPage(second, 1352)
	expectMovie(second, 1353, 263)
	expectAdvance(second, 10001, 2)
	expectPage(third, 1693)
	expectMovie(third, 1694, 264)
	expectAdvance(third, 10002, 3)
	expectPage(fourth, 2034)
	expectMovie(fourth, 2035, 265)
	expectAdvance(fourth, 10003, 4)
	expectPage(fifth, 2375)
	expectMovie(fifth, 2376, 266)
	expectAdvance(fifth, 10004, 5)
	expectPage(sixth, 2716)
	expectMovie(sixth, 2717, 267)
	expectAdvance(sixth, 10005, 6)
	expectPage(seventh, 3057)
	expectMovie(seventh, 3058, 268)
	if !selectDialog(seventh, 10255) || p.quest(abyssTrainingQuestID).Status != "REWARD" || questVar(p.quest(abyssTrainingQuestID).Vars, 0) != 6 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(seventh.id, 10, 0).Data) {
		t.Fatalf("final conversation did not complete training: quest=%+v window=%x", p.quest(abyssTrainingQuestID), packets.last(smDialogWindow))
	}
	if !selectDialog(end, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10002, abyssTrainingQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(end, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, abyssTrainingQuestID).Data) {
		t.Fatalf("reward selection page = %x", packets.last(smDialogWindow))
	}
	beforeExperience, beforeAbyssPoints := p.Exp, p.abyss.AP
	if !selectDialog(end, 17) || p.quest(abyssTrainingQuestID).Status != "COMPLETE" || p.quest(abyssTrainingQuestID).CompleteCount != 1 ||
		s.countItems(p, 169000005) != 100 || s.countItems(p, 184000017) != 1 || p.Exp-beforeExperience != 15000 || p.abyss.AP-beforeAbyssPoints != 500 {
		t.Fatalf("Abyss Training rewards did not complete: quest=%+v coins=%d item=%d XP=%d AP=%d", p.quest(abyssTrainingQuestID), s.countItems(p, 169000005), s.countItems(p, 184000017), p.Exp-beforeExperience, p.abyss.AP-beforeAbyssPoints)
	}
}
