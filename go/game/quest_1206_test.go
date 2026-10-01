package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestCollectingAriaNPCDialogAndTurnIn(t *testing.T) {
	d := staticDataOrSkip(t)
	const questID int32 = 1206
	script := d.QuestScripts[questID]
	template := d.Quests[questID]
	if script == nil || template == nil || script.Kind != data.QuestItemCollecting || script.StartNPC != 203059 || script.EndNPC != 203059 ||
		template.MinLevel != 2 || len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: 152000401, Count: 10}) {
		t.Fatalf("unexpected Collecting Aria metadata: script=%+v quest=%+v", script, template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: questID, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	polinia := questCatalogNPC(s, p, script.EndNPC, 0x31206)
	polinia.npc = d.Npcs[script.EndNPC]
	if polinia.npc == nil {
		t.Fatal("Polinia is missing from the NPC catalog")
	}

	c.showDialog(dialogRequest(cmShowDialog, polinia.id, 0, 0))
	// Java's ItemCollecting has no dialog -1 for a quest in START: the click gets the main menu and select 25 the page.
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(polinia.id, 10, 0).Data) {
		t.Fatalf("active collection quest click = %x", got)
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, polinia.id, 25, questID))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(polinia.id, 2375, questID).Data) {
		t.Fatalf("active collection quest dialog = %x", got)
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, polinia.id, 33, questID))
	if q := p.quest(questID); q.Status != "START" || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(polinia.id, 2716, questID).Data) {
		t.Fatalf("missing tribute response = quest %+v dialog %x", q, packets.last(smDialogWindow))
	}
	if !s.addItem(p, 152000401, 10) {
		t.Fatal("could not add Aria for turn-in")
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, polinia.id, 33, questID))
	if q := p.quest(questID); q.Status != "REWARD" || q.Vars != 1 || s.countItems(p, 152000401) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(polinia.id, 5, questID).Data) {
		t.Fatalf("tribute turn-in = quest %+v aria=%d dialog=%x", q, s.countItems(p, 152000401), packets.last(smDialogWindow))
	}
}
