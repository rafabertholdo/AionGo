package game

import (
	"testing"

	"aionlightning/wire"
)

// TestAdminCommands has a game master give itself kinah and an item, and a plain player fail to.
func TestAdminCommands(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	ann, bob, _, _ := twoPlayers(t, s)
	say := func(p *player, text string) {
		p.conn.chat(packet(cmChatMessagePublic, func(w *wire.Writer) { w.C(chatNormal); w.S(text) }))
	}
	say(ann, "//kinah 100")
	if ann.kinah.Count != 0 {
		t.Fatalf("a player got kinah")
	}
	ann.conn.account.accessLevel = 3
	say(ann, "//kinah 100")
	say(ann, "//kinah Bob 50")
	say(ann, "//add 160010002 5")
	if ann.kinah.Count != 100 || bob.kinah.Count != 50 || len(ann.cube) != 1 || ann.cube[0].Count != 5 {
		t.Errorf("kinah %d and %d, %d items", ann.kinah.Count, bob.kinah.Count, len(ann.cube))
	}
	say(ann, "//set level 5")
	if ann.level != 5 {
		t.Errorf("level %d", ann.level)
	}
}
