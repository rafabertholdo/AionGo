package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func customQuestPortFixture(t *testing.T, questID int32, quests []store.Quest) (*data.Data, *Server, *player, *conn, *data.QuestScript, *questPackets) {
	t.Helper()
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[questID], d.Quests[questID]
	if script == nil || template == nil {
		t.Fatalf("custom quest %d is missing from the quest catalog", questID)
	}
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", template.MinLevel
	p.Exp = d.ExpStart(p.level)
	p.WorldID = 210010000
	p.stats = s.playerStats(p)
	p.appearance = &store.Appearance{}
	p.cube = []*store.Item{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.quests = append([]store.Quest(nil), quests...)
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	return d, s, p, c, script, packets
}
