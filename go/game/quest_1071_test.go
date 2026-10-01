package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

type speakingBalaurFixture struct {
	d          *data.Data
	s          *Server
	p          *player
	c          *conn
	packets    *questPackets
	script     *data.QuestScript
	start      *object
	merchant   *object
	translator *object
	report     *object
}

func newSpeakingBalaurFixture(t *testing.T) speakingBalaurFixture {
	t.Helper()
	d := staticDataOrSkip(t)
	template := d.Quests[speakingBalaurQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 25 || template.NameID != 2204241 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 291400 || template.Rewards[0].AbyssPoints != 800 || template.Rewards[0].TitleID != 30 || len(template.Rewards[0].Items) != 0 || len(template.Rewards[0].SelectableItems) != 0 {
		t.Fatalf("unexpected Speaking Balaur metadata: %+v", template)
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 25
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x31179, ItemID: data.Kinah, Owner: p.ID, Count: 19999}
	p.quests = []store.Quest{{ID: speakingBalaurQuestID, Status: "LOCKED"}, {ID: 1701, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	return speakingBalaurFixture{
		d: d, s: s, p: p, c: c, packets: packets,
		script:     &data.QuestScript{ID: speakingBalaurQuestID, Kind: data.QuestCustom, ItemID: speakingBalaurPhraseItemID},
		start:      npc(speakingBalaurStartNPCID, 0x31170),
		merchant:   npc(speakingBalaurMerchantID, 0x31171),
		translator: npc(speakingBalaurTranslatorID, 0x31172),
		report:     npc(speakingBalaurReportNPCID, 0x31173),
	}
}

func TestSpeakingBalaurKinahRouteAndReward(t *testing.T) {
	f := newSpeakingBalaurFixture(t)
	if f.c.speakingBalaurLevelUp() {
		t.Fatal("quest unlocked before quest 1701 was completed")
	}
	f.p.quest(1701).Status = "COMPLETE"
	if !f.c.speakingBalaurLevelUp() || f.p.quest(speakingBalaurQuestID).Status != "START" || f.c.speakingBalaurLevelUp() {
		t.Fatalf("quest did not unlock exactly once after quest 1701: %+v", f.p.quest(speakingBalaurQuestID))
	}
	if !f.c.speakingBalaurDialog(f.start, f.script, 25) || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.start.id, 1011, speakingBalaurQuestID).Data) {
		t.Fatalf("opening quest page = %x", f.packets.last(smDialogWindow))
	}
	if !f.c.speakingBalaurDialog(f.start, f.script, 10000) || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != 1 {
		t.Fatalf("start conversation did not advance: %+v", f.p.quest(speakingBalaurQuestID))
	}
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 25) || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.merchant.id, 1352, speakingBalaurQuestID).Data) {
		t.Fatalf("merchant offer page = %x", f.packets.last(smDialogWindow))
	}
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 10010) || f.p.kinah.Count != 19999 || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != 1 || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.merchant.id, 1355, speakingBalaurQuestID).Data) {
		t.Fatalf("unaffordable phrase purchase did not show page 1355: kinah=%d quest=%+v dialog=%x", f.p.kinah.Count, f.p.quest(speakingBalaurQuestID), f.packets.last(smDialogWindow))
	}
	f.s.increaseKinah(f.p, 20001)
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 10010) || f.p.kinah.Count != 20000 || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != 7 || f.s.countItems(f.p, speakingBalaurPhraseItemID) != 1 {
		t.Fatalf("paid phrase route failed: kinah=%d quest=%+v item=%d", f.p.kinah.Count, f.p.quest(speakingBalaurQuestID), f.s.countItems(f.p, speakingBalaurPhraseItemID))
	}
	item := f.p.cubeItem(f.p.cube[0].UniqueID)
	if item == nil || !f.c.speakingBalaurItemUse(item) || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != 8 || f.s.countItems(f.p, speakingBalaurPhraseItemID) != 0 || !bytes.Equal(f.packets.last(smItemUsageAnimation), itemUsageAnimation(f.p.ID, item.UniqueID, item.ItemID, 1, 1, 0).Data) {
		t.Fatalf("item-use route did not consume the phrase and advance: quest=%+v count=%d animation=%x", f.p.quest(speakingBalaurQuestID), f.s.countItems(f.p, speakingBalaurPhraseItemID), f.packets.last(smItemUsageAnimation))
	}
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 25) || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.merchant.id, 3057, speakingBalaurQuestID).Data) {
		t.Fatalf("var-eight merchant page = %x", f.packets.last(smDialogWindow))
	}
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 10006) || f.p.quest(speakingBalaurQuestID).Status != "REWARD" || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.merchant.id, 10, 0).Data) {
		t.Fatalf("merchant did not make the quest ready for reward: quest=%+v dialog=%x", f.p.quest(speakingBalaurQuestID), f.packets.last(smDialogWindow))
	}
	if !f.c.speakingBalaurDialog(f.start, f.script, -1) || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.start.id, 10002, speakingBalaurQuestID).Data) {
		t.Fatalf("reward preview click = %x", f.packets.last(smDialogWindow))
	}
	if !f.c.speakingBalaurDialog(f.start, f.script, 1009) || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.start.id, 5, speakingBalaurQuestID).Data) {
		t.Fatalf("reward selection page = %x", f.packets.last(smDialogWindow))
	}
	beforeExperience, beforeAP := f.p.Exp, f.p.abyss.AP
	if !f.c.speakingBalaurDialog(f.start, f.script, 8) || f.p.quest(speakingBalaurQuestID).Status != "COMPLETE" || f.p.quest(speakingBalaurQuestID).CompleteCount != 1 || f.p.Exp-beforeExperience != 291400 || f.p.abyss.AP-beforeAP != 800 || !hasSpeakingBalaurTitle(f.p.titles, 30) {
		t.Fatalf("fixed reward did not complete the quest: quest=%+v exp=%d AP=%d titles=%v", f.p.quest(speakingBalaurQuestID), f.p.Exp-beforeExperience, f.p.abyss.AP-beforeAP, f.p.titles)
	}
}

