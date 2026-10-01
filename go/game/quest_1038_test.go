package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestShadowsCommandLevelUpObjectsBossAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[shadowsCommandQuestID], d.Quests[shadowsCommandQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 29 || template.NameID != 2204115 ||
		len(template.CollectItems) != 3 || len(template.QuestDrops) != 6 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 633400 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected The Shadow's Command metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{shadowsCommandNPCID, shadowsCommandEndNPCID, shadowsCommandFirstObjectID, shadowsCommandSecondObjectID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("The Shadow's Command talk index is missing NPC %d", npcID)
		}
	}
	if len(d.QuestKills[shadowsCommandBossID]) == 0 {
		t.Fatal("The Shadow's Command boss kill index is missing")
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 28
	p.WorldID, p.X, p.Y, p.Z = shadowsCommandWorldID, 1768, 924, 422
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: shadowsCommandQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	s.addPlayerCell(p)
	if c.shadowsCommandLevelUp() {
		t.Fatal("quest unlocked below level 29")
	}
	p.level = 29
	if !c.shadowsCommandLevelUp() || p.quest(shadowsCommandQuestID).Status != "START" || c.shadowsCommandLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(shadowsCommandQuestID))
	}
	npc := func(id, objectID int32) *object {
		return questCatalogNPC(s, p, id, objectID)
	}
	firstObject := npc(shadowsCommandFirstObjectID, 0x31080)
	questNPC := npc(shadowsCommandNPCID, 0x31081)
	secondObject := npc(shadowsCommandSecondObjectID, 0x31082)
	endNPC := npc(shadowsCommandEndNPCID, 0x31083)
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, shadowsCommandQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, firstObject.id, 0, 0))
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 0 || firstObject.useTask == nil || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, firstObject.id, 1).Data) {
		t.Fatalf("first object did not start: quest=%+v task=%v packet=%x", p.quest(shadowsCommandQuestID), firstObject.useTask != nil, packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 1 || firstObject.useTask != nil || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(34).Data) {
		t.Fatalf("first object did not complete: quest=%+v task=%v movie=%x", p.quest(shadowsCommandQuestID), firstObject.useTask != nil, packets.last(smPlayMovie))
	}
	selectDialog(questNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(questNPC.id, 1352, shadowsCommandQuestID).Data) {
		t.Fatalf("quest NPC first page = %x", packets.last(smDialogWindow))
	}
	selectDialog(questNPC, 10001)
	for _, item := range template.CollectItems {
		if !s.addItem(p, item.ID, item.Count) {
			t.Fatalf("could not add quest collectible %d", item.ID)
		}
	}
	c.showDialog(dialogRequest(cmShowDialog, secondObject.id, 0, 0))
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 2 || secondObject.useTask == nil {
		t.Fatalf("second object did not start: quest=%+v task=%v", p.quest(shadowsCommandQuestID), secondObject.useTask != nil)
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 3 || s.countItems(p, shadowsCommandOfferingID) != 1 || secondObject.useTask != nil {
		t.Fatalf("second object did not grant the offering: quest=%+v item=%d task=%v", p.quest(shadowsCommandQuestID), s.countItems(p, shadowsCommandOfferingID), secondObject.useTask != nil)
	}
	selectDialog(questNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(questNPC.id, 1694, shadowsCommandQuestID).Data) {
		t.Fatalf("quest NPC collection page = %x", packets.last(smDialogWindow))
	}
	selectDialog(questNPC, 33)
	for _, item := range template.CollectItems {
		if s.countItems(p, item.ID) != 0 {
			t.Fatalf("quest collectible %d was not consumed", item.ID)
		}
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(questNPC.id, 2035, shadowsCommandQuestID).Data) {
		t.Fatalf("successful collection page = %x", packets.last(smDialogWindow))
	}
	selectDialog(questNPC, 10002)
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 4 {
		t.Fatalf("collection did not advance stage four: %+v", p.quest(shadowsCommandQuestID))
	}
	selectDialog(questNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(questNPC.id, 2034, shadowsCommandQuestID).Data) {
		t.Fatalf("second quest NPC page = %x", packets.last(smDialogWindow))
	}
	selectDialog(questNPC, 10003)
	selectDialog(endNPC, 25)
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 6 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 2375, shadowsCommandQuestID).Data) {
		t.Fatalf("end NPC did not open the boss stage: quest=%+v page=%x", p.quest(shadowsCommandQuestID), packets.last(smDialogWindow))
	}
	selectDialog(endNPC, 10004)
	if questVar(p.quest(shadowsCommandQuestID).Vars, 0) != 7 || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(35).Data) {
		t.Fatalf("boss spawn scene did not begin: quest=%+v movie=%x", p.quest(shadowsCommandQuestID), packets.last(smPlayMovie))
	}
	var boss *object
	for _, object := range s.byID {
		if object.npc != nil && object.npc.ID == shadowsCommandBossID {
			boss = object
			break
		}
	}
	if boss == nil || boss.worldID != shadowsCommandWorldID || boss.x != float32(1768.16) || boss.y != float32(924.47) {
		t.Fatalf("quest boss did not spawn at the Java location: %+v", boss)
	}
	s.recordQuestKill(boss, p)
	if quest := p.quest(shadowsCommandQuestID); quest.Status != "REWARD" || questVar(quest.Vars, 0) != 7 {
		t.Fatalf("boss kill did not open reward: %+v", quest)
	}
	c.showDialog(dialogRequest(cmShowDialog, endNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, shadowsCommandQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	selectDialog(endNPC, 8)
	if quest := p.quest(shadowsCommandQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("selectable reward did not complete quest: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[0].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}
