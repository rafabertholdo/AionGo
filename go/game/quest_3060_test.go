package game

import (
	"bytes"
	"errors"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestRedJournalCatalogAndFourNPCChain(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[redJournalQuestID]
	if script == nil || script.Kind != data.QuestCustom || script.ItemUseDelay != 3000 || script.ItemID != redJournalItemID {
		t.Fatalf("Red Journal handler registration = %+v", script)
	}
	if got := d.QuestItemUses[redJournalItemID]; len(got) != 1 || got[0].ID != redJournalQuestID {
		t.Fatalf("journal item-use index = %+v", got)
	}
	for _, npcID := range []int32{redJournalStart, redJournalSecond, redJournalThird, redJournalEnd} {
		found := false
		for _, entry := range d.QuestCustomTalks[npcID] {
			found = found || entry.ID == redJournalQuestID
		}
		if !found {
			t.Errorf("NPC %d is missing the quest talk registration", npcID)
		}
	}

	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.level = "ELYOS", 48
	p.Exp = d.ExpStart(p.level)
	p.kinah = &store.Item{UniqueID: 0x33061, ItemID: data.Kinah, Count: 1, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.cube = []*store.Item{{UniqueID: 0x33060, ItemID: redJournalItemID, Count: 1, Owner: p.ID}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c

	if !c.redJournalDialog(nil, script, 1002) || p.quest(redJournalQuestID) == nil || p.quest(redJournalQuestID).Status != "START" {
		t.Fatal("the journal start dialog did not accept and persist the quest")
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(0, 0, 0).Data) {
		t.Fatalf("start dialog = %x", packets.last(smDialogWindow))
	}
	for index, step := range []struct {
		npcID  int32
		dialog int32
		page   uint16
	}{{redJournalStart, 10000, 0}, {redJournalSecond, 10001, 0}, {redJournalThird, 10002, 0}} {
		npc := questCatalogNPC(s, p, step.npcID, int32(0x33070+index))
		page := []uint16{1352, 1693, 2034}[index]
		if !c.redJournalDialog(npc, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, page, redJournalQuestID).Data) {
			t.Fatalf("NPC %d did not show page %d", step.npcID, page)
		}
		if !c.redJournalDialog(npc, script, step.dialog) || questVar(p.quest(redJournalQuestID).Vars, 0) != int32(index+1) {
			t.Fatalf("NPC %d did not advance stage %d", step.npcID, index+1)
		}
		if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 10, 0).Data) {
			t.Fatalf("NPC %d completion dialog = %x", step.npcID, packets.last(smDialogWindow))
		}
	}
	if c.redJournalDialog(questCatalogNPC(s, p, redJournalEnd, 0x33080), script, 10001) {
		t.Fatal("an unrelated dialog advanced the quest")
	}
	end := questCatalogNPC(s, p, redJournalEnd, 0x33081)
	if !c.redJournalDialog(end, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2375, redJournalQuestID).Data) {
		t.Fatal("the journal quest reward page was not shown")
	}
	s.addItem(p, redJournalItemID, 2)
	if !c.redJournalDialog(end, script, 1009) || p.quest(redJournalQuestID).Status != "REWARD" || s.countItems(p, redJournalItemID) != 0 {
		t.Fatalf("turn-in failed: quest=%+v itemCount=%d", p.quest(redJournalQuestID), s.countItems(p, redJournalItemID))
	}
	if c.redJournalDialog(questCatalogNPC(s, p, redJournalSecond, 0x33082), script, 8) {
		t.Fatal("a non-end NPC offered the reward")
	}
	if !c.redJournalDialog(end, script, 17) || p.quest(redJournalQuestID).Status != "COMPLETE" || p.quest(redJournalQuestID).CompleteCount != 1 {
		t.Fatalf("reward did not complete exactly once: %+v", p.quest(redJournalQuestID))
	}
	questTransitions := 0
	for _, saved := range saver.saved {
		if saved.ID == redJournalQuestID {
			questTransitions++
		}
	}
	if questTransitions != 6 {
		t.Fatalf("persisted Red Journal transitions = %d, want start + 3 reports + reward + completion", questTransitions)
	}
	if len(saver.rewards) != 1 {
		t.Fatalf("completion transactions = %d, want 1", len(saver.rewards))
	}
}

func TestRedJournalRejectsWrongNpcAndState(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.quests = &recordedQuests{err: errors.New("database unavailable")}
	p := wrathchild(s)
	p.Race, p.level = "ELYOS", 48
	p.seen = map[int32]*object{}
	script := d.QuestScripts[redJournalQuestID]
	c := &conn{s: s, player: p}
	p.conn = c
	npc := questCatalogNPC(s, p, redJournalSecond, 0x33100)
	if c.redJournalDialog(npc, script, 10001) {
		t.Fatal("an unstarted quest advanced through a middle NPC")
	}
	p.quests = append(p.quests, store.Quest{ID: redJournalQuestID, Status: "START", Vars: 0})
	if c.redJournalDialog(npc, script, 10001) {
		t.Fatal("the wrong middle NPC advanced the quest")
	}
	if c.redJournalDialog(questCatalogNPC(s, p, redJournalStart, 0x33101), script, 10000) {
		t.Fatal("a transition without a working quest store was reported as successful")
	}
}
