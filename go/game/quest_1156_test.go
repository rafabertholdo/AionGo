package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestStolenVillageSealObjectFlowAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[stolenVillageSealQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 14 || len(template.FinishedQuestConditions) != 0 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 14900 {
		t.Fatalf("unexpected Stolen Village Seal metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 14
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: stolenVillageSealQuestID, Kind: data.QuestCustom, StartNPC: 203128, EndNPC: 798003, NPCStart: true,
		TalkNPCs: []int32{700003}}
	start := questCatalogNPC(s, p, 203128, 0x61156)
	seal := questCatalogNPC(s, p, 700003, 0x61157)
	end := questCatalogNPC(s, p, 798003, 0x61158)

	if !c.stolenVillageSealDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, stolenVillageSealQuestID).Data) {
		t.Fatalf("quest offer = %x", packets.last(smDialogWindow))
	}
	if c.stolenVillageSealDialog(seal, script, 1002) {
		t.Fatal("field object started the quest")
	}
	if !c.stolenVillageSealDialog(start, script, 1002) || p.quest(stolenVillageSealQuestID) == nil || p.quest(stolenVillageSealQuestID).Status != "START" {
		t.Fatalf("quest did not start: %+v", p.quest(stolenVillageSealQuestID))
	}
	if c.stolenVillageSealDialog(start, script, 1002) {
		t.Fatal("quest start repeated")
	}
	if !c.stolenVillageSealDialog(seal, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(seal.id, 1352, stolenVillageSealQuestID).Data) {
		t.Fatalf("seal page = %x", packets.last(smDialogWindow))
	}
	if c.stolenVillageSealDialog(seal, script, 9999) || questVar(p.quest(stolenVillageSealQuestID).Vars, 0) != 0 {
		t.Fatal("invalid seal dialog advanced the quest")
	}
	p.targetID = seal.id
	if !c.stolenVillageSealDialog(seal, script, 1353) || seal.useTask == nil || !bytes.Equal(packets.last(smUseObject), useObject(p.ID, seal.id, 1).Data) {
		t.Fatal("seal search animation did not start")
	}
	if c.stolenVillageSealDialog(seal, script, 1353) {
		t.Fatal("duplicate seal use started a second animation")
	}
	time.Sleep(3200 * time.Millisecond)
	if seal.useTask != nil || questVar(p.quest(stolenVillageSealQuestID).Vars, 0) != 0 ||
		!bytes.Equal(packets.last(smUseObject), useObject(p.ID, seal.id, 0).Data) ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(seal.id, 1353, stolenVillageSealQuestID).Data) {
		t.Fatalf("seal animation result: task=%v quest=%+v", seal.useTask, p.quest(stolenVillageSealQuestID))
	}
	if !c.stolenVillageSealDialog(seal, script, 10000) || p.quest(stolenVillageSealQuestID).Status != "REWARD" || questVar(p.quest(stolenVillageSealQuestID).Vars, 0) != 1 {
		t.Fatalf("seal report did not open reward: %+v", p.quest(stolenVillageSealQuestID))
	}
	if c.stolenVillageSealDialog(seal, script, 10000) {
		t.Fatal("repeated seal report advanced the quest")
	}
	if !c.stolenVillageSealRewardShowDialog(end, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2375, stolenVillageSealQuestID).Data) {
		t.Fatal("reward preview failed")
	}
	if !c.stolenVillageSealDialog(end, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, stolenVillageSealQuestID).Data) {
		t.Fatal("reward confirmation page failed")
	}
	beforeExp := p.Exp
	if !c.stolenVillageSealDialog(end, script, 17) || p.quest(stolenVillageSealQuestID).Status != "COMPLETE" ||
		p.Exp-beforeExp != template.Rewards[0].Experience {
		t.Fatalf("reward mismatch: %+v exp=%d", p.quest(stolenVillageSealQuestID), p.Exp-beforeExp)
	}
	if c.stolenVillageSealDialog(end, script, 17) || p.quest(stolenVillageSealQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward dialog completed the quest twice")
	}
}
