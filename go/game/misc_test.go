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
