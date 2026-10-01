package game

import (
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestNymphsGownAsterosRewardPath(t *testing.T) {
	d := staticDataOrSkip(t)
	s, p, c, packets := nymphsGownFixture(t, d)
	script := d.QuestScripts[1114]
	if script == nil || script.Kind != data.QuestCustom || len(d.QuestItemUses[nymphsDiaryItemID]) != 1 {
		t.Fatalf("Nymph's Gown is not registered: %+v", script)
	}
	if !s.addItem(p, nymphsDiaryItemID, 1) {
		t.Fatal("could not add Namus's diary")
	}
	diary := p.cube[len(p.cube)-1]
	c.nymphsGownItemUse(diary, script)
	if packets.last(smDialogWindow) == nil || p.quest(1114) != nil {
		t.Fatal("using diary should ask for quest acceptance first")
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, 0, 1002, 1114))
	if q := p.quest(1114); q == nil || q.Status != "START" || s.countItems(p, nymphsDiaryItemID) != 0 || s.countItems(p, nymphsLetterItemID) != 1 {
		t.Fatalf("diary acceptance state: quest=%+v", q)
	}
	namus := questCatalogNPC(s, p, 203075, 0x31141)
	selectQuestDialog(c, namus, 25, 1114)
	selectQuestDialog(c, namus, 10000, 1114)
	if q := p.quest(1114); questVar(q.Vars, 0) != 1 || s.countItems(p, nymphsLetterItemID) != 0 {
		t.Fatalf("Namus did not take the letter: %+v", q)
	}
	clothes := questCatalogNPC(s, p, 700008, 0x31142)
	if !c.nymphsGownDialog(clothes, script, -1) {
		t.Fatal("Seirenia's clothes did not start the object-use sequence")
	}
	time.Sleep(3100 * time.Millisecond)
	if q := p.quest(1114); questVar(q.Vars, 0) != 2 || s.countItems(p, nymphsDressItemID) != 1 || packets.last(smUseObject) == nil {
		t.Fatalf("clothes interaction did not grant dress: quest=%+v", q)
	}
	selectQuestDialog(c, namus, 10001, 1114)
	asteros := questCatalogNPC(s, p, 203058, 0x31143)
	selectQuestDialog(c, asteros, 25, 1114)
	if !c.nymphsGownDialog(asteros, script, 10002) {
		t.Fatal("Asteros reward choice was not handled")
	}
	if q := p.quest(1114); q.Status != "REWARD" || s.countItems(p, nymphsDressItemID) != 0 {
		t.Fatalf("Asteros reward state: %+v", q)
	}
	beforeExp, beforeKinah := p.Exp, p.kinah.Count
	c.nymphsGownDialog(asteros, script, 17)
	reward := d.Quests[1114].Rewards[1]
	if q := p.quest(1114); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != reward.Experience || p.kinah.Count-beforeKinah != reward.Kinah {
		t.Fatalf("Asteros reward mismatch: quest=%+v exp=%d kinah=%d", q, p.Exp-beforeExp, p.kinah.Count-beforeKinah)
	}
}

func TestNymphsGownNamusRewardPathAndInvalidEvents(t *testing.T) {
	d := staticDataOrSkip(t)
	_, p, c, _ := nymphsGownFixture(t, d)
	script := d.QuestScripts[1114]
	p.quests = []store.Quest{{ID: 1114, Status: "START", Vars: 3}}
	namus := questCatalogNPC(c.s, p, 203075, 0x31151)
	if c.nymphsGownDialog(namus, script, 10000) || c.nymphsGownDialog(namus, script, 10001) {
		t.Fatal("Namus accepted a stale step")
	}
	if !c.nymphsGownDialog(namus, script, 1009) || p.quest(1114).Status != "REWARD" {
		t.Fatal("Namus did not advance to reward")
	}
	beforeExp, beforeKinah := p.Exp, p.kinah.Count
	c.nymphsGownDialog(namus, script, 17)
	reward := d.Quests[1114].Rewards[0]
	if q := p.quest(1114); q.Status != "COMPLETE" || p.Exp-beforeExp != reward.Experience || p.kinah.Count-beforeKinah != reward.Kinah {
		t.Fatalf("Namus reward mismatch: quest=%+v exp=%d kinah=%d", q, p.Exp-beforeExp, p.kinah.Count-beforeKinah)
	}
}

func nymphsGownFixture(t *testing.T, d *data.Data) (*Server, *player, *conn, *questPackets) {
	t.Helper()
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.seen = map[int32]*object{}
	p.spawned = true
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	return s, p, c, packets
}
