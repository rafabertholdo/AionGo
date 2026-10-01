package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestPearlOfProtectionNPCChainWorkItemsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[pearlOfProtectionQuestID]
	template := d.Quests[pearlOfProtectionQuestID]
	if script == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || template == nil || template.Race != "ELYOS" || template.MinLevel != 50 || len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1097 ||
		len(template.QuestWorkItems) != 4 || template.QuestWorkItems[0].ID != pearlOfProtectionFirstItem || template.QuestWorkItems[1].ID != pearlOfProtectionSecondItem ||
		template.QuestWorkItems[2].ID != pearlOfProtectionThirdItem || template.QuestWorkItems[3].ID != pearlOfProtectionFourthItem ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 7303500 {
		t.Fatalf("unexpected Pearl of Protection metadata: %+v", template)
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Class = "SORCERER"
	p.Race, p.level = "ELYOS", 49
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: pearlOfProtectionQuestID, Status: "LOCKED"}, {ID: 1097, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.pearlOfProtectionLevelUp() {
		t.Fatal("quest unlocked below level 50")
	}
	p.level = 50
	if c.pearlOfProtectionLevelUp() {
		t.Fatal("quest unlocked before quest 1097 was complete")
	}
	p.quest(1097).Status = "COMPLETE"
	if !c.pearlOfProtectionLevelUp() || p.quest(pearlOfProtectionQuestID).Status != "START" || c.pearlOfProtectionLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(pearlOfProtectionQuestID))
	}
	for _, npcID := range []int32{pearlOfProtectionPernos, pearlOfProtectionDaminu, pearlOfProtectionLodas, pearlOfProtectionArbolu, pearlOfProtectionKhidia, pearlOfProtectionTumblusen, pearlOfProtectionAtropos, pearlOfProtectionAphesius, pearlOfProtectionJucleas, pearlOfProtectionMorai, pearlOfProtectionGaia, pearlOfProtectionKimeia, pearlOfProtectionJamanok, pearlOfProtectionSerimnir, pearlOfProtectionMaximus} {
		if len(d.QuestCustomTalks[npcID]) == 0 {
			t.Fatalf("custom talk index is missing NPC %d", npcID)
		}
	}
	npcs := make(map[int32]*object, len(pearlOfProtectionSteps))
	for i, step := range pearlOfProtectionSteps {
		npcs[step.npcID] = questCatalogNPC(s, p, step.npcID, int32(0x51098+i))
	}
	dialog := func(npcID, dialogID int32) bool {
		return c.pearlOfProtectionDialog(npcs[npcID], script, dialogID)
	}

	if dialog(pearlOfProtectionPernos, -1) || dialog(pearlOfProtectionDaminu, 25) || dialog(pearlOfProtectionPernos, 10001) {
		t.Fatal("plain click or wrong stage unexpectedly advanced the Java chain")
	}
	for index, step := range pearlOfProtectionSteps {
		if !dialog(step.npcID, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npcs[step.npcID].id, uint16(step.page), pearlOfProtectionQuestID).Data) {
			t.Fatalf("step %d page = %x", index, packets.last(smDialogWindow))
		}
		if dialog(step.npcID, step.acceptDialog+100) || questVar(p.quest(pearlOfProtectionQuestID).Vars, 0) != int32(index) {
			t.Fatalf("step %d accepted an invalid dialog", index)
		}
		if step.removeItem != 0 {
			if !s.addItem(p, step.removeItem, 2) {
				t.Fatalf("could not seed work item %d", step.removeItem)
			}
		}
		if !dialog(step.npcID, step.acceptDialog) {
			t.Fatalf("step %d did not advance", index)
		}
		wantVariable := int32(index + 1)
		wantStatus := "START"
		if step.npcID == pearlOfProtectionMaximus {
			wantVariable = int32(index)
			wantStatus = "REWARD"
		}
		if quest := p.quest(pearlOfProtectionQuestID); quest.Status != wantStatus || questVar(quest.Vars, 0) != wantVariable {
			t.Fatalf("step %d state = %+v, want status=%s variable=%d", index, quest, wantStatus, wantVariable)
		}
		if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npcs[step.npcID].id, 10, 0).Data) {
			t.Fatalf("step %d acceptance response = %x", index, got)
		}
		if step.removeItem != 0 && s.countItems(p, step.removeItem) != 0 {
			t.Fatalf("step %d did not remove all of work item %d", index, step.removeItem)
		}
		if step.grantItem != 0 && s.countItems(p, step.grantItem) != 1 {
			t.Fatalf("step %d did not grant next work item %d", index, step.grantItem)
		}
	}

	pernos := npcs[pearlOfProtectionPernos]
	if !dialog(pearlOfProtectionPernos, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 10002, pearlOfProtectionQuestID).Data) {
		t.Fatalf("reward offer page = %x", packets.last(smDialogWindow))
	}
	if !dialog(pearlOfProtectionPernos, 1009) || p.quest(pearlOfProtectionQuestID).Vars != 14 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 5, pearlOfProtectionQuestID).Data) {
		t.Fatalf("reward ready response = %x, quest=%+v", packets.last(smDialogWindow), p.quest(pearlOfProtectionQuestID))
	}
	if !dialog(pearlOfProtectionPernos, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 5, pearlOfProtectionQuestID).Data) {
		t.Fatalf("reward click response = %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	if !dialog(pearlOfProtectionPernos, 8) || p.quest(pearlOfProtectionQuestID).Status != "COMPLETE" || p.quest(pearlOfProtectionQuestID).CompleteCount != 1 || p.Exp-beforeExperience != template.Rewards[0].Experience {
		t.Fatalf("reward did not complete quest: %+v, experience delta=%d", p.quest(pearlOfProtectionQuestID), p.Exp-beforeExperience)
	}
	if dialog(pearlOfProtectionPernos, 17) {
		t.Fatal("completed quest accepted a repeated reward selection")
	}
}

func TestPearlOfProtectionPernosInitialNPCStart(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[pearlOfProtectionQuestID]
	if script == nil {
		t.Fatal("Pearl of Protection is missing from the custom quest catalog")
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Class = "SORCERER"
	p.Race, p.level = "ELYOS", 50
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1097, Status: "COMPLETE", CompleteCount: 1}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	pernos := questCatalogNPC(s, p, pearlOfProtectionPernos, 0x51097)
	pernos.npc = d.Npcs[pearlOfProtectionPernos]
	if pernos.npc == nil {
		t.Fatalf("Pernos NPC template %d is missing", pearlOfProtectionPernos)
	}
	if !c.pearlOfProtectionDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 1011, pearlOfProtectionQuestID).Data) {
		t.Fatalf("Pernos did not show the initial offer: %x", packets.last(smDialogWindow))
	}
	if !c.pearlOfProtectionDialog(pernos, script, 10000) {
		t.Fatal("Pernos did not accept the NPC-started quest")
	}
	quest := p.quest(pearlOfProtectionQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 1 || s.countItems(p, pearlOfProtectionFirstItem) != 1 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 10, 0).Data) {
		t.Fatalf("Pernos start created unexpected quest state: quest=%+v item=%d dialog=%x", quest, s.countItems(p, pearlOfProtectionFirstItem), packets.last(smDialogWindow))
	}
}
