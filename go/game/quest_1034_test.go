package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestDisappearingAetherLevelUpArtifactTurnInAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[disappearingAetherQuestID], d.Quests[disappearingAetherQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 21 || template.NameID != 2204107 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: disappearingAetherItemID, Count: 5}) || len(template.QuestDrops) != 2 {
		t.Fatalf("unexpected Disappearing Aether metadata: script=%+v template=%+v", script, template)
	}
	if len(d.QuestCustomTalks[disappearingAetherNPCID]) == 0 || len(d.QuestCustomTalks[lakaiasNPCID]) == 0 || len(d.QuestCustomTalks[destroyedArtifactNPCID]) == 0 {
		t.Fatal("Disappearing Aether NPC talk indexes are missing")
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 20
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: disappearingAetherQuestID, Status: "LOCKED"}, {ID: 1300, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	if c.disappearingAetherLevelUp() {
		t.Fatal("quest unlocked below level 21")
	}
	p.level = 21
	if c.disappearingAetherLevelUp() {
		t.Fatal("quest unlocked before quest 1300 was complete")
	}
	p.quest(1300).Status = "COMPLETE"
	if !c.disappearingAetherLevelUp() || p.quest(disappearingAetherQuestID).Status != "START" || c.disappearingAetherLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(disappearingAetherQuestID))
	}
	valerius := questCatalogNPC(s, p, disappearingAetherNPCID, 0x31043)
	lakaias := questCatalogNPC(s, p, lakaiasNPCID, 0x31044)
	artifact := questCatalogNPC(s, p, destroyedArtifactNPCID, 0x31045)
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, disappearingAetherQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, valerius.id, 0, 0))
	selectDialog(valerius, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(valerius.id, 1011, disappearingAetherQuestID).Data) {
		t.Fatalf("Valerius opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(valerius, 10000)
	if questVar(p.quest(disappearingAetherQuestID).Vars, 0) != 1 {
		t.Fatalf("Valerius did not start the quest: %+v", p.quest(disappearingAetherQuestID))
	}
	selectDialog(lakaias, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(lakaias.id, 1352, disappearingAetherQuestID).Data) {
		t.Fatalf("Lakaias opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(lakaias, 10001)
	if questVar(p.quest(disappearingAetherQuestID).Vars, 0) != 2 {
		t.Fatalf("Lakaias did not send the player to the artifact: %+v", p.quest(disappearingAetherQuestID))
	}
	p.targetID = artifact.id
	c.showDialog(dialogRequest(cmShowDialog, artifact.id, 0, 0))
	if questVar(p.quest(disappearingAetherQuestID).Vars, 0) != 3 || artifact.useTask == nil || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, artifact.id, 1).Data) {
		t.Fatalf("artifact interaction did not advance the quest: quest=%+v task=%v packet=%x", p.quest(disappearingAetherQuestID), artifact.useTask != nil, packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if artifact.useTask != nil || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, artifact.id, 0).Data) {
		t.Fatalf("artifact interaction did not finish: task=%v packet=%x", artifact.useTask != nil, packets.last(smUseObject))
	}
	selectDialog(lakaias, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(lakaias.id, 1693, disappearingAetherQuestID).Data) {
		t.Fatalf("post-artifact page = %x", packets.last(smDialogWindow))
	}
	selectDialog(lakaias, 33)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(lakaias.id, 2120, disappearingAetherQuestID).Data) {
		t.Fatalf("missing-item page = %x", packets.last(smDialogWindow))
	}
	if !s.addItem(p, disappearingAetherItemID, 5) {
		t.Fatal("could not add the five collected quest items")
	}
	selectDialog(lakaias, 33)
	if quest := p.quest(disappearingAetherQuestID); quest.Status != "REWARD" || s.countItems(p, disappearingAetherItemID) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(lakaias.id, 2035, disappearingAetherQuestID).Data) {
		t.Fatalf("item turn-in did not open reward: quest=%+v items=%d page=%x", quest, s.countItems(p, disappearingAetherItemID), packets.last(smDialogWindow))
	}
	c.showDialog(dialogRequest(cmShowDialog, valerius.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(valerius.id, 2375, disappearingAetherQuestID).Data) {
		t.Fatalf("Valerius reward preview = %x", packets.last(smDialogWindow))
	}
	selectDialog(valerius, 9)
	if quest := p.quest(disappearingAetherQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("selectable reward did not complete the quest: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[1].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}
