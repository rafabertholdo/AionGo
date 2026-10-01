package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestKaliosCallZoneDialogAndLockedFollowUps(t *testing.T) {
	d := staticDataOrSkip(t)
	d.QuestScripts[1100] = &data.QuestScript{ID: 1100, Kind: data.QuestCustom, EndNPC: 203067}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	kalio := questCatalogNPC(s, p, 203067, 0x30001)
	script := d.QuestScripts[1100]

	if c.kaliosCallEnterZone("OTHER") || p.quest(1100) != nil {
		t.Fatal("unrelated zone started quest")
	}
	if !c.kaliosCallEnterZone("AKARIOS_VILLAGE") || p.quest(1100).Status != "START" {
		t.Fatal("Akarios Village did not start quest")
	}
	if c.kaliosCallEnterZone("AKARIOS_VILLAGE") {
		t.Fatal("repeated entry restarted quest")
	}
	if c.kaliosCallShowDialog(kalio, script) {
		t.Fatal("inactive reward preview opened")
	}
	c.kaliosCallDialog(kalio, script, 1007)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 4, 1100).Data) {
		t.Fatalf("start dialog = %x", got)
	}
	c.kaliosCallDialog(kalio, script, 1003)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 1004, 1100).Data) {
		t.Fatalf("decline dialog = %x", got)
	}
	c.kaliosCallDialog(kalio, script, 17)
	if p.quest(1100).Status != "START" || p.quest(1001) != nil {
		t.Fatal("early reward advanced quest")
	}
	c.kaliosCallDialog(kalio, script, 25)
	if q := p.quest(1100); q.Status != "REWARD" || q.Vars != 1 {
		t.Fatalf("Kalio dialog state = %+v", q)
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 1011, 1100).Data) {
		t.Fatalf("Kalio dialog = %x", got)
	}
	if !c.kaliosCallShowDialog(kalio, script) {
		t.Fatal("reward preview did not open")
	}
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 5, 1100).Data) {
		t.Fatalf("reward preview = %x", got)
	}
	c.kaliosCallDialog(kalio, script, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(kalio.id, 5, 1100).Data) {
		t.Fatalf("reward dialog = %x", got)
	}
	beforeExp := p.Exp
	c.kaliosCallDialog(kalio, script, 17)
	if q := p.quest(1100); q.Status != "COMPLETE" || q.CompleteCount != 1 {
		t.Fatalf("completion = %+v", q)
	}
	if p.Exp-beforeExp != 10 {
		t.Fatalf("experience reward = %d", p.Exp-beforeExp)
	}
	for _, id := range []int32{1001, 1002, 1003, 1004, 1005} {
		want := "START"
		if id == 1005 {
			want = "LOCKED"
		}
		if q := p.quest(id); q == nil || q.Status != want || q.Vars != 0 {
			t.Fatalf("follow-up %d = %+v", id, q)
		}
	}
	c.kaliosCallDialog(kalio, script, 17)
	if p.Exp != beforeExp+10 || p.quest(1100).CompleteCount != 1 {
		t.Fatal("repeated completion paid reward")
	}
}

func TestKaliosCallRejectsWrongRaceAndNPC(t *testing.T) {
	d := staticDataOrSkip(t)
	d.QuestScripts[1100] = &data.QuestScript{ID: 1100, Kind: data.QuestCustom, EndNPC: 203067}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ASMODIANS"
	p.level = 4
	p.seen = map[int32]*object{}
	c := &conn{s: s, player: p}
	p.conn = c
	if c.kaliosCallEnterZone("AKARIOS_VILLAGE") || p.quest(1100) != nil {
		t.Fatal("wrong race started quest")
	}
	p.Race = "ELYOS"
	if !c.kaliosCallEnterZone("AKARIOS_VILLAGE") {
		t.Fatal("eligible player did not start quest")
	}
	other := questCatalogNPC(s, p, 203075, 0x30002)
	c.kaliosCallDialog(other, d.QuestScripts[1100], 25)
	if p.quest(1100).Status != "START" {
		t.Fatal("wrong NPC advanced quest")
	}
}
