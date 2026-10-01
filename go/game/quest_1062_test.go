package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestIndratuLegionDialogKillsSpawnAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[indratuLegionQuestID], d.Quests[indratuLegionQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != indratuLegionStartNPCID || script.EndNPC != indratuLegionStartNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 41 || template.NameID != 2204223 || len(template.FinishedQuestConditions) != 0 ||
		len(template.QuestDrops) != 1 || template.QuestDrops[0] != (data.QuestDrop{ItemID: indratuLegionDropItemID, NPCID: indratuLegionBossNPCID, Chance: 100}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 4226400 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Indratu Legion metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{indratuLegionStartNPCID, indratuLegionSecondNPCID, indratuLegionThirdNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	if len(d.QuestDropsByNPC[indratuLegionBossNPCID]) == 0 {
		t.Fatalf("quest drop index is missing boss NPC %d", indratuLegionBossNPCID)
	}
	for _, npcID := range []int32{indratuLegionTargetNPCID, indratuLegionBossNPCID} {
		found := false
		for _, registered := range d.QuestKills[npcID] {
			found = found || registered.ID == indratuLegionQuestID
		}
		if !found {
			t.Fatalf("quest kill index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 40
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.WorldID, p.instance = 210040000, 1
	p.quests = []store.Quest{{ID: indratuLegionQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.indratuLegionLevelUp() {
		t.Fatal("quest unlocked below level 41")
	}
	p.level = 41
	if c.indratuLegionLevelUp() {
		t.Fatal("quest unlocked before quest 1500 was completed")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.indratuLegionLevelUp() || p.quest(indratuLegionQuestID).Status != "START" || c.indratuLegionLevelUp() {
		t.Fatalf("quest did not unlock exactly once after quest 1500: %+v", p.quest(indratuLegionQuestID))
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
	startNPC := makeNPC(indratuLegionStartNPCID, 0x31161)
	secondNPC := makeNPC(indratuLegionSecondNPCID, 0x31162)
	thirdNPC := makeNPC(indratuLegionThirdNPCID, 0x31163)
	selectDialog := func(o *object, dialogID int32) bool {
		return c.indratuLegionDialog(o, script, dialogID)
	}
	if !selectDialog(startNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, indratuLegionQuestID).Data) {
		t.Fatalf("opening offer page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(startNPC, 10000) || questVar(p.quest(indratuLegionQuestID).Vars, 0) != 1 {
		t.Fatalf("opening conversation did not advance: %+v", p.quest(indratuLegionQuestID))
	}
	if !selectDialog(secondNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(secondNPC.id, 1352, indratuLegionQuestID).Data) {
		t.Fatalf("second NPC offer page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(secondNPC, 10001) || questVar(p.quest(indratuLegionQuestID).Vars, 0) != 2 ||
		!bytes.Equal(packets.last(smEmotion), s.playerEmotionTo(p, emoteStartFlyTele, indratuLegionFlightPathID, 0, 0, 0, 0, 0).Data) {
		t.Fatalf("flight dialogue did not advance and start the flight: quest=%+v emotion=%x", p.quest(indratuLegionQuestID), packets.last(smEmotion))
	}
	if !selectDialog(thirdNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(thirdNPC.id, 1693, indratuLegionQuestID).Data) {
		t.Fatalf("third NPC page = %x", packets.last(smDialogWindow))
	}
	if selectDialog(thirdNPC, 1694) || packets.last(smPlayMovie) == nil || questVar(p.quest(indratuLegionQuestID).Vars, 0) != 2 {
		t.Fatalf("movie branch did not preserve Java's unhandled response: movie=%x quest=%+v", packets.last(smPlayMovie), p.quest(indratuLegionQuestID))
	}
	if !selectDialog(thirdNPC, 10002) || questVar(p.quest(indratuLegionQuestID).Vars, 0) != 3 {
		t.Fatalf("third NPC conversation did not start the kill stage: %+v", p.quest(indratuLegionQuestID))
	}
	ordinary := &object{id: 0x31164, worldID: p.WorldID, instance: p.instance, x: 424, y: 672, z: 194, heading: 61, npc: &data.NpcTemplate{ID: indratuLegionTargetNPCID}}
	if c.indratuLegionKill(&object{npc: &data.NpcTemplate{ID: indratuLegionBossNPCID}}) {
		t.Fatal("boss kill advanced before the ten legionnaire kills")
	}
	for killCount := int32(0); killCount < 9; killCount++ {
		if !c.indratuLegionKill(ordinary) || questVar(p.quest(indratuLegionQuestID).Vars, 0) != killCount+4 {
			t.Fatalf("legionnaire kill %d did not advance objective: %+v", killCount+1, p.quest(indratuLegionQuestID))
		}
	}
	if questVar(p.quest(indratuLegionQuestID).Vars, 0) != 12 {
		t.Fatalf("quest should wait for the tenth kill before spawning the boss: quest=%+v", p.quest(indratuLegionQuestID))
	}
	for _, spawned := range s.byID {
		if spawned.npc != nil && spawned.npc.ID == indratuLegionBossNPCID {
			t.Fatal("Indratu boss spawned before the tenth legionnaire kill")
		}
	}
	if !c.indratuLegionKill(ordinary) || questVar(p.quest(indratuLegionQuestID).Vars, 0) != 13 {
		t.Fatalf("tenth legionnaire kill did not schedule the boss: %+v", p.quest(indratuLegionQuestID))
	}
	time.Sleep(3100 * time.Millisecond)
	var boss *object
	for _, spawned := range s.byID {
		if spawned.npc != nil && spawned.npc.ID == indratuLegionBossNPCID {
			boss = spawned
			break
		}
	}
	if boss == nil || boss.worldID != p.WorldID || boss.instance != p.instance || boss.x != ordinary.x || boss.y != ordinary.y || boss.z != ordinary.z || boss.heading != ordinary.heading {
		t.Fatalf("delayed Indratu boss spawn is missing or misplaced: boss=%+v", boss)
	}
	if !c.indratuLegionKill(boss) || p.quest(indratuLegionQuestID).Status != "REWARD" {
		t.Fatalf("boss kill did not make quest ready for reward: %+v", p.quest(indratuLegionQuestID))
	}
	if !selectDialog(startNPC, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 10002, indratuLegionQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(startNPC, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 5, indratuLegionQuestID).Data) {
		t.Fatalf("reward selection page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(startNPC, 8) || p.quest(indratuLegionQuestID).Status != "COMPLETE" || p.quest(indratuLegionQuestID).CompleteCount != 1 ||
		s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 {
		t.Fatalf("Indratu Legion reward did not complete: quest=%+v chosen item=%d", p.quest(indratuLegionQuestID), s.countItems(p, template.Rewards[0].SelectableItems[0].ID))
	}
}
