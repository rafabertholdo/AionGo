package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSummonsToTheCitadelZoneStartAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[summonsToCitadelQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 10 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 1500 {
		t.Fatalf("unexpected Summons to the Citadel metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.WorldID = 210030000
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: summonsToCitadelQuestID, Kind: data.QuestCustom, StartNPC: summonsToCitadelNPC, EndNPC: summonsToCitadelNPC}
	aegir := questCatalogNPC(s, p, summonsToCitadelNPC, 0x31130)

	if c.summonsToCitadelEnterZone("OTHER_ZONE") || p.quest(summonsToCitadelQuestID) != nil {
		t.Fatal("unrelated zone started the quest")
	}
	if !c.summonsToCitadelEnterZone("VERTERON_CITADEL") || p.quest(summonsToCitadelQuestID) == nil || p.quest(summonsToCitadelQuestID).Status != "START" {
		t.Fatal("citadel entry did not start the quest")
	}
	if c.summonsToCitadelEnterZone("VERTERON_CITADEL") {
		t.Fatal("repeated citadel entry started a duplicate quest")
	}
	if c.summonsToCitadelDialog(questCatalogNPC(s, p, 203097, 0x31131), script, 25) || c.summonsToCitadelDialog(aegir, script, 10000) {
		t.Fatal("wrong NPC or dialog advanced the quest")
	}
	if !c.summonsToCitadelDialog(aegir, script, 25) || p.quest(summonsToCitadelQuestID).Status != "REWARD" || questVar(p.quest(summonsToCitadelQuestID).Vars, 0) != 1 {
		t.Fatal("Aegir's report did not open the reward")
	}
	if !bytes.Equal(packets.last(smDialogWindow), dialogWindow(aegir.id, 1011, summonsToCitadelQuestID).Data) {
		t.Fatalf("report dialog = %x", packets.last(smDialogWindow))
	}
	if !c.summonsToCitadelDialog(aegir, script, ^uint16(0)) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(aegir.id, 5, summonsToCitadelQuestID).Data) {
		t.Fatal("reward preview did not show")
	}
	beforeExp := p.Exp
	if !c.summonsToCitadelDialog(aegir, script, 17) {
		t.Fatal("reward selection did not complete")
	}
	if q := p.quest(summonsToCitadelQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience {
		t.Fatalf("quest reward mismatch: quest=%+v gained experience=%d", q, p.Exp-beforeExp)
	}
	for id := int32(1011); id <= 1023; id++ {
		if p.quest(id) == nil {
			t.Fatalf("Verteron follow-up quest %d was not created", id)
		}
	}
	if c.summonsToCitadelDialog(aegir, script, 17) || p.quest(summonsToCitadelQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward selection completed the quest twice")
	}
}

func TestSummonsToTheCitadelLocksFollowupsWithoutReplacingState(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.quests = []store.Quest{
		{ID: 1012, Status: "START", Vars: 4},
		{ID: summonsToCitadelQuestID, Status: "REWARD", Vars: 1},
	}
	c := &conn{s: s, player: p}
	c.summonsToCitadelLockFollowups()
	if q := p.quest(1012); q == nil || q.Status != "START" || q.Vars != 4 {
		t.Fatalf("existing follow-up state was overwritten: %+v", q)
	}
	for id := int32(1011); id <= 1023; id++ {
		if id == 1012 {
			continue
		}
		if q := p.quest(id); q == nil || q.Status != "LOCKED" {
			t.Fatalf("follow-up %d lock state = %+v", id, q)
		}
	}
}
