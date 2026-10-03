package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestKrallBookItemStartAndReport(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, krallBookQuestID, nil)
	template := d.Quests[krallBookQuestID]
	if script.Kind != data.QuestCustom || script.StartNPC != krallBookNPCID || script.EndNPC != krallBookReportID || script.ItemID != krallBookItemID || script.ItemUseDelay != 3000 ||
		template.Race != "ELYOS" || template.MinLevel != 14 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 14850 || template.Rewards[0].Kinah != 820 {
		t.Fatalf("unexpected Krall Book data: script=%+v template=%+v", script, template)
	}
	if !slicesContainsQuest(d.QuestCustomTalks[krallBookNPCID], krallBookQuestID) || !slicesContainsQuest(d.QuestEnds[krallBookReportID], krallBookQuestID) || len(d.QuestItemUses[krallBookItemID]) == 0 {
		t.Fatal("Krall Book is missing its NPC or item-use registration")
	}

	bookGiver := questCatalogNPC(s, p, krallBookNPCID, 0x31197)
	bookGiver.npc = d.Npcs[krallBookNPCID]
	if bookGiver.npc == nil {
		t.Fatal("book giver is absent from static NPC data")
	}
	s.initNpc(bookGiver)
	bookGiver.watchers[p.ID] = p
	c.showDialog(dialogRequest(cmShowDialog, bookGiver.id, 0, 0))
	if s.countItems(p, krallBookItemID) != 1 || p.seen[bookGiver.id] != nil {
		t.Fatalf("book interaction did not grant one book and despawn the giver: count=%d", s.countItems(p, krallBookItemID))
	}
	if !c.krallBookDialog(bookGiver, script, 25) || s.countItems(p, krallBookItemID) != 1 {
		t.Fatal("repeated book interaction granted another copy")
	}
	item := p.cubeItem(p.cube[0].UniqueID)
	if item == nil || !c.delayedItemQuestUse(item, script) || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, krallBookItemID, 3000, 0, 0).Data) {
		t.Fatal("book use did not begin Java's three-second animation")
	}
	if !c.itemStartedQuestDialog(script, 1002) || p.quest(krallBookQuestID) == nil || p.quest(krallBookQuestID).Status != "START" {
		t.Fatal("book acceptance did not start the quest")
	}
	reportNPC := questCatalogNPC(s, p, krallBookReportID, 0x32197)
	if !c.krallBookDialog(reportNPC, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 2375, krallBookQuestID).Data) {
		t.Fatalf("report page = %x", packets.last(smDialogWindow))
	}
	if !s.addItem(p, krallBookItemID, 2) {
		t.Fatal("could not seed repeated book copies")
	}
	if !c.krallBookDialog(reportNPC, script, 1009) || p.quest(krallBookQuestID).Status != "REWARD" || questVar(p.quest(krallBookQuestID).Vars, 0) != 1 || s.countItems(p, krallBookItemID) != 0 {
		t.Fatalf("report did not consume every book and ready reward: quest=%+v items=%d", p.quest(krallBookQuestID), s.countItems(p, krallBookItemID))
	}
	if !c.krallBookDialog(reportNPC, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 5, krallBookQuestID).Data) {
		t.Fatal("reward click did not open the preview")
	}
	beforeExperience := p.Exp
	if !c.krallBookDialog(reportNPC, script, 17) || p.quest(krallBookQuestID).Status != "COMPLETE" || p.Exp-beforeExperience != template.Rewards[0].Experience {
		t.Fatalf("fixed reward did not complete quest: quest=%+v experience=%d", p.quest(krallBookQuestID), p.Exp-beforeExperience)
	}
	if c.krallBookDialog(reportNPC, script, 17) {
		t.Fatal("completed quest accepted a duplicate reward")
	}
	// The delayed helper's timer must not write into this fixture after the test.
	p.conn = nil
}

func TestKrallBookRefusesInvalidNPCAndDoesNotDuplicateItem(t *testing.T) {
	_, s, p, c, script, _ := customQuestPortFixture(t, krallBookQuestID, []store.Quest{{ID: krallBookQuestID, Status: "START"}})
	bookGiver := questCatalogNPC(s, p, krallBookNPCID, 0x33197)
	wrongNPC := questCatalogNPC(s, p, 203130, 0x34197)
	if !c.krallBookDialog(bookGiver, script, -1) || s.countItems(p, krallBookItemID) != 0 {
		t.Fatal("book giver duplicated the item for an active quest")
	}
	if c.krallBookDialog(wrongNPC, script, 25) {
		t.Fatal("unregistered NPC handled the quest")
	}
}
