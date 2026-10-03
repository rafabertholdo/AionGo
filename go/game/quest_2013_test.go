package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
)

func TestDangerousCropZoneObjectsCollectionAndReward(t *testing.T) {
	s, p, c, packets, script := altgardStartupQuestFixture(t, dangerousCropQuestID, 0)
	report := questCatalogNPC(s, p, dangerousCropReportNPCID, 0x32131)
	field := questCatalogNPC(s, p, dangerousCropFieldNPCID, 0x32132)
	if !c.dangerousCropDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1011, dangerousCropQuestID).Data) {
		t.Fatal("initial page")
	}
	if !c.dangerousCropDialog(report, script, 10000) || questVar(p.quest(dangerousCropQuestID).Vars, 0) != 1 {
		t.Fatal("first conversation stage")
	}
	if c.dangerousCropEnterZone("OTHER") || !c.dangerousCropEnterZone(dangerousCropZone) || questVar(p.quest(dangerousCropQuestID).Vars, 0) != 2 {
		t.Fatal("farmland entry did not advance the quest")
	}
	if !c.dangerousCropDialog(report, script, 10000) || s.countItems(p, dangerousCropFieldItemID) != 1 || questVar(p.quest(dangerousCropQuestID).Vars, 0) != 3 {
		t.Fatal("field item was not granted at stage two")
	}
	for _, tc := range []struct {
		variable int32
		want     int32
	}{
		{variable: 3, want: 4},
		{variable: 4, want: 5},
		{variable: 5, want: 8},
	} {
		quest := p.quest(dangerousCropQuestID)
		quest.Vars = setQuestVar(quest.Vars, 0, tc.variable)
		p.targetID = field.id
		if !c.dangerousCropDialog(field, script, -1) || field.useTask == nil {
			t.Fatalf("field interaction at variable %d did not start", tc.variable)
		}
		time.Sleep(3100 * time.Millisecond)
		if got := questVar(p.quest(dangerousCropQuestID).Vars, 0); got != tc.want {
			t.Fatalf("field interaction at variable %d advanced to %d, want %d", tc.variable, got, tc.want)
		}
	}
	if !c.dangerousCropDialog(report, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 1693, dangerousCropQuestID).Data) {
		t.Fatal("report page after the field sequence")
	}
	if !c.dangerousCropDialog(report, script, 10002) || questVar(p.quest(dangerousCropQuestID).Vars, 0) != 9 {
		t.Fatal("report did not unlock collection turn-in")
	}
	if !c.dangerousCropDialog(report, script, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(report.id, 2120, dangerousCropQuestID).Data) {
		t.Fatal("missing collection did not show the Java error page")
	}
	template := s.data.Quests[dangerousCropQuestID]
	for _, required := range template.CollectItems {
		if !s.addItem(p, required.ID, required.Count) {
			t.Fatalf("could not add required item %d", required.ID)
		}
	}
	if !c.dangerousCropDialog(report, script, 33) || p.quest(dangerousCropQuestID).Status != "REWARD" {
		t.Fatal("complete collection did not open the reward state")
	}
	for _, required := range template.CollectItems {
		if s.countItems(p, required.ID) != 0 {
			t.Fatalf("collection item %d was not consumed", required.ID)
		}
	}
	before := p.Exp
	if !c.dangerousCropDialog(report, script, 17) || p.quest(dangerousCropQuestID).Status != "COMPLETE" || p.Exp-before != template.Rewards[0].Experience {
		t.Fatal("fixed reward did not complete the quest")
	}
}

func TestDangerousCropHandlerRegistration(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[dangerousCropQuestID]
	if script == nil || script.Kind != data.QuestCustom || script.EndNPC != dangerousCropReportNPCID || len(d.QuestCustomTalks[dangerousCropFieldNPCID]) != 1 || d.QuestCustomTalks[dangerousCropFieldNPCID][0].ID != dangerousCropQuestID {
		t.Fatalf("farmland object is not registered for its custom handler: %+v", script)
	}
	if len(d.Zones[altgardDutiesMapID]) == 0 {
		t.Fatal("Altgard zones missing from static data")
	}
}
