package game

import (
	"bytes"
	"slices"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestKaidanPrisonerLevelUpRescueAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[kaidanPrisonerQuestID], d.Quests[kaidanPrisonerQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 24 || template.NameID != 2204111 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: kaidanProofItemID, Count: 1}) || len(template.QuestDrops) != 2 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 233800 || template.Rewards[0].TitleID != 10 || len(template.Rewards[0].SelectableItems) != 2 {
		t.Fatalf("unexpected Kaidan Prisoner metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{kaidanPrisonerStartID, kaidanPrisonerNPCID, kaidanOfficerNPCID, kaidanQuartermasterID, kaidanHandlerNPCID, kaidanRewardNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("Kaidan Prisoner talk index is missing NPC %d", npcID)
		}
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 23
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: kaidanPrisonerQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.kaidanPrisonerLevelUp() {
		t.Fatal("quest unlocked below level 24")
	}
	p.level = 24
	if !c.kaidanPrisonerLevelUp() || p.quest(kaidanPrisonerQuestID).Status != "START" || c.kaidanPrisonerLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(kaidanPrisonerQuestID))
	}
	npc := func(id, objectID int32) *object {
		return questCatalogNPC(s, p, id, objectID)
	}
	start := npc(kaidanPrisonerStartID, 0x31060)
	prisoner := npc(kaidanPrisonerNPCID, 0x31061)
	officer := npc(kaidanOfficerNPCID, 0x31062)
	quartermaster := npc(kaidanQuartermasterID, 0x31063)
	handler := npc(kaidanHandlerNPCID, 0x31064)
	rewardNPC := npc(kaidanRewardNPCID, 0x31065)
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, kaidanPrisonerQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, start.id, 0, 0))
	selectDialog(start, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, kaidanPrisonerQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(start, 10000)
	selectDialog(prisoner, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(prisoner.id, 1352, kaidanPrisonerQuestID).Data) {
		t.Fatalf("prisoner page = %x", packets.last(smDialogWindow))
	}
	selectDialog(prisoner, 1354)
	if !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(32).Data) {
		t.Fatalf("rescue movie = %x", packets.last(smPlayMovie))
	}
	selectDialog(prisoner, 10001)
	selectDialog(officer, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(officer.id, 1693, kaidanPrisonerQuestID).Data) {
		t.Fatalf("officer page = %x", packets.last(smDialogWindow))
	}
	selectDialog(officer, 10002)
	selectDialog(officer, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(officer.id, 2120, kaidanPrisonerQuestID).Data) {
		t.Fatalf("missing proof page = %x", packets.last(smDialogWindow))
	}
	if !s.addItem(p, kaidanProofItemID, 1) {
		t.Fatal("could not add the Kaidan proof item")
	}
	selectDialog(officer, 25)
	if s.countItems(p, kaidanProofItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(officer.id, 2034, kaidanPrisonerQuestID).Data) {
		t.Fatalf("officer did not accept the proof: item=%d page=%x", s.countItems(p, kaidanProofItemID), packets.last(smDialogWindow))
	}
	selectDialog(officer, 10003)
	if questVar(p.quest(kaidanPrisonerQuestID).Vars, 0) != 4 || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(50).Data) {
		t.Fatalf("officer did not complete the rescue movie stage: quest=%+v movie=%x", p.quest(kaidanPrisonerQuestID), packets.last(smPlayMovie))
	}
	selectDialog(quartermaster, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(quartermaster.id, 2375, kaidanPrisonerQuestID).Data) {
		t.Fatalf("quartermaster page = %x", packets.last(smDialogWindow))
	}
	selectDialog(quartermaster, 10004)
	if questVar(p.quest(kaidanPrisonerQuestID).Vars, 0) != 5 || s.countItems(p, kaidanKeyItemID) != 1 {
		t.Fatalf("quartermaster did not issue the key: quest=%+v item=%d", p.quest(kaidanPrisonerQuestID), s.countItems(p, kaidanKeyItemID))
	}
	selectDialog(handler, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(handler.id, 2716, kaidanPrisonerQuestID).Data) {
		t.Fatalf("Kaidan handler page = %x", packets.last(smDialogWindow))
	}
	selectDialog(handler, 2717)
	if quest := p.quest(kaidanPrisonerQuestID); quest.Status != "REWARD" || questVar(quest.Vars, 0) != 6 ||
		s.countItems(p, kaidanKeyItemID) != 0 || s.countItems(p, kaidanReportItemID) != 1 {
		t.Fatalf("handler did not exchange key for reward: quest=%+v key=%d report=%d", quest, s.countItems(p, kaidanKeyItemID), s.countItems(p, kaidanReportItemID))
	}
	c.showDialog(dialogRequest(cmShowDialog, rewardNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(rewardNPC.id, 5, kaidanPrisonerQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	selectDialog(rewardNPC, 9)
	if quest := p.quest(kaidanPrisonerQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 || !slices.Contains(p.titles, 10) {
		t.Fatalf("selected reward/title did not complete the quest: quest=%+v titles=%v", quest, p.titles)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[1].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}
