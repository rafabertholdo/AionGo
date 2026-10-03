package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestMandurisSecretLevelUpKillsAndStoryReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[mandurisSecretQuestID], d.Quests[mandurisSecretQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 19 || template.NameID != 2204101 || len(template.Rewards) != 1 {
		t.Fatalf("unexpected Manduri's Secret metadata: script=%+v template=%+v", script, template)
	}
	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 18
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: mandurisSecretQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	if c.mandurisSecretLevelUp() {
		t.Fatal("The Manduri's Secret unlocked below level 19")
	}
	p.level = 19
	if !c.mandurisSecretLevelUp() || p.quest(mandurisSecretQuestID).Status != "START" || c.mandurisSecretLevelUp() {
		t.Fatalf("locked-to-start level transition failed or repeated: %+v", p.quest(mandurisSecretQuestID))
	}

	npc := func(id int32, objectID int32) *object {
		object := questCatalogNPC(s, p, id, objectID)
		object.npc = d.Npcs[id]
		if object.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		return object
	}
	aurelius := npc(mandurisAureliusNPC, 0x31031)
	selectDialog := func(o *object, dialog uint16) {
		c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, mandurisSecretQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, aurelius.id, 0, 0)) // Java has no case -1: the main menu
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(aurelius.id, 10, 0).Data) {
		t.Fatalf("Aurelius click = %x", got)
	}
	selectDialog(aurelius, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(aurelius.id, 1011, mandurisSecretQuestID).Data) {
		t.Fatalf("Aurelius opening page = %x", got)
	}
	selectDialog(aurelius, 10000)
	if questVar(p.quest(mandurisSecretQuestID).Vars, 0) != 1 {
		t.Fatal("Aurelius did not start the Manduri hunt")
	}
	for index, npcID := range []int32{210770, 210771, 210759, 210758, 210770, 210771} {
		dead := &object{npc: d.Npcs[npcID]}
		if dead.npc == nil {
			t.Fatalf("Manduri NPC template %d is missing", npcID)
		}
		s.recordQuestKill(dead, p)
		if got, want := questVar(p.quest(mandurisSecretQuestID).Vars, 0), int32(index+2); got != want {
			t.Fatalf("Manduri kill %d set variable %d, want %d", index+1, got, want)
		}
	}
	selectDialog(aurelius, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(aurelius.id, 1352, mandurisSecretQuestID).Data) {
		t.Fatalf("Aurelius hunt report page = %x", got)
	}
	selectDialog(aurelius, 10001)
	if questVar(p.quest(mandurisSecretQuestID).Vars, 0) != 8 {
		t.Fatal("Aurelius did not advance the hunt report")
	}
	archelaos := npc(mandurisArchelaosNPC, 0x31032)
	selectDialog(archelaos, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(archelaos.id, 1693, mandurisSecretQuestID).Data) {
		t.Fatalf("Archelaos page = %x", got)
	}
	selectDialog(archelaos, 10002)
	if questVar(p.quest(mandurisSecretQuestID).Vars, 0) != 9 {
		t.Fatal("Archelaos did not send the player to the glider")
	}

	glider := npc(mandurisGliderNPC, 0x31033)
	p.targetID = glider.id
	c.showDialog(dialogRequest(cmShowDialog, glider.id, 0, 0))
	if questVar(p.quest(mandurisSecretQuestID).Vars, 0) != 10 ||
		!bytes.Equal(packets.last(smUseObject), useObject(p.ID, glider.id, 1).Data) {
		t.Fatalf("glider interaction did not start: quest=%+v task=%v packet=%x", p.quest(mandurisSecretQuestID), glider.useTask != nil, packets.last(smUseObject))
	}

	melginie := npc(mandurisMelginieNPC, 0x31034)
	selectDialog(melginie, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(melginie.id, 2375, mandurisSecretQuestID).Data) {
		t.Fatalf("Melginie page = %x", got)
	}
	selectDialog(melginie, 10004)
	if questVar(p.quest(mandurisSecretQuestID).Vars, 0) != 12 {
		t.Fatal("Melginie did not advance the quest past the skipped escort")
	}
	celestine := npc(mandurisCelestineNPC, 0x31035)
	selectDialog(celestine, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(celestine.id, 3057, mandurisSecretQuestID).Data) {
		t.Fatalf("Celestine page = %x", got)
	}
	selectDialog(celestine, 10006)
	if p.quest(mandurisSecretQuestID).Status != "REWARD" {
		t.Fatalf("Celestine did not unlock the reward: %+v", p.quest(mandurisSecretQuestID))
	}
	c.showDialog(dialogRequest(cmShowDialog, aurelius.id, 0, 0))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(aurelius.id, 3398, mandurisSecretQuestID).Data) {
		t.Fatalf("Aurelius reward preview = %x", got)
	}
	selectDialog(aurelius, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(aurelius.id, 5, mandurisSecretQuestID).Data) {
		t.Fatalf("Aurelius reward selection = %x", got)
	}
	selectDialog(aurelius, 8)
	if quest := p.quest(mandurisSecretQuestID); quest.Status != "COMPLETE" || quest.CompleteCount != 1 {
		t.Fatalf("reward did not complete The Manduri's Secret: %+v", quest)
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[0].ID); got != 1 {
		t.Errorf("selected quest reward count = %d, want 1", got)
	}
}
