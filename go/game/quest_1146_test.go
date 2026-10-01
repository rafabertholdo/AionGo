package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func delicateMandrakeTestFixture(t *testing.T, status string) (*Server, *player, *conn, *data.QuestScript, *questPackets) {
	t.Helper()
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Class = "SORCERER"
	p.Race, p.Class = "ELYOS", "SORCERER"
	p.level = 12
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = nil
	if status != "" {
		p.quests = append(p.quests, store.Quest{ID: delicateMandrakeQuestID, Status: status})
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: delicateMandrakeQuestID, Kind: data.QuestCustom,
		StartNPC: delicateMandrakeStartNPCID, EndNPC: delicateMandrakeEndNPCID, NPCStart: true}
	return s, p, c, script, packets
}

func TestDelicateMandrakeStartAndInventoryGuard(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[delicateMandrakeQuestID]
	if template == nil || template.MinLevel != 12 || template.Race != "ELYOS" || len(template.QuestWorkItems) != 1 || template.QuestWorkItems[0].ID != delicateMandrakeWorkItemID {
		t.Fatal("Delicate Mandrake work-item metadata is missing")
	}
	s, p, c, script, packets := delicateMandrakeTestFixture(t, "")
	start := questCatalogNPC(s, p, delicateMandrakeStartNPCID, 0x31146)
	if !c.delicateMandrakeDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, delicateMandrakeQuestID).Data) {
		t.Fatalf("quest offer = %x", packets.last(smDialogWindow))
	}
	if !c.delicateMandrakeDialog(start, script, 1007) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 4, delicateMandrakeQuestID).Data) {
		t.Fatalf("quest details = %x", packets.last(smDialogWindow))
	}
	if !c.delicateMandrakeDialog(start, script, 1003) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1004, delicateMandrakeQuestID).Data) {
		t.Fatalf("pre-accept dialog = %x", packets.last(smDialogWindow))
	}
	for len(p.cube) < p.cubeLimit() {
		p.cube = append(p.cube, &store.Item{UniqueID: int32(len(p.cube)) + 0x60000, ItemID: data.Kinah, Count: 1})
	}
	if !c.delicateMandrakeDialog(start, script, 1002) || p.quest(delicateMandrakeQuestID) != nil || s.countItems(p, delicateMandrakeWorkItemID) != 0 || packets.last(smSystemMessage) == nil {
		t.Fatal("full inventory accepted the quest without its timer work item")
	}
	p.cube = []*store.Item{}
	if !c.delicateMandrakeDialog(start, script, 1002) {
		t.Fatal("NPC did not accept the quest")
	}
	if q := p.quest(delicateMandrakeQuestID); q == nil || q.Status != "START" || s.countItems(p, delicateMandrakeWorkItemID) != 1 {
		t.Fatalf("quest acceptance = %+v, work item=%d", q, s.countItems(p, delicateMandrakeWorkItemID))
	}
	if !bytes.Equal(packets.last(smQuestAccepted), delicateMandrakeTimerPacket(delicateMandrakeTimerSeconds).Data) {
		t.Fatalf("quest timer packet = %x", packets.last(smQuestAccepted))
	}
	if c.delicateMandrakeDialog(start, script, 1002) {
		t.Fatal("repeated acceptance was handled while quest was already active")
	}
}

func TestDelicateMandrakeTurnInAndRewards(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[delicateMandrakeQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].Items) != 0 || template.Rewards[0].Experience != 7120 || template.Rewards[0].Kinah != 460 {
		t.Fatal("Delicate Mandrake fixed reward metadata is missing")
	}
	s, p, c, script, packets := delicateMandrakeTestFixture(t, "START")
	p.quests[0].Vars = 0
	end := questCatalogNPC(s, p, delicateMandrakeEndNPCID, 0x32146)
	if !s.addItem(p, delicateMandrakeWorkItemID, 1) {
		t.Fatal("could not add the quest timer work item")
	}
	if !c.delicateMandrakeDialog(end, script, -1) || p.quest(delicateMandrakeQuestID).Status != "START" || packets.last(smDialogWindow) == nil {
		t.Fatal("missing mandrake item changed quest status")
	}
	if !s.addItem(p, delicateMandrakeTurnInItemID, 1) {
		t.Fatal("could not add the Java handler's required turn-in item")
	}
	if !c.delicateMandrakeDialog(end, script, -1) || p.quest(delicateMandrakeQuestID).Status != "REWARD" || questVar(p.quest(delicateMandrakeQuestID).Vars, 0) != 2 || s.countItems(p, delicateMandrakeTurnInItemID) != 0 {
		t.Fatalf("turn-in transition = %+v item=%d", p.quest(delicateMandrakeQuestID), s.countItems(p, delicateMandrakeTurnInItemID))
	}
	if !bytes.Equal(packets.last(smQuestAccepted), delicateMandrakeTimerPacket(0).Data) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, delicateMandrakeQuestID).Data) {
		t.Fatal("turn-in did not stop the timer and show the default reward dialog")
	}
	if c.delicateMandrakeDialog(end, script, 8) || p.quest(delicateMandrakeQuestID).Status != "REWARD" {
		t.Fatal("invalid fixed-reward choice completed the quest")
	}
	beforeExp, beforeKinah := p.Exp, p.kinah.Count
	if !c.delicateMandrakeDialog(end, script, 17) {
		t.Fatal("valid fixed-reward selection was not handled")
	}
	if q := p.quest(delicateMandrakeQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != template.Rewards[0].Experience || p.kinah.Count-beforeKinah != template.Rewards[0].Kinah {
		t.Fatalf("quest reward mismatch: %+v exp=%d kinah=%d", q, p.Exp-beforeExp, p.kinah.Count-beforeKinah)
	}
	if s.countItems(p, delicateMandrakeWorkItemID) != 1 {
		t.Fatal("Java handler did not leave its work item after successful timer cancellation")
	}
	if c.delicateMandrakeDialog(end, script, 17) || p.quest(delicateMandrakeQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward selection completed the quest twice")
	}
}

func TestDelicateMandrakeTimerExpiry(t *testing.T) {
	s, p, c, _, packets := delicateMandrakeTestFixture(t, "START")
	p.quests[0].Vars = 0
	if !s.addItem(p, delicateMandrakeWorkItemID, 1) {
		t.Fatal("could not add the temporary quest work item")
	}
	if !c.delicateMandrakeTimerEnd() {
		t.Fatal("active quest timer did not expire the quest")
	}
	if q := p.quest(delicateMandrakeQuestID); q.Status != "NONE" || q.Vars != 0 {
		t.Fatalf("timer expiry did not reset quest status: %+v", q)
	}
	if !bytes.Equal(packets.last(smQuestAccepted), questAccepted(3, store.Quest{ID: delicateMandrakeQuestID}).Data) || s.countItems(p, delicateMandrakeWorkItemID) != 1 {
		t.Fatal("timer expiry packet/work-item behavior differs from Java's deleteQuest path")
	}
	if c.delicateMandrakeTimerEnd() {
		t.Fatal("inactive quest timer expired the quest a second time")
	}
}
