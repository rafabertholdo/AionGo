package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestKlawThreatLevelUpKillItemTurnInAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[klawThreatQuestID], d.Quests[klawThreatQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != klawThreatStartNPCID || template.Race != "ELYOS" || template.MinLevel != 33 || template.NameID != 2204205 ||
		len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: klawThreatCollectItemID, Count: 3}) || len(template.QuestDrops) != 4 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 1496700 || len(template.Rewards[0].SelectableItems) != 8 {
		t.Fatalf("unexpected The Klaw Threat metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{klawThreatStartNPCID, klawThreatEndNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	for _, npcID := range []int32{klawThreatLarvaNPCID, klawThreatQueenNPCID} {
		found := false
		for _, registered := range d.QuestKills[npcID] {
			found = found || registered.ID == klawThreatQuestID
		}
		if !found {
			t.Fatalf("quest kill index is missing NPC %d", npcID)
		}
	}
	for _, npcID := range []int32{211146, 211161, 211145, 211160} {
		found := false
		for _, registered := range d.QuestDropsByNPC[npcID] {
			found = found || registered.ID == klawThreatQuestID
		}
		if !found {
			t.Fatalf("quest drop index is missing NPC %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "GLADIATOR", 32
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.quests = []store.Quest{{ID: klawThreatQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.klawThreatLevelUp() {
		t.Fatal("quest unlocked below level 33")
	}
	p.level = 33
	if c.klawThreatLevelUp() {
		t.Fatal("quest unlocked before quest 1500 was complete")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.klawThreatLevelUp() || p.quest(klawThreatQuestID).Status != "START" || c.klawThreatLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(klawThreatQuestID))
	}

	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, klawThreatQuestID))
	}
	startNPC := npc(klawThreatStartNPCID, 0x31121)
	endNPC := npc(klawThreatEndNPCID, 0x31122)
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, klawThreatQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10000)
	if questVar(p.quest(klawThreatQuestID).Vars, 0) != 1 {
		t.Fatalf("opening dialogue did not advance to stage one: %+v", p.quest(klawThreatQuestID))
	}
	selectDialog(startNPC, 25)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1352, klawThreatQuestID).Data) {
		t.Fatalf("collection stage page = %x", packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 33)
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 10001, klawThreatQuestID).Data) {
		t.Fatalf("missing collection page = %x", packets.last(smDialogWindow))
	}
	if !s.addItem(p, klawThreatCollectItemID, 3) {
		t.Fatal("could not add three Klaw larva carapaces")
	}
	selectDialog(startNPC, 33)
	if s.countItems(p, klawThreatCollectItemID) != 0 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 10000, klawThreatQuestID).Data) {
		t.Fatalf("collection hand-in did not consume three items: count=%d page=%x", s.countItems(p, klawThreatCollectItemID), packets.last(smDialogWindow))
	}
	selectDialog(startNPC, 10000)
	if questVar(p.quest(klawThreatQuestID).Vars, 0) != 3 {
		t.Fatalf("second acceptance did not advance to stage three: %+v", p.quest(klawThreatQuestID))
	}
	if !s.addItem(p, klawThreatCollectItemID, 3) {
		t.Fatal("could not add a second set of Klaw larva carapaces")
	}
	selectDialog(startNPC, 25)
	if s.countItems(p, klawThreatCollectItemID) != 3 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 10001, klawThreatQuestID).Data) {
		t.Fatalf("Java's var-three fallthrough should not consume collection items: count=%d page=%x", s.countItems(p, klawThreatCollectItemID), packets.last(smDialogWindow))
	}
	s.removeItemsByID(p, klawThreatCollectItemID, 3)

	deadLarva := &object{id: 0x31123, worldID: klawThreatWorldID, instance: 1, x: 200, y: 300, z: 100, npc: d.Npcs[klawThreatLarvaNPCID]}
	var queen *object
	for range 100 {
		if c.klawThreatKill(deadLarva) {
			for _, candidate := range s.byID {
				if candidate.npc != nil && candidate.npc.ID == klawThreatQueenNPCID {
					queen = candidate
					break
				}
			}
			if queen != nil {
				break
			}
		}
	}
	if queen == nil || !queen.noRespawn || queen.worldID != klawThreatWorldID || queen.instance != 1 || queen.x != deadLarva.x || queen.y != deadLarva.y || queen.z != deadLarva.z {
		t.Fatalf("Klaw queen was not spawned at the slain larva: %+v", queen)
	}
	s.scheduleRespawn(queen)
	if queen.respawn != nil {
		t.Fatal("the temporary Klaw queen was scheduled to respawn")
	}
	if c.klawThreatKill(queen) || p.quest(klawThreatQuestID).Status != "REWARD" {
		t.Fatalf("queen death at stage three did not activate reward: %+v", p.quest(klawThreatQuestID))
	}

	c.showDialog(dialogRequest(cmShowDialog, endNPC.id, 0, 0))
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 5, klawThreatQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	selectDialog(endNPC, 8)
	if quest := p.quest(klawThreatQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("selectable reward did not complete the quest: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[0].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}
