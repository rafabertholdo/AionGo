package game

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// windows lists the (dialog id, quest id) of every SM_DIALOG_WINDOW sent since frame index from.
func (r *questPackets) windows(from int) [][2]int32 {
	var out [][2]int32
	for _, f := range r.frames[from:] {
		if f[0] == smDialogWindow {
			out = append(out, [2]int32{int32(binary.LittleEndian.Uint16(f[5:])), int32(binary.LittleEndian.Uint32(f[7:]))})
		}
	}
	return out
}

// kerubFixture is Wrathchild (Elyos, level 4) next to Muranes (203071) and the reward NPC Kalio (203067).
func kerubFixture(t *testing.T, status string, vars int32) (*Server, *player, *conn, *questPackets, *object, *object) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.cube = nil
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1001, Status: status, Vars: vars}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	return s, p, c, packets, questCatalogNPC(s, p, 203071, 0x31001), questCatalogNPC(s, p, 203067, 0x31002)
}

// TestKerubThreatMatchesRecordedJavaDialogs replays the dialog sequences the real 1.9 client had with the Java
// server (quest-debug run 2026-09-29, npc Muranes 203071): the main menu, the client's select 25, and every page
// the handler does not handle answered with the window of the same number.
func TestKerubThreatMatchesRecordedJavaDialogs(t *testing.T) {
	s, p, c, packets, muranes, kalio := kerubFixture(t, "LOCKED", 0)
	if c.kerubThreatKill(210670) || c.kerubThreatShowDialog(kalio, &data.QuestScript{ID: 1001}) {
		t.Fatal("locked quest accepted an event")
	}
	if !c.kerubThreatLevelUp() || p.quest(1001).Status != "START" || c.kerubThreatLevelUp() {
		t.Fatal("level transition failed or repeated")
	}
	from := len(packets.frames)
	step := func(name string, do func(), want ...[2]int32) {
		t.Helper()
		from = len(packets.frames)
		do()
		if got := packets.windows(from); !slices.Equal(got, want) {
			t.Fatalf("%s: windows = %v, want %v", name, got, want)
		}
	}
	show := func(o *object) func() {
		return func() { c.showDialog(dialogRequest(cmShowDialog, o.id, 0, 0)) }
	}
	sel := func(o *object, dialog uint16) func() {
		return func() { c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, 1001)) }
	}
	menu := [2]int32{10, 0}
	// Accepting: the recorded Java sequence 10, 25 -> 1011, 1012 -> movie + 1012, 1013 -> 1013, 10000 -> accepted + 10.
	step("show", show(muranes), menu)
	if packets.last(smLookatobject) == nil {
		t.Fatal("CM_SHOW_DIALOG did not turn the npc to the player")
	}
	step("select 25", sel(muranes, 25), [2]int32{1011, 1001})
	step("select 1012", sel(muranes, 1012), [2]int32{1012, 1001})
	if packets.last(smPlayMovie) == nil {
		t.Fatal("select 1012 did not play the movie")
	}
	step("select 1013", sel(muranes, 1013), [2]int32{1013, 1001})
	c.playMovieEnd(movieEnded(15))
	if p.quest(1001).Vars != 0 {
		t.Fatal("the movie or the pages advanced the quest; only select 10000 accepts it")
	}
	from = len(packets.frames)
	sel(muranes, 10000)()
	if q := p.quest(1001); q.Vars != 1 || q.Status != "START" {
		t.Fatalf("select 10000: %+v", q)
	}
	accepted := packets.frames[from]
	if accepted[0] != smQuestAccepted || !bytes.Equal(accepted, questAccepted(2, *p.quest(1001)).Data) {
		t.Fatalf("first packet after 10000 = %x", accepted)
	}
	if got := packets.windows(from); !slices.Equal(got, [][2]int32{menu}) {
		t.Fatalf("after 10000: %v", got)
	}
	// Kill Kerub six times, the variable reaching 6.
	for i := int32(2); i <= 6; i++ {
		if !c.kerubThreatKill(210670) || p.quest(1001).Vars != i {
			t.Fatalf("kill %d failed: %+v", i, p.quest(1001))
		}
	}
	if c.kerubThreatKill(210671) || c.kerubThreatKill(210670) {
		t.Fatal("wrong or extra kill advanced")
	}
	// The turn-in of the recorded run: 10, 25 -> 1352, 1353 -> 1353, 1354 -> 1354, 10001 -> var 7 + 10.
	step("show var 6", show(muranes), menu)
	step("select 25 var 6", sel(muranes, 25), [2]int32{1352, 1001})
	step("select 1353", sel(muranes, 1353), [2]int32{1353, 1001})
	step("select 1354", sel(muranes, 1354), [2]int32{1354, 1001})
	step("select 10001", sel(muranes, 10001), menu)
	if p.quest(1001).Vars != 7 {
		t.Fatalf("10001 did not advance: %+v", p.quest(1001))
	}
	// Collecting: 25 -> 1693, 33 without the items -> 1779 (recorded), with them -> 1694, 10002 -> reward.
	step("show var 7", show(muranes), menu)
	step("select 25 var 7", sel(muranes, 25), [2]int32{1693, 1001})
	step("select 33 without items", sel(muranes, 33), [2]int32{1779, 1001})
	step("select 10002 without items", sel(muranes, 10002), [2]int32{1779, 1001})
	if !s.addItem(p, 182200001, 6) {
		t.Fatal("could not add quest items")
	}
	step("select 33", sel(muranes, 33), [2]int32{1694, 1001})
	step("select 10002", sel(muranes, 10002), menu)
	if q := p.quest(1001); q.Status != "REWARD" || q.Vars != 8 || s.countItems(p, 182200001) != 0 {
		t.Fatalf("turn-in = %+v, items=%d", q, s.countItems(p, 182200001))
	}
	// Muranes no longer handles the quest in REWARD (Java returns false), so the framework echoes the page.
	step("repeat 10002", sel(muranes, 10002), [2]int32{10002, 1001})
	if p.quest(1001).Vars != 8 {
		t.Fatal("repeat turn-in advanced")
	}
	// A click on Muranes no longer offers the quest; Kalio's click is the reward window (defaultQuestEndDialog -1).
	step("show Muranes in REWARD", show(muranes), menu)
	step("show Kalio in REWARD", show(kalio), [2]int32{5, 1001})
	step("select 1009", sel(kalio, 1009), [2]int32{5, 1001})
}

