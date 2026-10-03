package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func reducingTursinStrengthFixture(t *testing.T, quests []store.Quest) (*data.Data, *Server, *player, *conn, *data.QuestScript, *object, *questPackets) {
	t.Helper()
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[reducingTursinStrengthQuestID], d.Quests[reducingTursinStrengthQuestID]
	if script == nil || template == nil {
		t.Fatal("Reducing Tursin Strength is missing from the quest catalog")
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 19
	p.WorldID = 210030000
	p.Exp = d.ExpStart(p.level)
	p.stats = s.playerStats(p)
	p.appearance = &store.Appearance{}
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = append([]store.Quest(nil), quests...)
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	npc := questCatalogNPC(s, p, reducingTursinStrengthStartNPCID, 0x31194)
	npc.npc = d.Npcs[reducingTursinStrengthStartNPCID]
	if npc.npc == nil {
		t.Fatalf("starter NPC %d is absent from static data", reducingTursinStrengthStartNPCID)
	}
	s.initNpc(npc)
	return d, s, p, c, script, npc, packets
}

func reducingTursinStrengthSelect(c *conn, npc *object, dialogID uint16) {
	c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, dialogID, reducingTursinStrengthQuestID))
}

func TestReducingTursinStrengthRegistrationAndProgress(t *testing.T) {
	d, s, p, c, script, npc, packets := reducingTursinStrengthFixture(t, []store.Quest{{ID: 1193, Status: "COMPLETE"}})
	template := d.Quests[reducingTursinStrengthQuestID]
	if script.Kind != data.QuestCustom || !script.NPCStart || script.StartNPC != reducingTursinStrengthStartNPCID || script.EndNPC != reducingTursinStrengthStartNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 19 || template.NameID != 2204729 || template.MaxRepeatCount != 1 ||
		len(template.FinishedQuestConditions) != 1 || template.FinishedQuestConditions[0] != 1193 || len(template.Rewards) != 1 ||
		template.Rewards[0].Experience != 60000 || len(template.Rewards[0].SelectableItems) != 2 {
		t.Fatalf("unexpected quest metadata: script=%+v template=%+v", script, template)
	}
	if !slicesContainsQuest(d.QuestStarts[reducingTursinStrengthStartNPCID], reducingTursinStrengthQuestID) ||
		!slicesContainsQuest(d.QuestEnds[reducingTursinStrengthStartNPCID], reducingTursinStrengthQuestID) ||
		!slicesContainsQuest(d.QuestCustomTalks[reducingTursinStrengthStartNPCID], reducingTursinStrengthQuestID) {
		t.Fatal("quest is missing its starter, end, or talk registration")
	}
	for _, npcID := range []int32{reducingTursinStrengthFirstMobID, reducingTursinStrengthSecondMobID} {
		if !slicesContainsQuest(d.QuestKills[npcID], reducingTursinStrengthQuestID) {
			t.Errorf("kill index is missing NPC %d", npcID)
		}
	}
	if !s.canStartQuest(p, script) {
		t.Fatal("eligible Elyos character cannot start the quest")
	}

	c.showDialog(dialogRequest(cmShowDialog, npc.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 10, 0).Data) {
		t.Fatalf("starter click = %x, want main menu", got)
	}
	reducingTursinStrengthSelect(c, npc, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1011, reducingTursinStrengthQuestID).Data) {
		t.Fatalf("offer page = %x", got)
	}
	reducingTursinStrengthSelect(c, npc, 1007)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 4, reducingTursinStrengthQuestID).Data) {
		t.Fatalf("quest detail = %x", got)
	}
	reducingTursinStrengthSelect(c, npc, 1002)
	quest := p.quest(reducingTursinStrengthQuestID)
	if quest == nil || quest.Status != "START" || quest.Vars != 0 {
		t.Fatalf("quest acceptance failed: %+v", quest)
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1003, reducingTursinStrengthQuestID).Data) {
		t.Fatalf("acceptance page = %x", got)
	}

	s.recordQuestKill(&object{npc: d.Npcs[reducingTursinStrengthFirstMobID]}, p)
	if quest.Vars != 0 {
		t.Fatalf("kill before entering the garrison advanced the quest: %+v", quest)
	}
	var garrison *data.Zone
	for _, zone := range d.Zones[p.WorldID] {
		if zone.Name == reducingTursinStrengthZoneName {
			garrison = zone
			break
		}
	}
	if garrison == nil {
		t.Fatal("Tursin Garrison zone is missing from the Verteron map")
	}
	s.enterQuestZone(p, nil, garrison)
	if questVar(quest.Vars, 0) != 1 || quest.Status != "START" {
		t.Fatalf("entering the garrison did not initialize progress: %+v", quest)
	}
	s.enterQuestZone(p, garrison, garrison)
	if questVar(quest.Vars, 0) != 1 {
		t.Fatalf("unchanged zone repeated progress: %+v", quest)
	}
	if c.reducingTursinStrengthEnterZone("OTHER_ZONE") {
		t.Fatal("unregistered zone advanced the quest")
	}

	for kill := int32(0); kill < reducingTursinStrengthRequiredKills-1; kill++ {
		npcID := reducingTursinStrengthFirstMobID
		if kill%2 == 1 {
			npcID = reducingTursinStrengthSecondMobID
		}
		s.recordQuestKill(&object{npc: d.Npcs[npcID]}, p)
		if got, want := questVar(quest.Vars, 0), kill+2; got != want || quest.Status != "START" {
			t.Fatalf("kill %d progress = %d/%s, want %d/START", kill+1, got, quest.Status, want)
		}
	}
	s.recordQuestKill(&object{npc: d.Npcs[reducingTursinStrengthSecondMobID]}, p)
	if quest.Status != "REWARD" || questVar(quest.Vars, 0) != reducingTursinStrengthRequiredKills {
		t.Fatalf("required final kill did not unlock reward: %+v", quest)
	}
	s.recordQuestKill(&object{npc: d.Npcs[reducingTursinStrengthFirstMobID]}, p)
	if quest.Status != "REWARD" || questVar(quest.Vars, 0) != reducingTursinStrengthRequiredKills {
		t.Fatalf("kill after completion changed quest progress: %+v", quest)
	}

	c.showDialog(dialogRequest(cmShowDialog, npc.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1352, reducingTursinStrengthQuestID).Data) {
		t.Fatalf("reward click = %x", got)
	}
	reducingTursinStrengthSelect(c, npc, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 5, reducingTursinStrengthQuestID).Data) {
		t.Fatalf("reward page = %x", got)
	}
}

