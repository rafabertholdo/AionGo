package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestKrallDesecrationLevelUpPrerequisites(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.level = 13
	p.quests = []store.Quest{
		{ID: krallDesecrationQuestID, Status: "LOCKED"},
		{ID: 1017, Status: "LOCKED"},
	}
	c := &conn{s: s, player: p, tap: (&questPackets{}).tap}
	p.conn = c
	if c.krallDesecrationLevelUp() {
		t.Fatal("unlocked below the minimum level or before Held Sacred completed")
	}
	p.level = 14
	if c.krallDesecrationLevelUp() {
		t.Fatal("unlocked before Held Sacred completed")
	}
	p.quest(1017).Status = "COMPLETE"
	if !c.krallDesecrationLevelUp() || p.quest(krallDesecrationQuestID).Status != "START" || c.krallDesecrationLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(krallDesecrationQuestID))
	}
}

func TestKrallDesecrationOrderedKillsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[krallDesecrationQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 14 || len(template.Rewards) != 1 ||
		template.Rewards[0].Experience != 86400 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0].ID != 123000879 {
		t.Fatalf("unexpected Krall Desecration metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 14
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.quests = []store.Quest{{ID: krallDesecrationQuestID, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: krallDesecrationQuestID, Kind: data.QuestCustom, StartNPC: 203178, EndNPC: 203178}
	captain := questCatalogNPC(s, p, 203178, 0x61022)
	wrongNPC := questCatalogNPC(s, p, 203179, 0x61023)

	if c.krallDesecrationDialog(wrongNPC, script, 25) || c.krallDesecrationDialog(captain, script, 9999) {
		t.Fatal("wrong NPC or premature event was accepted")
	}
	if !c.krallDesecrationDialog(captain, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(captain.id, 1011, krallDesecrationQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	if c.krallDesecrationDialog(captain, script, 10002) || questVar(p.quest(krallDesecrationQuestID).Vars, 0) != 0 {
		t.Fatal("invalid dialog advanced the quest")
	}
	if !c.krallDesecrationDialog(captain, script, 10001) || questVar(p.quest(krallDesecrationQuestID).Vars, 0) != 1 {
		t.Fatalf("captain did not start the hunt: %+v", p.quest(krallDesecrationQuestID))
	}
	if c.krallDesecrationDialog(captain, script, 10000) {
		t.Fatal("repeated starter dialog was accepted")
	}
	if c.krallDesecrationKill(210179) || questVar(p.quest(krallDesecrationQuestID).Vars, 0) != 1 {
		t.Fatal("wrong monster advanced the kill count")
	}
	for kill := int32(1); kill <= 5; kill++ {
		if !c.krallDesecrationKill(210178) {
			t.Fatalf("kill %d was not recorded", kill)
		}
		quest := p.quest(krallDesecrationQuestID)
		if kill < 5 && (quest.Status != "START" || questVar(quest.Vars, 0) != kill+1) {
			t.Fatalf("kill %d state = %+v", kill, quest)
		}
		if kill == 5 && (quest.Status != "REWARD" || questVar(quest.Vars, 0) != 5) {
			t.Fatalf("fifth kill did not open reward: %+v", quest)
		}
	}
	if c.krallDesecrationKill(210178) {
		t.Fatal("repeated kill advanced a reward quest")
	}
	if !c.krallDesecrationShowDialog(captain, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(captain.id, 5, krallDesecrationQuestID).Data) {
		t.Fatal("reward preview failed")
	}
	if !c.krallDesecrationDialog(captain, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(captain.id, 5, krallDesecrationQuestID).Data) {
		t.Fatal("reward confirmation page failed")
	}
	if c.krallDesecrationDialog(captain, script, 7) || p.quest(krallDesecrationQuestID).Status != "REWARD" {
		t.Fatal("out-of-range reward dialog was accepted")
	}
	beforeExp := p.Exp
	if !c.krallDesecrationDialog(captain, script, 8) || p.quest(krallDesecrationQuestID).Status != "COMPLETE" ||
		p.Exp-beforeExp != template.Rewards[0].Experience || s.countItems(p, 123000879) != 1 {
		t.Fatalf("quest reward mismatch: %+v exp=%d item=%d", p.quest(krallDesecrationQuestID), p.Exp-beforeExp, s.countItems(p, 123000879))
	}
	if c.krallDesecrationDialog(captain, script, 17) || p.quest(krallDesecrationQuestID).CompleteCount != 1 || s.countItems(p, 123000879) != 1 {
		t.Fatal("repeated reward duplicated completion or item")
	}
}
