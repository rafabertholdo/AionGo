package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSanctumCeremonyLevelUpAndMageReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[sanctumCeremonyQuestID], d.Quests[sanctumCeremonyQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.LevelUpNPC != 790001 ||
		template.MinLevel != 10 || template.NameID != 2204015 || len(template.Rewards) != 4 {
		t.Fatalf("unexpected Sanctum ceremony registration: script=%+v template=%+v", script, template)
	}

	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "MAGE", 10
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: ascensionQuestID, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c

	c.levelUpStartQuests()
	if p.quest(sanctumCeremonyQuestID) != nil {
		t.Fatal("ceremony started before Ascension was complete")
	}
	p.quest(ascensionQuestID).Status = "COMPLETE"
	c.levelUpStartQuests()
	if quest := p.quest(sanctumCeremonyQuestID); quest == nil || quest.Status != "START" {
		t.Fatalf("level ten after Ascension did not start the ceremony: %+v", quest)
	}
	priest := questCatalogNPC(s, p, 790001, 0x31007)
	priest.npc = d.Npcs[790001]
	if priest.npc == nil {
		t.Fatal("High Priest's NPC template is missing")
	}
	selectDialog := func(npc *object, dialogID uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, dialogID, sanctumCeremonyQuestID))
	}
	selectDialog(priest, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(priest.id, 1011, sanctumCeremonyQuestID).Data) {
		t.Fatalf("ceremony offer page = %x", got)
	}
	selectDialog(priest, 10000)
	if quest := p.quest(sanctumCeremonyQuestID); questVar(quest.Vars, 0) != 1 {
		t.Fatalf("teleport acceptance left quest at step %d", questVar(quest.Vars, 0))
	}
	if got := packets.last(smTeleportLoc); !bytes.Equal(got, sanctumCeremonyTeleportLocation().Data) {
		t.Fatalf("Sanctum teleport location packet = %x", got)
	}

	prelate := questCatalogNPC(s, p, 203725, 0x31008)
	prelate.npc = d.Npcs[203725]
	selectDialog(prelate, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(prelate.id, 1352, sanctumCeremonyQuestID).Data) {
		t.Fatalf("ceremony prelate page = %x", got)
	}
	selectDialog(prelate, 1353)
	if got := packets.last(smPlayMovie); !bytes.Equal(got, ascensionMovie(92).Data) {
		t.Fatalf("ceremony first movie = %x", got)
	}
	selectDialog(prelate, 10001)
	if questVar(p.quest(sanctumCeremonyQuestID).Vars, 0) != 2 {
		t.Fatal("prelate report did not advance the ceremony")
	}

	deacon := questCatalogNPC(s, p, 203752, 0x31009)
	deacon.npc = d.Npcs[203752]
	selectDialog(deacon, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(deacon.id, 1693, sanctumCeremonyQuestID).Data) {
		t.Fatalf("ceremony deacon page = %x", got)
	}
	selectDialog(deacon, 1694)
	if got := packets.last(smPlayMovie); !bytes.Equal(got, ascensionMovie(91).Data) {
		t.Fatalf("ceremony second movie = %x", got)
	}
	selectDialog(deacon, 10002)
	if quest := p.quest(sanctumCeremonyQuestID); quest.Status != "REWARD" || questVar(quest.Vars, 0) != 30 {
		t.Fatalf("mage class was not assigned the mage reward: %+v", quest)
	}

	mage := questCatalogNPC(s, p, 203760, 0x3100a)
	mage.npc = d.Npcs[203760]
	c.showDialog(dialogRequest(cmShowDialog, mage.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(mage.id, 2716, sanctumCeremonyQuestID).Data) {
		t.Fatalf("mage reward preview = %x", got)
	}
	selectDialog(mage, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(mage.id, 7, sanctumCeremonyQuestID).Data) {
		t.Fatalf("mage reward choice window = %x", got)
	}
	originalExperience := p.Exp
	selectDialog(mage, 8)
	if quest := p.quest(sanctumCeremonyQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("mage reward did not complete the quest: %+v", quest)
	}
	if p.Exp != originalExperience+template.Rewards[2].Experience {
		t.Errorf("mage reward experience = %d, want %d", p.Exp-originalExperience, template.Rewards[2].Experience)
	}
	if s.countItems(p, template.Rewards[2].SelectableItems[0].ID) != 1 {
		t.Errorf("mage selectable reward item was not granted")
	}
}

func TestSanctumCeremonyStartingClassBuckets(t *testing.T) {
	for _, tc := range []struct {
		class    string
		variable int32
		endNPC   int32
		reward   int
	}{
		{"WARRIOR", 10, 203758, 0},
		{"GLADIATOR", 10, 203758, 0},
		{"SCOUT", 20, 203759, 1},
		{"RANGER", 20, 203759, 1},
		{"MAGE", 30, 203760, 2},
		{"SORCERER", 30, 203760, 2},
		{"PRIEST", 40, 203761, 3},
		{"CLERIC", 40, 203761, 3},
	} {
		if got := sanctumCeremonyClassVariable(tc.class); got != tc.variable {
			t.Errorf("class %s bucket = %d, want %d", tc.class, got, tc.variable)
		}
		npc, _, _, reward := sanctumCeremonyReward(tc.variable)
		if npc != tc.endNPC || reward != tc.reward {
			t.Errorf("class %s reward route = NPC %d reward %d, want NPC %d reward %d", tc.class, npc, reward, tc.endNPC, tc.reward)
		}
	}
}
