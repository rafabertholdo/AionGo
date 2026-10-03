package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestOrdersFromTelemachusZoneStartLevelUpAndFollowUps(t *testing.T) {
	d, s, p, c, script, packets := customQuestPortFixture(t, ordersFromTelemachusQuestID, nil)
	template := d.Quests[ordersFromTelemachusQuestID]
	if script.Kind != data.QuestCustom || script.StartNPC != ordersFromTelemachusNPCID || script.EndNPC != ordersFromTelemachusNPCID || template.Race != "ELYOS" ||
		template.MinLevel != 19 || template.NameID != 2204801 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 10000 {
		t.Fatalf("unexpected Orders from Telemachus data: script=%+v template=%+v", script, template)
	}
	if !slicesContainsQuest(d.QuestEnds[ordersFromTelemachusNPCID], ordersFromTelemachusQuestID) {
		t.Fatal("Telemachus is missing its talk-event index")
	}

	p.WorldID = 210020000
	zone := &data.Zone{Name: ordersFromTelemachusZone, MapID: p.WorldID}
	s.enterQuestZone(p, nil, zone)
	if p.quest(ordersFromTelemachusQuestID) == nil || p.quest(ordersFromTelemachusQuestID).Status != "START" {
		t.Fatal("entering Eltnen Fortress did not start the quest")
	}
	if c.ordersFromTelemachusEnterZone(ordersFromTelemachusZone) {
		t.Fatal("repeated zone entry restarted the quest")
	}
	npc := questCatalogNPC(s, p, ordersFromTelemachusNPCID, 0x31300)
	c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, 25, ordersFromTelemachusQuestID))
	if p.quest(ordersFromTelemachusQuestID).Status != "REWARD" || questVar(p.quest(ordersFromTelemachusQuestID).Vars, 0) != 1 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 1011, ordersFromTelemachusQuestID).Data) {
		t.Fatalf("Telemachus report state or page = %+v / %x", p.quest(ordersFromTelemachusQuestID), packets.last(smDialogWindow))
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, 17, ordersFromTelemachusQuestID))
	if p.quest(ordersFromTelemachusQuestID).Status != "COMPLETE" {
		t.Fatalf("quest did not complete: %+v", p.quest(ordersFromTelemachusQuestID))
	}
	for _, id := range ordersFromTelemachusFollowUps {
		wantStatus := "LOCKED"
		if id == 1031 || id == 1032 {
			wantStatus = "START" // Completing Q1300 runs the Java level-up hooks for these eligible follow-ups.
		}
		if q := p.quest(id); q == nil || q.Status != wantStatus {
			t.Errorf("follow-up quest %d status = %+v, want %s", id, q, wantStatus)
		}
	}

	_, _, lockedPlayer, lockedConn, _, _ := customQuestPortFixture(t, ordersFromTelemachusQuestID, []store.Quest{{ID: ordersFromTelemachusQuestID, Status: "LOCKED"}})
	lockedPlayer.level = template.MinLevel - 1
	if lockedConn.ordersFromTelemachusLevelUp() {
		t.Fatal("level-up unlocked the quest below its minimum level")
	}
	lockedPlayer.level = template.MinLevel
	if !lockedConn.ordersFromTelemachusLevelUp() || lockedPlayer.quest(ordersFromTelemachusQuestID).Status != "START" || lockedConn.ordersFromTelemachusLevelUp() {
		t.Fatal("level-up did not unlock a locked quest exactly once")
	}
}
