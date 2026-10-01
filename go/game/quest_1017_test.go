package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestHeldSacredLevelUpAndTurnIn(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[heldSacredQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 13 || len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: heldSacredItemID, Count: 5}) {
		t.Fatalf("unexpected Held Sacred metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 12
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: heldSacredQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: heldSacredQuestID, Kind: data.QuestCustom, StartNPC: heldSacredNPCID, EndNPC: heldSacredNPCID}
	npc := questCatalogNPC(s, p, heldSacredNPCID, 0x31017)

	if c.heldSacredLevelUp() {
		t.Fatal("quest unlocked below its minimum level")
	}
	p.level = 13
	if !c.heldSacredLevelUp() || p.quest(heldSacredQuestID).Status != "START" || c.heldSacredLevelUp() {
		t.Fatalf("level-up transition failed or repeated: %+v", p.quest(heldSacredQuestID))
	}
	if c.heldSacredDialog(npc, script, 9999) || c.heldSacredDialog(npc, script, 25) == false {
		t.Fatal("invalid dialog or initial offer handling failed")
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1011, heldSacredQuestID).Data) {
		t.Fatalf("initial conversation = %x", got)
	}
	if !c.heldSacredDialog(npc, script, 10000) || p.quest(heldSacredQuestID).Vars != 1 || c.heldSacredDialog(npc, script, 10000) {
		t.Fatalf("initial conversation did not advance exactly once: %+v", p.quest(heldSacredQuestID))
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 10, 0).Data) {
		t.Fatalf("conversation close = %x", got)
	}
	c.heldSacredDialog(npc, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1352, heldSacredQuestID).Data) {
		t.Fatalf("turn-in conversation = %x", got)
	}
	if !s.addItem(p, heldSacredItemID, 4) {
		t.Fatal("could not add test quest items")
	}
	if !c.heldSacredDialog(npc, script, 33) || p.quest(heldSacredQuestID).Status != "START" {
		t.Fatal("insufficient turn-in items were accepted")
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1353, heldSacredQuestID).Data) {
		t.Fatalf("insufficient-item dialog = %x", got)
	}
	if !s.addItem(p, heldSacredItemID, 2) {
		t.Fatal("could not add remaining test quest items")
	}
	if !c.heldSacredDialog(npc, script, 33) {
		t.Fatal("valid turn-in was rejected")
	}
	if q := p.quest(heldSacredQuestID); q.Status != "REWARD" || q.Vars != 2 || s.countItems(p, heldSacredItemID) != 1 {
		t.Fatalf("turn-in did not consume five items and reach reward: %+v, items=%d", q, s.countItems(p, heldSacredItemID))
	}
	if c.heldSacredDialog(npc, script, 33) || p.quest(heldSacredQuestID).Status != "REWARD" {
		t.Fatal("repeated turn-in changed reward state")
	}
	if !c.heldSacredDialog(npc, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 5, heldSacredQuestID).Data) {
		t.Fatal("reward dialog did not show")
	}
	if c.heldSacredDialog(npc, script, 12) || p.quest(heldSacredQuestID).Status != "REWARD" {
		t.Fatal("out-of-range reward choice was accepted")
	}
}

func TestHeldSacredSelectableRewards(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[heldSacredQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 4 {
		t.Fatal("Held Sacred selectable reward metadata is missing")
	}
	for index, choice := range template.Rewards[0].SelectableItems {
		t.Run(choiceName(index), func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.Race, p.Class = "ELYOS", "SORCERER"
			p.level = template.MinLevel
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.quests = []store.Quest{{ID: heldSacredQuestID, Status: "REWARD", Vars: 2}}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			script := &data.QuestScript{ID: heldSacredQuestID, Kind: data.QuestCustom, StartNPC: heldSacredNPCID, EndNPC: heldSacredNPCID}
			npc := questCatalogNPC(s, p, heldSacredNPCID, 0x32017)

			beforeExp := p.Exp
			if !c.heldSacredDialog(npc, script, int32(8+index)) {
				t.Fatal("valid selectable reward was rejected")
			}
			reward := template.Rewards[0]
			if q := p.quest(heldSacredQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != reward.Experience || s.countItems(p, choice.ID) != choice.Count {
				t.Fatalf("wrong reward choice %d: quest=%+v gained experience=%d item count=%d", index, q, p.Exp-beforeExp, s.countItems(p, choice.ID))
			}
			if c.heldSacredDialog(npc, script, int32(8+index)) || p.quest(heldSacredQuestID).CompleteCount != 1 || s.countItems(p, choice.ID) != choice.Count {
				t.Fatal("repeated reward choice granted a duplicate")
			}
		})
	}
}
