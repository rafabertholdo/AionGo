package game

import (
	"slices"
	"testing"

	"aionlightning/game/store"
)

// The dialog framework of NpcController.onDialogRequest / onDialogSelect, on the report quest 1101 (Elyos, NPCs
// 203049 -> 203057), a template whose Java handler has no dialog -1 except the reward window.
func TestQuestDialogFrameworkMainMenuAndFallback(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.seen = map[int32]*object{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: 182400001, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	start := questCatalogNPC(s, p, 203049, 0x30001)
	end := questCatalogNPC(s, p, 203057, 0x30002)
	windows := func(do func()) [][2]int32 {
		from := len(packets.frames)
		do()
		return packets.windows(from)
	}
	show := func(o *object) [][2]int32 {
		return windows(func() { c.showDialog(dialogRequest(cmShowDialog, o.id, 0, 0)) })
	}
	sel := func(o *object, dialog uint16, quest int32) [][2]int32 {
		return windows(func() { c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialog, quest)) })
	}
	menu := [][2]int32{{10, 0}}

	if got := show(start); !slices.Equal(got, menu) || packets.last(smLookatobject) == nil {
		t.Fatalf("click on the giver = %v, want the main menu after a look-at", got)
	}
	if got := sel(start, 25, 1101); !slices.Equal(got, [][2]int32{{1011, 1101}}) {
		t.Fatalf("select 25 = %v", got)
	}
	// Not handled by the quest: answered with the window of the same number, with or without a quest id.
	if got := sel(start, 1013, 1101); !slices.Equal(got, [][2]int32{{1013, 1101}}) {
		t.Fatalf("unhandled select with a quest = %v", got)
	}
	if got := sel(start, 1013, 0); !slices.Equal(got, [][2]int32{{1013, 0}}) {
		t.Fatalf("unhandled select without a quest = %v", got)
	}
	// The ids NpcController answers itself are not echoed.
	for _, dialog := range []uint16{2, 4, 20, 61} {
		if got := sel(start, dialog, 0); len(got) != 0 {
			t.Fatalf("service dialog %d was echoed: %v", dialog, got)
		}
	}
	// A quest id the client names is the handler that answers, whatever npc is talked to.
	if got := sel(start, 1002, 1101); !slices.Equal(got, [][2]int32{{1003, 1101}}) {
		t.Fatalf("accept = %v", got)
	}
	// In START the end npc's click is the main menu too (Java's ReportTo has no -1), then 25 opens the turn-in page.
	if got := show(end); !slices.Equal(got, menu) {
		t.Fatalf("click on the end npc in START = %v", got)
	}
	if got := sel(end, 25, 1101); !slices.Equal(got, [][2]int32{{2375, 1101}}) {
		t.Fatalf("turn-in page = %v", got)
	}
	if got := sel(end, 1009, 1101); !slices.Equal(got, [][2]int32{{5, 1101}}) {
		t.Fatalf("ready for reward = %v", got)
	}
	// In REWARD the click is defaultQuestEndDialog(-1): the reward window.
	if got := show(end); !slices.Equal(got, [][2]int32{{5, 1101}}) {
		t.Fatalf("click on the end npc in REWARD = %v", got)
	}
}

// A Java handler that plays a movie for a select and then returns false leaves the framework to answer with the
// window of the same number: the client sees the movie and then that page (recorded for quest 1001).
func TestQuestMoviePagesAreEchoedAfterTheMovie(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		quest, npc, vars, dialog int32
		race                     string
	}{
		{1002, 730007, 1, 1353, "ELYOS"},
		{1004, 203082, 0, 1013, "ELYOS"},
		{2001, 203518, 0, 1012, "ASMODIANS"},
		{2002, 203534, 1, 1353, "ASMODIANS"},
		{2003, 203539, 0, 1012, "ASMODIANS"},
		{2005, 203540, 0, 1012, "ASMODIANS"},
	} {
		s := testServer(d)
		p := wrathchild(s)
		p.Race = tc.race
		p.level = 10
		p.seen = map[int32]*object{}
		p.quests = []store.Quest{{ID: tc.quest, Status: "START", Vars: tc.vars}}
		packets := &questPackets{}
		c := &conn{s: s, player: p, tap: packets.tap}
		p.conn = c
		s.spawned[p.ID] = p
		npc := questCatalogNPC(s, p, tc.npc, 0x30001)
		c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, uint16(tc.dialog), tc.quest))
		if got := packets.windows(0); !slices.Equal(got, [][2]int32{{tc.dialog, tc.quest}}) || packets.last(smPlayMovie) == nil {
			t.Errorf("quest %d select %d: windows %v, movie %v", tc.quest, tc.dialog, got, packets.last(smPlayMovie) != nil)
		}
	}
}
