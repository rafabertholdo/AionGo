package game

import (
	"testing"

	"aionlightning/wire"
)

// TestSmallRequests covers titles, macros, looking at a player and searching for players.
func TestSmallRequests(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	ann.friendStatus, bob.friendStatus = friendStatusOnline, friendStatusOnline
	ann.conn.titleSet(packet(cmTitleSet, func(w *wire.Writer) { w.D(7) }))
	if ann.TitleID != 7 || annTap.count(smTitleSet) != 1 || bobTap.count(smTitleUpdate) != 1 {
		t.Errorf("title %d: %v %v", ann.TitleID, annTap.counts, bobTap.counts)
	}
	ann.conn.macroCreate(packet(cmMacroCreate, func(w *wire.Writer) { w.C(1); w.S("<macro/>") }))
	ann.conn.macroCreate(packet(cmMacroCreate, func(w *wire.Writer) { w.C(1); w.S("<other/>") }))
	if len(ann.macros) != 1 || annTap.count(smMacroResult) != 2 {
		t.Errorf("macros %v", ann.macros)
	}
	ann.conn.macroDelete(packet(cmMacroDelete, func(w *wire.Writer) { w.C(1) }))
	if len(ann.macros) != 0 {
		t.Errorf("the macro stayed")
	}
	ann.conn.viewPlayerDetails(packet(cmViewPlayerDetails, func(w *wire.Writer) { w.D(bob.ID) }))
	if annTap.count(smViewPlayerDetails) != 1 {
		t.Errorf("no details of Bob")
	}
	ann.conn.playerSearch(packet(cmPlayerSearch, func(w *wire.Writer) {
		w.S("bo")
		w.B(make([]byte, 44-(len("bo")*2+2)))
		w.D(0)
		w.D(0)
		w.C(0xFF)
		w.C(0xFF)
		w.C(0)
		w.C(0)
	}))
	if annTap.count(smPlayerSearch) != 1 {
		t.Errorf("no search results")
	}
}

// TestLookingForGroupSearch covers the toggle and the social search packet layout.
func TestLookingForGroupSearch(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, _, _ := twoPlayers(t, s)
	ann.friendStatus, bob.friendStatus = friendStatusOnline, friendStatusOnline
	packets := &questPackets{}
	ann.conn.tap = packets.tap
	search := func(lfg byte, wantCount uint16, wantStatus byte) {
		t.Helper()
		packets.frames = nil
		ann.conn.playerSearch(packet(cmPlayerSearch, func(w *wire.Writer) {
			w.S("bO") // name matching stays case insensitive
			w.B(make([]byte, 38))
			w.D(0)
			w.D(0)
			w.C(0xFF)
			w.C(0xFF)
			w.C(lfg)
			w.C(0)
		}))
		got := packets.last(smPlayerSearch)
		if len(got) != 3+int(wantCount)*64 {
			t.Fatalf("search packet size = %d, want %d: %x", len(got), 3+int(wantCount)*64, got)
		}
		r := wire.NewReader(got[1:])
		if count := r.H(); count != wantCount {
			t.Fatalf("search count = %d, want %d", count, wantCount)
		}
		if wantCount == 0 {
			return
		}
		world, x, y, z := r.D(), r.F(), r.F(), r.F()
		class, gender, level, status := r.C(), r.C(), r.C(), r.C()
		name := r.S()
		_, wantGender := raceGender(bob.Character)
		if r.Err != nil || world != bob.WorldID || x != bob.X || y != bob.Y || z != bob.Z ||
			class != byte(classIDs[bob.Class]) || gender != byte(wantGender) || level != byte(bob.level) ||
			status != wantStatus || name != bob.Name {
			t.Fatalf("unexpected search result: %x (read error: %v)", got, r.Err)
		}
		for _, b := range r.B(36) { // Bob's name occupies eight of the 44 bytes
			if b != 0 {
				t.Fatalf("nonzero search name padding: %x", got)
			}
		}
	}
	search(0, 1, 0)
	search(1, 0, 0)
	bob.conn.playerStatusInfo(packet(cmPlayerStatusInfo, func(w *wire.Writer) { w.C(9); w.D(2) }))
	if !bob.lookingForGroup || ann.lookingForGroup {
		t.Fatal("LFG toggle did not apply only to Bob")
	}
	search(1, 1, 2)
	search(0, 1, 2)
	bob.friendStatus = friendStatusOffline
	search(1, 0, 0)
	bob.friendStatus = friendStatusOnline
	bob.conn.playerStatusInfo(packet(cmPlayerStatusInfo, func(w *wire.Writer) { w.C(9); w.D(0) }))
	if bob.lookingForGroup {
		t.Fatal("LFG toggle stayed enabled")
	}
	search(1, 0, 0)
	search(0, 1, 0)
}

func TestLookingForGroupToggleRejectsTruncatedPackets(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	_, bob, _, _ := twoPlayers(t, s)
	for _, initial := range []bool{false, true} {
		bob.lookingForGroup = initial
		id := int32(2)
		if initial {
			id = 0
		}
		w := wire.Packet(cmPlayerStatusInfo)
		w.C(9)
		w.D(id)
		for size := 0; size < 5; size++ {
			bob.conn.playerStatusInfo(wire.NewReader(w.Data[1 : 1+size]))
			if bob.lookingForGroup != initial {
				t.Fatalf("truncated toggle (%d bytes) changed LFG from %v", size, initial)
			}
		}
	}
}