func TestReducingTursinStrengthRequiresPrerequisite(t *testing.T) {
	_, _, p, c, _, npc, packets := reducingTursinStrengthFixture(t, nil)
	reducingTursinStrengthSelect(c, npc, 1002)
	if p.quest(reducingTursinStrengthQuestID) != nil {
		t.Fatalf("quest started without prerequisite 1193: %+v", p.quest(reducingTursinStrengthQuestID))
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1002, reducingTursinStrengthQuestID).Data) {
		t.Fatalf("failed accept fallback = %x", got)
	}
}

func TestReducingTursinStrengthSelectableRewards(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[reducingTursinStrengthQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 2 {
		t.Fatal("selectable reward data is missing")
	}
	for index, choice := range template.Rewards[0].SelectableItems {
		t.Run(choiceName(index), func(t *testing.T) {
			_, _, p, c, _, npc, _ := reducingTursinStrengthFixture(t, []store.Quest{
				{ID: 1193, Status: "COMPLETE"},
				{ID: reducingTursinStrengthQuestID, Status: "REWARD", Vars: reducingTursinStrengthRequiredKills},
			})
			beforeExperience := p.Exp
			reducingTursinStrengthSelect(c, npc, uint16(8+index))
			quest := p.quest(reducingTursinStrengthQuestID)
			if quest.Status != "COMPLETE" || quest.CompleteCount != 1 || p.Exp-beforeExperience != template.Rewards[0].Experience || c.s.countItems(p, choice.ID) != choice.Count {
				t.Fatalf("reward choice %d produced quest=%+v experience=%d item count=%d", index, quest, p.Exp-beforeExperience, c.s.countItems(p, choice.ID))
			}
			reducingTursinStrengthSelect(c, npc, uint16(8+index))
			if quest.CompleteCount != 1 || c.s.countItems(p, choice.ID) != choice.Count {
				t.Fatal("repeated reward choice granted a duplicate")
			}
		})
	}
}

func slicesContainsQuest(scripts []*data.QuestScript, questID int32) bool {
	for _, script := range scripts {
		if script.ID == questID {
			return true
		}
	}
	return false
}
