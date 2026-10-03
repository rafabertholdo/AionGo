package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSecretsOfTempleLevelUpCollectRitualAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[secretsOfTempleQuestID], d.Quests[secretsOfTempleQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 25 || template.NameID != 2204113 ||
		len(template.CollectItems) != 4 || len(template.QuestDrops) != 13 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 291400 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatalf("unexpected Secrets of the Temple metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{secretsOfTempleGuideID, secretsOfTempleScribeID, 700151, 700154, 700150, 700153, 700152} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("Secrets of the Temple talk index is missing NPC %d", npcID)
		}
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 24
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: secretsOfTempleQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.secretsOfTempleLevelUp() {
		t.Fatal("quest unlocked below level 25")
	}
	p.level = 25
	if !c.secretsOfTempleLevelUp() || p.quest(secretsOfTempleQuestID).Status != "START" || c.secretsOfTempleLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(secretsOfTempleQuestID))
	}
	npc := func(id, objectID int32) *object {
		return questCatalogNPC(s, p, id, objectID)
	}
	guide := npc(secretsOfTempleGuideID, 0x31070)
	scribe := npc(secretsOfTempleScribeID, 0x31071)
	ritualSite := npc(700151, 0x31072)
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, secretsOfTempleQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, guide.id, 0, 0))
	selectDialog(guide, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guide.id, 1011, secretsOfTempleQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guide, 10000)
	selectDialog(scribe, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scribe.id, 1352, secretsOfTempleQuestID).Data) {
		t.Fatalf("scribe introduction page = %x", packets.last(smDialogWindow))
	}
	selectDialog(scribe, 10001)
	selectDialog(scribe, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scribe.id, 1693, secretsOfTempleQuestID).Data) {
		t.Fatalf("scribe collection page = %x", packets.last(smDialogWindow))
	}
	selectDialog(scribe, 1694)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scribe.id, 1779, secretsOfTempleQuestID).Data) {
		t.Fatalf("missing artifact page = %x", packets.last(smDialogWindow))
	}
	for _, itemID := range secretsOfTempleItems {
		if !s.addItem(p, itemID, 1) {
			t.Fatalf("could not add collected temple item %d", itemID)
		}
	}
	selectDialog(scribe, 1694)
	for _, itemID := range secretsOfTempleItems {
		if s.countItems(p, itemID) != 0 {
			t.Fatalf("temple collection item %d was not consumed", itemID)
		}
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(scribe.id, 1694, secretsOfTempleQuestID).Data) {
		t.Fatalf("collected artifact page = %x", packets.last(smDialogWindow))
	}
	selectDialog(scribe, 10002)
	if questVar(p.quest(secretsOfTempleQuestID).Vars, 0) != 3 || s.countItems(p, secretsOfTempleRitualItem) != 1 {
		t.Fatalf("scribe did not issue the ritual item: quest=%+v item=%d", p.quest(secretsOfTempleQuestID), s.countItems(p, secretsOfTempleRitualItem))
	}
	// Java: each of the five ritual sites plays movie 33 and, when its use timer ends, raises the variable; the fifth
	// (at 7) takes the ritual item and opens the reward.
	p.targetID = ritualSite.id
	c.showDialog(dialogRequest(cmShowDialog, ritualSite.id, 0, 0))
	if !bytes.Equal(packets.last(smPlayMovie), movie(0, 33).Data) || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, ritualSite.id, 1).Data) {
		t.Fatalf("ritual site did not start: movie=%x use=%x", packets.last(smPlayMovie), packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(secretsOfTempleQuestID).Vars, 0) != 4 || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, ritualSite.id, 0).Data) {
		t.Fatalf("ritual site interaction did not finish: quest=%+v use=%x", p.quest(secretsOfTempleQuestID), packets.last(smUseObject))
	}
	lastSite := npc(700152, 0x31073)
	p.quest(secretsOfTempleQuestID).Vars = setQuestVar(p.quest(secretsOfTempleQuestID).Vars, 0, 7) // the three sites between
	p.targetID = lastSite.id
	c.showDialog(dialogRequest(cmShowDialog, lastSite.id, 0, 0))
	time.Sleep(3100 * time.Millisecond)
	if quest := p.quest(secretsOfTempleQuestID); quest.Status != "REWARD" || s.countItems(p, secretsOfTempleRitualItem) != 0 {
		t.Fatalf("last ritual site did not open the reward: quest=%+v item=%d", quest, s.countItems(p, secretsOfTempleRitualItem))
	}
	c.showDialog(dialogRequest(cmShowDialog, guide.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(guide.id, 5, secretsOfTempleQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	selectDialog(guide, 10)
	if quest := p.quest(secretsOfTempleQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("selectable reward did not complete quest: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[2].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}
