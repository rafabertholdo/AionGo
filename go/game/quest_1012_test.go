package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestMaskedLoiterersLevelZoneTurnInAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[1012]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 10 || len(template.Rewards) == 0 {
		t.Fatalf("unexpected Masked Loiterers metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 9
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1012, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1012, Kind: data.QuestCustom, StartNPC: 203111, EndNPC: 203111}
	npc := questCatalogNPC(s, p, 203111, 0x31012)

	if c.maskedLoiterersLevelUp() {
		t.Fatal("quest unlocked below its minimum level")
	}
	p.level = 10
	if !c.maskedLoiterersLevelUp() || p.quest(1012).Status != "START" || c.maskedLoiterersLevelUp() {
		t.Fatalf("level-up transition failed or repeated: %+v", p.quest(1012))
	}
	c.maskedLoiterersDialog(npc, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1011, 1012).Data) {
		t.Fatalf("initial conversation = %x", got)
	}
	c.maskedLoiterersDialog(npc, script, 10000)
	if q := p.quest(1012); q.Vars != 1 || q.Status != "START" {
		t.Fatalf("initial conversation did not advance: %+v", q)
	}
	if c.maskedLoiterersEnterZone("WRONG_ZONE") || p.quest(1012).Vars != 1 {
		t.Fatal("wrong zone advanced the quest")
	}
	if !c.maskedLoiterersEnterZone("Q1012") || p.quest(1012).Vars != 2 || c.maskedLoiterersEnterZone("Q1012") {
		t.Fatalf("Q1012 entry failed or repeated: %+v", p.quest(1012))
	}
	c.maskedLoiterersDialog(npc, script, 25)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1352, 1012).Data) {
		t.Fatalf("zone report = %x", got)
	}
	c.maskedLoiterersDialog(npc, script, 10001)
	if q := p.quest(1012); q.Vars != 3 || q.Status != "START" {
		t.Fatalf("zone report did not advance: %+v", q)
	}
	if !s.addItem(p, maskedLoiterersItemID, 4) {
		t.Fatal("could not add test quest items")
	}
	c.maskedLoiterersDialog(npc, script, 10001)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1779, 1012).Data) || p.quest(1012).Status != "START" {
		t.Fatalf("insufficient items did not block turn-in: packet=%x quest=%+v", got, p.quest(1012))
	}
	if !s.addItem(p, maskedLoiterersItemID, 2) {
		t.Fatal("could not add remaining quest items")
	}
	c.maskedLoiterersDialog(npc, script, 33)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1694, 1012).Data) || p.quest(1012).Vars != 3 {
		t.Fatalf("turn-in confirmation = %x, quest=%+v", got, p.quest(1012))
	}
	c.maskedLoiterersDialog(npc, script, 10001)
	if q := p.quest(1012); q.Status != "REWARD" || q.Vars != 4 || s.countItems(p, maskedLoiterersItemID) != 0 {
		t.Fatalf("turn-in failed to consume items and reach reward: %+v items=%d", q, s.countItems(p, maskedLoiterersItemID))
	}
	c.maskedLoiterersDialog(npc, script, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 5, 1012).Data) {
		t.Fatalf("reward preview = %x", got)
	}
	beforeExp := p.Exp
	c.maskedLoiterersDialog(npc, script, 17)
	reward := template.Rewards[0]
	if q := p.quest(1012); q.Status != "COMPLETE" || p.Exp-beforeExp != reward.Experience {
		t.Fatalf("quest reward mismatch: quest=%+v gained experience=%d expected=%d", q, p.Exp-beforeExp, reward.Experience)
	}
	for _, item := range reward.Items {
		if got := s.countItems(p, item.ID); got < item.Count {
			t.Fatalf("reward item %d count=%d, want at least %d", item.ID, got, item.Count)
		}
	}
	c.maskedLoiterersDialog(npc, script, 17)
	if p.quest(1012).CompleteCount != 1 {
		t.Fatal("repeated reward dialog completed quest again")
	}
}
