package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestHuntingLepharistRevolutionariesProgression(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 9
	p.Exp = d.ExpStart(p.level)
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: huntingLepharistRevolutionariesQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: huntingLepharistRevolutionariesQuestID, Kind: data.QuestCustom, EndNPC: 203126}
	erytes := questCatalogNPC(s, p, 203126, 0x31013)

	if c.huntingLepharistRevolutionariesLevelUp() {
		t.Fatal("quest unlocked below its minimum level")
	}
	p.level = 10
	if !c.huntingLepharistRevolutionariesLevelUp() || c.huntingLepharistRevolutionariesLevelUp() || p.quest(1013).Status != "START" {
		t.Fatalf("level-up transition = %+v", p.quest(1013))
	}
	if c.huntingLepharistRevolutionariesDialog(erytes, script, 9999) || c.huntingLepharistRevolutionariesKill(210688) {
		t.Fatal("quest advanced before its opening conversation")
	}
	if !c.huntingLepharistRevolutionariesDialog(erytes, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(erytes.id, 1011, 1013).Data) {
		t.Fatalf("opening conversation = %x", packets.last(smDialogWindow))
	}
	if !c.huntingLepharistRevolutionariesDialog(erytes, script, 1012) || !bytes.Equal(packets.last(smPlayMovie), playMovie(25).Data) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(erytes.id, 1012, 1013).Data) {
		t.Fatal("opening movie dialog did not match the Java event")
	}
	if !c.huntingLepharistRevolutionariesDialog(erytes, script, 10000) || c.huntingLepharistRevolutionariesDialog(erytes, script, 10000) || questVar(p.quest(1013).Vars, 0) != 1 {
		t.Fatalf("first hunt stage = %+v", p.quest(1013))
	}
	if c.huntingLepharistRevolutionariesKill(210316) || c.huntingLepharistRevolutionariesKill(210687) {
		t.Fatal("wrong kill target advanced the first hunt stage")
	}
	for kill := 0; kill < 11; kill++ {
		if !c.huntingLepharistRevolutionariesKill(210688) {
			t.Fatalf("revolutionary kill %d did not advance", kill+1)
		}
	}
	if got := questVar(p.quest(1013).Vars, 0); got != 12 || c.huntingLepharistRevolutionariesKill(210688) {
		t.Fatalf("revolutionary kill boundary = %d", got)
	}
	if !c.huntingLepharistRevolutionariesDialog(erytes, script, 25) || p.quest(1013).Status != "REWARD" || questVar(p.quest(1013).Vars, 0) != 12 {
		t.Fatalf("quest did not enter its Java dialogue reward branch: %+v", p.quest(1013))
	}
	if c.huntingLepharistRevolutionariesKill(210316) {
		t.Fatal("kill event advanced after reward status")
	}
}

func TestHuntingLepharistRevolutionariesOutlawKillAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[huntingLepharistRevolutionariesQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].Items) != 2 {
		t.Fatal("Hunting Lepharist Revolutionaries fixed rewards are missing from quest metadata")
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: huntingLepharistRevolutionariesQuestID, Status: "START", Vars: 12}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: huntingLepharistRevolutionariesQuestID, Kind: data.QuestCustom, EndNPC: 203126}
	erytes := questCatalogNPC(s, p, 203126, 0x32013)

	if c.huntingLepharistRevolutionariesKill(210688) || p.quest(1013).Status != "START" {
		t.Fatal("extra revolutionary kill advanced at the outlaw stage")
	}
	if !c.huntingLepharistRevolutionariesKill(210316) || c.huntingLepharistRevolutionariesKill(210316) || p.quest(1013).Status != "REWARD" {
		t.Fatalf("outlaw kill transition = %+v", p.quest(1013))
	}
	if !c.huntingLepharistRevolutionariesDialog(erytes, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(erytes.id, 5, 1013).Data) {
		t.Fatalf("reward dialog = %x", packets.last(smDialogWindow))
	}
	beforeExp := p.Exp
	beforeKinah := p.kinah.Count
	if !c.huntingLepharistRevolutionariesDialog(erytes, script, 17) {
		t.Fatal("valid fixed-reward completion dialog was not handled")
	}
	if q := p.quest(1013); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || p.kinah.Count-beforeKinah != template.Rewards[0].Kinah {
		t.Fatalf("quest rewards mismatch: quest=%+v exp=%d kinah=%d", q, p.Exp-beforeExp, p.kinah.Count-beforeKinah)
	}
	for _, item := range template.Rewards[0].Items {
		if got := s.countItems(p, item.ID); got != item.Count {
			t.Fatalf("reward item %d count = %d, want %d", item.ID, got, item.Count)
		}
	}
	if c.huntingLepharistRevolutionariesDialog(erytes, script, 17) || p.quest(1013).CompleteCount != 1 {
		t.Fatal("repeated reward dialog completed the quest twice")
	}
}
