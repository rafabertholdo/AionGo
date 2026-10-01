package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestRulersDutyLevelUpItemUseAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[rulersDutyQuestID], d.Quests[rulersDutyQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 19 || template.NameID != 2204103 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: rulersDutyItemID, Count: 1}) || len(template.QuestDrops) != 1 || template.QuestDrops[0].NPCID != seauKerubienNPCID {
		t.Fatalf("unexpected A Ruler's Duty metadata: script=%+v template=%+v", script, template)
	}
	if len(d.QuestItemUses[rulersDutyItemID]) != 1 || d.QuestItemUses[rulersDutyItemID][0].ID != rulersDutyQuestID {
		t.Fatalf("ruler's duty item use is not registered: %+v", d.QuestItemUses[rulersDutyItemID])
	}
	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 18
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: rulersDutyQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	if c.rulersDutyLevelUp() {
		t.Fatal("A Ruler's Duty unlocked below level 19")
	}
	p.level = 19
	if !c.rulersDutyLevelUp() || p.quest(rulersDutyQuestID).Status != "START" || c.rulersDutyLevelUp() {
		t.Fatalf("locked-to-start transition failed or repeated: %+v", p.quest(rulersDutyQuestID))
	}
	npc := func(id, objectID int32) *object {
		object := questCatalogNPC(s, p, id, objectID)
		object.npc = d.Npcs[id]
		if object.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		return object
	}
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, rulersDutyQuestID))
	}
	phomona := npc(phomonaNPCID, 0x31036)
	c.showDialog(dialogRequest(cmShowDialog, phomona.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(phomona.id, 10, 0).Data) {
		t.Fatalf("Phomona main menu = %x", got)
	}
	selectDialog(phomona, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(phomona.id, 1011, rulersDutyQuestID).Data) {
		t.Fatalf("Phomona opening page = %x", got)
	}
	selectDialog(phomona, 10000)
	demro := npc(demroNPCID, 0x31037)
	selectDialog(demro, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(demro.id, 1352, rulersDutyQuestID).Data) {
		t.Fatalf("Demro page = %x", got)
	}
	selectDialog(demro, 10001)
	lodas := npc(lodasNPCID, 0x31038)
	selectDialog(lodas, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(lodas.id, 1693, rulersDutyQuestID).Data) {
		t.Fatalf("Lodas page = %x", got)
	}
	selectDialog(lodas, 10002)
	if got := questVar(p.quest(rulersDutyQuestID).Vars, 0); got != 3 || packets.last(smPlayMovie) == nil {
		t.Fatalf("Lodas did not advance to the Kerubien: quest=%+v", p.quest(rulersDutyQuestID))
	}

	kerubien := npc(seauKerubienNPCID, 0x31039)
	p.targetID = kerubien.id
	c.showDialog(dialogRequest(cmShowDialog, kerubien.id, 0, 0))
	if got := s.countItems(p, rulersDutyItemID); got != 1 || kerubien.useTask == nil || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, kerubien.id, 1).Data) {
		t.Fatalf("Kerubien did not start the item interaction: item=%d task=%v packet=%x", got, kerubien.useTask != nil, packets.last(smUseObject))
	}
	kerubien.useTask.cancel()
	kerubien.useTask = nil
	p.zone = &data.Zone{Name: "PUTRID_MIRE", MapID: 210020000}
	item := p.cubeItem(p.cube[0].UniqueID)
	c.rulersDutyItemUse(item)
	time.Sleep(3100 * time.Millisecond)
	if questVar(p.quest(rulersDutyQuestID).Vars, 0) != 4 || s.countItems(p, rulersDutyItemID) != 1 {
		t.Fatalf("item use did not advance to step four while preserving the proof: quest=%+v item=%d", p.quest(rulersDutyQuestID), s.countItems(p, rulersDutyItemID))
	}
	selectDialog(lodas, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(lodas.id, 2034, rulersDutyQuestID).Data) {
		t.Fatalf("Lodas proof page = %x", got)
	}
	selectDialog(lodas, 10003)
	selectDialog(demro, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(demro.id, 2375, rulersDutyQuestID).Data) {
		t.Fatalf("Demro report page = %x", got)
	}
	selectDialog(demro, 10004)
	if p.quest(rulersDutyQuestID).Status != "REWARD" {
		t.Fatalf("Demro did not unlock the reward: %+v", p.quest(rulersDutyQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, phomona.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(phomona.id, 2716, rulersDutyQuestID).Data) {
		t.Fatalf("Phomona reward preview = %x", got)
	}
	selectDialog(phomona, 1009)
	selectDialog(phomona, 8)
	if quest := p.quest(rulersDutyQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("selected reward did not complete A Ruler's Duty: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[0].ID); got != 1 {
		t.Errorf("selected reward count = %d, want 1", got)
	}
}
