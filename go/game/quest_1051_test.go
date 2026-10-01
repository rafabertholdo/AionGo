package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestTheRuinsOfRoahProgressAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[ruinsOfRoahQuestID], d.Quests[ruinsOfRoahQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != ruinsOfRoahStartNPC || script.EndNPC != ruinsOfRoahStartNPC || template.Race != "ELYOS" || template.MinLevel != 30 || template.NameID != 2204201 ||
		len(template.FinishedQuestConditions) != 0 || len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: ruinsOfRoahCollectible, Count: 1}) || len(template.Rewards) != 1 || template.Rewards[0].Experience != 404500 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected The Ruins of Roah metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{ruinsOfRoahStartNPC, ruinsOfRoahSecondNPC, ruinsOfRoahThirdNPC, ruinsOfRoahEndNPC, ruinsOfRoahTablet, ruinsOfRoahStonePlate} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC/object %d", npcID)
		}
	}
	foundDrop := false
	for _, registered := range d.QuestDropsByNPC[ruinsOfRoahStonePlate] {
		foundDrop = foundDrop || registered.ID == ruinsOfRoahQuestID
	}
	if !foundDrop {
		t.Fatal("collectible quest drop index is missing stone plate 700303")
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 29
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: ruinsOfRoahQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.ruinsOfRoahLevelUp() {
		t.Fatal("quest unlocked below level 30")
	}
	p.level = 30
	if c.ruinsOfRoahLevelUp() {
		t.Fatal("quest unlocked before quest 1500 was complete")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.ruinsOfRoahLevelUp() || p.quest(ruinsOfRoahQuestID).Status != "START" || c.ruinsOfRoahLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(ruinsOfRoahQuestID))
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
		c.dialogSelect(dialogRequest(cmDialogSelect, object.id, dialog, ruinsOfRoahQuestID))
	}
	start := npc(ruinsOfRoahStartNPC, 0x31121)
	second := npc(ruinsOfRoahSecondNPC, 0x31122)
	third := npc(ruinsOfRoahThirdNPC, 0x31123)
	end := npc(ruinsOfRoahEndNPC, 0x31124)
	tablet := npc(ruinsOfRoahTablet, 0x31125)
	stone := npc(ruinsOfRoahStonePlate, 0x31126)
	selectDialog(start, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, ruinsOfRoahQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(start, 10000)
	selectDialog(second, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(second.id, 1352, ruinsOfRoahQuestID).Data) {
		t.Fatalf("second NPC page = %x", packets.last(smDialogWindow))
	}
	selectDialog(second, 10001)
	if questVar(p.quest(ruinsOfRoahQuestID).Vars, 0) != 2 {
		t.Fatalf("second NPC did not advance to tablet: %+v", p.quest(ruinsOfRoahQuestID))
	}
	p.seen[tablet.id] = tablet
	p.targetID = tablet.id
	c.customQuestDialogID(tablet, script, -1)
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, tablet.id, 1).Data) {
		t.Fatalf("tablet interaction did not start: %x", packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, tablet.id, 0).Data) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(tablet.id, 1693, ruinsOfRoahQuestID).Data) {
		t.Fatalf("tablet interaction did not finish: use=%x dialog=%x", packets.last(smUseObject), packets.last(smDialogWindow))
	}
	c.customQuestDialogID(tablet, script, 10002)
	if questVar(p.quest(ruinsOfRoahQuestID).Vars, 0) != 3 || s.countItems(p, ruinsOfRoahArtifact) != 1 {
		t.Fatalf("tablet did not grant artifact: quest=%+v item=%d", p.quest(ruinsOfRoahQuestID), s.countItems(p, ruinsOfRoahArtifact))
	}
	selectDialog(second, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(second.id, 2034, ruinsOfRoahQuestID).Data) {
		t.Fatalf("artifact report page = %x", packets.last(smDialogWindow))
	}
	selectDialog(second, 10003)
	if questVar(p.quest(ruinsOfRoahQuestID).Vars, 0) != 4 || s.countItems(p, ruinsOfRoahArtifact) != 0 {
		t.Fatalf("artifact report did not consume artifact: quest=%+v item=%d", p.quest(ruinsOfRoahQuestID), s.countItems(p, ruinsOfRoahArtifact))
	}
	selectDialog(start, 10000) // Java case 10000 falls through to 10004 at var four.
	if questVar(p.quest(ruinsOfRoahQuestID).Vars, 0) != 5 {
		t.Fatalf("start NPC fallthrough did not advance: %+v", p.quest(ruinsOfRoahQuestID))
	}
	selectDialog(third, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(third.id, 2716, ruinsOfRoahQuestID).Data) {
		t.Fatalf("third NPC page = %x", packets.last(smDialogWindow))
	}
	selectDialog(third, 10005)
	selectDialog(end, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 3057, ruinsOfRoahQuestID).Data) {
		t.Fatalf("end NPC stage six page = %x", packets.last(smDialogWindow))
	}
	selectDialog(end, 10006)
	if questVar(p.quest(ruinsOfRoahQuestID).Vars, 0) != 7 {
		t.Fatalf("end NPC did not advance to stone plate: %+v", p.quest(ruinsOfRoahQuestID))
	}
	p.seen[stone.id] = stone
	p.targetID = stone.id
	c.customQuestDialogID(stone, script, -1)
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(ruinsOfRoahQuestID).Vars, 0) != 7 || s.countItems(p, ruinsOfRoahCollectible) != 1 {
		t.Fatalf("stone plate did not grant collectible: quest=%+v item=%d", p.quest(ruinsOfRoahQuestID), s.countItems(p, ruinsOfRoahCollectible))
	}
	selectDialog(end, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 3398, ruinsOfRoahQuestID).Data) {
		t.Fatalf("end NPC stage seven page = %x", packets.last(smDialogWindow))
	}
	selectDialog(end, 10007)
	selectDialog(end, 33)
	quest := p.quest(ruinsOfRoahQuestID)
	if quest.Status != "REWARD" || questVar(quest.Vars, 0) != 9 || s.countItems(p, ruinsOfRoahCollectible) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 10000, ruinsOfRoahQuestID).Data) {
		t.Fatalf("collectible turn-in failed: quest=%+v item=%d dialog=%x", quest, s.countItems(p, ruinsOfRoahCollectible), packets.last(smDialogWindow))
	}
	c.showDialog(dialogRequest(cmShowDialog, start.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 5, ruinsOfRoahQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	selectDialog(start, 8)
	if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, template.Rewards[0].SelectableItems[0].ID) != 1 || p.Exp != d.ExpStart(p.level)+404500 {
		t.Fatalf("reward choice did not complete quest: quest=%+v item=%d exp=%d", quest, s.countItems(p, template.Rewards[0].SelectableItems[0].ID), p.Exp)
	}
}