func TestKerubThreatEventGuards(t *testing.T) {
	_, p, c, _, muranes, kalio := kerubFixture(t, "START", 0)
	script := &data.QuestScript{ID: 1001, Kind: data.QuestCustom, EndNPC: 203067}
	c.kerubThreatDialog(kalio, script, 10000)
	if p.quest(1001).Vars != 0 {
		t.Fatal("wrong NPC advanced quest")
	}
	c.kerubThreatDialog(muranes, script, 1012)
	if p.quest(1001).Vars != 0 {
		t.Fatal("the movie dialog advanced the quest")
	}
	if c.kerubThreatKill(210670) {
		t.Fatal("kill counted before the quest was accepted")
	}
}

func TestKerubThreatSelectableReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 4
	p.Exp = d.ExpStart(4)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: 1001, Status: "REWARD", Vars: 8}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1001, Kind: data.QuestCustom, EndNPC: 203067}
	end := questCatalogNPC(s, p, 203067, 0x31002)
	if !c.kerubThreatShowDialog(end, script) {
		t.Fatal("reward preview absent")
	}
	c.kerubThreatDialog(end, script, 1009)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(end.id, 5, 1001).Data) {
		t.Fatalf("reward dialog = %x", got)
	}
	beforeExp := p.Exp
	c.kerubThreatDialog(end, script, 8)
	if q := p.quest(1001); q.Status != "COMPLETE" || q.CompleteCount != 1 || p.Exp-beforeExp != 1000 || s.countItems(p, 114100806) != 1 {
		t.Fatalf("completion = %+v, experience=%d, item=%d", q, p.Exp-beforeExp, s.countItems(p, 114100806))
	}
	c.kerubThreatDialog(end, script, 8)
	if p.quest(1001).CompleteCount != 1 || s.countItems(p, 114100806) != 1 {
		t.Fatal("repeat finish granted reward")
	}
}