func hasSpeakingBalaurTitle(titles []int32, titleID int32) bool {
	for _, existing := range titles {
		if existing == titleID {
			return true
		}
	}
	return false
}

func TestSpeakingBalaurCipherConversationAndItemRoute(t *testing.T) {
	f := newSpeakingBalaurFixture(t)
	f.p.quest(1701).Status = "COMPLETE"
	if !f.c.speakingBalaurLevelUp() {
		t.Fatal("quest did not unlock")
	}
	for _, tc := range []struct {
		npc      *object
		dialogID int32
		variable int32
		page     uint16
	}{
		{f.start, 10000, 1, 0},
		{f.merchant, 10011, 2, 0},
		{f.translator, 25, 2, 1693},
		{f.translator, 10002, 3, 0},
		{f.report, 25, 3, 2034},
		{f.report, 10003, 4, 0},
		{f.merchant, 25, 4, 2375},
	} {
		if !f.c.speakingBalaurDialog(tc.npc, f.script, tc.dialogID) || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != tc.variable {
			t.Fatalf("dialog %d at NPC %d failed: quest=%+v", tc.dialogID, tc.npc.npc.ID, f.p.quest(speakingBalaurQuestID))
		}
		if tc.page != 0 && !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(tc.npc.id, tc.page, speakingBalaurQuestID).Data) {
			t.Fatalf("NPC %d page = %x, want %d", tc.npc.npc.ID, f.packets.last(smDialogWindow), tc.page)
		}
	}
	if f.s.countItems(f.p, speakingBalaurCipherItemID) != 1 || !f.c.speakingBalaurDialog(f.merchant, f.script, 10004) || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != 5 || f.s.countItems(f.p, speakingBalaurCipherItemID) != 0 || f.s.countItems(f.p, speakingBalaurPhraseItemID) != 1 {
		t.Fatalf("cipher exchange did not grant the phrase at var five: quest=%+v cipher=%d phrase=%d", f.p.quest(speakingBalaurQuestID), f.s.countItems(f.p, speakingBalaurCipherItemID), f.s.countItems(f.p, speakingBalaurPhraseItemID))
	}
	item := f.p.cubeItem(f.p.cube[0].UniqueID)
	if item == nil || !f.c.speakingBalaurItemUse(item) || questVar(f.p.quest(speakingBalaurQuestID).Vars, 0) != 6 || f.s.countItems(f.p, speakingBalaurPhraseItemID) != 0 {
		t.Fatalf("phrase item did not advance the var-five path: quest=%+v item=%d", f.p.quest(speakingBalaurQuestID), f.s.countItems(f.p, speakingBalaurPhraseItemID))
	}
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 25) || !bytes.Equal(f.packets.last(smDialogWindow), dialogWindow(f.merchant.id, 3057, speakingBalaurQuestID).Data) {
		t.Fatalf("var-six merchant page = %x", f.packets.last(smDialogWindow))
	}
	if !f.c.speakingBalaurDialog(f.merchant, f.script, 10006) || f.p.quest(speakingBalaurQuestID).Status != "REWARD" {
		t.Fatalf("var-six route did not open reward state: %+v", f.p.quest(speakingBalaurQuestID))
	}
}
