package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/store"
)

func TestImpetusiumBoxGraveNamedEnemyAndReward(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, impetusiumQuestID, nil)
	p.Race = "ASMODIANS"
	p.WorldID = altgardDutiesMapID
	p.spawned = true
	p.quests = append(p.quests, store.Quest{ID: impetusiumQuestID, Status: "START"})
	report := questCatalogNPC(s, p, impetusiumReportNPCID, 0x32181)
	box := questCatalogNPC(s, p, impetusiumJewelBoxNPCID, 0x32182)
	box.npc = d.Npcs[impetusiumJewelBoxNPCID]
	box.interval = 295
	s.initNpc(box)
	if !c.impetusiumDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1011, impetusiumQuestID).Data) ||
		!c.impetusiumDialog(report, script, 10000) {
		t.Fatal("opening dialogue")
	}
	for range 3 {
		if !c.impetusiumKill(210588) {
			t.Fatal("Impetusium kill counter stopped before var four")
		}
	}
	if c.impetusiumKill(210588) || questVar(p.quest(impetusiumQuestID).Vars, 0) != 4 {
		t.Fatal("Impetusium kill counter exceeded var four")
	}
	if !c.impetusiumDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1352, impetusiumQuestID).Data) ||
		!c.impetusiumDialog(report, script, 10000) || questVar(p.quest(impetusiumQuestID).Vars, 0) != 5 {
		t.Fatal("jewel-box stage")
	}
	if c.impetusiumDialog(questCatalogNPC(s, p, impetusiumGraveNPCID, 0x32183), script, -1) {
		t.Fatal("grave accepted an incomplete collection")
	}
	for _, item := range s.data.Quests[impetusiumQuestID].CollectItems {
		if item.ID == 182203022 {
			continue
		}
		if !s.addItem(p, item.ID, item.Count) {
			t.Fatalf("could not add collection item %d", item.ID)
		}
	}
	if !c.impetusiumDialog(box, script, -1) || box.useTask == nil {
		t.Fatal("jewel box did not start its action-item animation")
	}
	time.Sleep(3100 * time.Millisecond)
	if box.loot == nil || len(box.loot.items) != 1 || box.loot.items[0].item != 182203022 {
		t.Fatalf("jewel box quest drop = %+v task=%v dead=%v playerSpawned=%v seen=%v conn=%v quest=%+v drops=%+v", box.loot, box.useTask, box.dead, p.spawned, p.seen[box.id] == box, p.conn == c, p.quest(impetusiumQuestID), d.QuestDropsByNPC[impetusiumJewelBoxNPCID])
	}
	s.takeLoot(p, box.id, box.loot.items[0].index)
	if s.countItems(p, 182203022) != 1 {
		t.Fatal("jewel-box collection did not enter the cube")
	}
	grave := questCatalogNPC(s, p, impetusiumGraveNPCID, 0x32183)
	grave.npc = d.Npcs[impetusiumGraveNPCID]
	grave.interval = 295
	s.initNpc(grave)
	p.targetID = grave.id
	if !c.impetusiumDialog(grave, script, -1) || grave.useTask == nil {
		t.Fatal("grave did not start when the three collection items were present")
	}
	time.Sleep(3100 * time.Millisecond)
	var named *object
	for _, candidate := range s.byID {
		if candidate.npc != nil && candidate.npc.ID == impetusiumNamedNPCID {
			named = candidate
			break
		}
	}
	if named == nil || !named.noRespawn || !grave.dead || named.x != grave.x || named.y != grave.y || named.z != grave.z {
		t.Fatalf("grave interaction did not replace the grave with Umkata: named=%+v grave=%+v", named, grave)
	}
	if !c.impetusiumKill(named.npc.ID) || questVar(p.quest(impetusiumQuestID).Vars, 0) != 7 {
		t.Fatal("named enemy kill did not advance the quest")
	}
	if !c.impetusiumDialog(report, script, 33) || p.quest(impetusiumQuestID).Status != "REWARD" ||
		s.countItems(p, 182203021)+s.countItems(p, 182203022)+s.countItems(p, 182203023) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1693, impetusiumQuestID).Data) {
		t.Fatal("collection turn-in did not consume the items and open reward")
	}
	before := p.Exp
	if !c.impetusiumDialog(report, script, 17) || p.quest(impetusiumQuestID).Status != "COMPLETE" || p.Exp-before != s.data.Quests[impetusiumQuestID].Rewards[0].Experience || s.countItems(p, 120000834) != 1 {
		t.Fatal("fixed reward did not complete the quest")
	}
}
