package game

import (
	"slices"
	"testing"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// memoryLegions keeps legions in memory, for tests.
type memoryLegions struct {
	legions map[int32]*store.Legion
	members map[int32]store.LegionMemberRow
	notices map[int32][]store.Announcement
	history map[int32][]store.HistoryEntry
}

func (m *memoryLegions) init() {
	if m.legions == nil {
		m.legions, m.members, m.notices = map[int32]*store.Legion{}, map[int32]store.LegionMemberRow{}, map[int32][]store.Announcement{}
	}
}
func (m *memoryLegions) LegionByID(id int32) (*store.Legion, error) {
	m.init()
	return m.legions[id], nil
}
func (m *memoryLegions) LegionNameUsed(name string) (bool, error) {
	m.init()
	for _, l := range m.legions {
		if l.Name == name {
			return true, nil
		}
	}
	return false, nil
}
func (m *memoryLegions) InsertLegion(id int32, name string) error {
	m.init()
	m.legions[id] = &store.Legion{ID: id, Name: name}
	return nil
}
func (m *memoryLegions) UpdateLegion(l *store.Legion) error {
	m.init()
	m.legions[l.ID] = l
	return nil
}
func (m *memoryLegions) DeleteLegion(id int32) error { m.init(); delete(m.legions, id); return nil }
func (m *memoryLegions) LegionMembers(id int32) ([]store.LegionMemberRow, error) {
	m.init()
	var rows []store.LegionMemberRow
	for _, r := range m.members {
		if r.LegionID == id {
			rows = append(rows, r)
		}
	}
	return rows, nil
}
func (m *memoryLegions) LegionMemberOf(id int32) (*store.LegionMemberRow, error) {
	m.init()
	if r, ok := m.members[id]; ok {
		return &r, nil
	}
	return nil, nil
}
func (m *memoryLegions) InsertLegionMember(legion, player int32, rank string) error {
	m.init()
	m.members[player] = store.LegionMemberRow{PlayerID: player, LegionID: legion, Rank: rank}
	return nil
}
func (m *memoryLegions) UpdateLegionMember(player int32, nickname, rank, intro string) error {
	m.init()
	r := m.members[player]
	r.Nickname, r.Rank, r.SelfIntro = nickname, rank, intro
	m.members[player] = r
	return nil
}
func (m *memoryLegions) DeleteLegionMember(player int32) error {
	m.init()
	delete(m.members, player)
	return nil
}
func (m *memoryLegions) LegionHistory(id int32) ([]store.HistoryEntry, error) {
	return slices.Clone(m.history[id]), nil
}
func (m *memoryLegions) InsertHistory(id int32, h store.HistoryEntry) error {
	if m.history == nil {
		m.history = map[int32][]store.HistoryEntry{}
	}
	m.history[id] = append(m.history[id], h)
	return nil
}
func (m *memoryLegions) LegionAnnouncements(id int32) ([]store.Announcement, error) {
	m.init()
	return m.notices[id], nil
}
func (m *memoryLegions) InsertAnnouncement(id int32, text string, at time.Time) error {
	m.init()
	m.notices[id] = append(m.notices[id], store.Announcement{Text: text, At: at})
	return nil
}

// TestLegion has Ann found a legion, Bob join it, talk to it, and be kicked.
func TestLegion(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	s.visMu.Lock()
	ann.kinah.Count = 20000
	s.visMu.Unlock()
	create := func(name string) {
		ann.conn.legionRequest(packet(cmLegion, func(w *wire.Writer) { w.C(0); w.D(0); w.S(name) }))
	}
	create("x")
	if ann.legion != nil {
		t.Fatalf("a legion with a bad name was made")
	}
	create("Angels")
	if ann.legion == nil || ann.member.rank != rankBrigadeGeneral || ann.kinah.Count != 10000 || annTap.count(smLegionInfo) != 1 {
		t.Fatalf("no legion: %v", annTap.counts)
	}
	ann.conn.legionRequest(packet(cmLegion, func(w *wire.Writer) { w.C(1); w.D(0); w.S("Bob") }))
	if bobTap.count(smQuestionWindow) != 1 {
		t.Fatalf("Bob wasn't asked")
	}
	bob.conn.questionResponse(packet(cmQuestionResponse, func(w *wire.Writer) { w.D(questionLegionInvite); w.C(1) }))
	if bob.legion != ann.legion || len(ann.legion.members) != 2 || bob.member.rank != rankLegionary {
		t.Fatalf("Bob didn't join")
	}
	ann.conn.chat(packet(cmChatMessagePublic, func(w *wire.Writer) { w.C(chatLegion); w.S("hello legion") }))
	if bobTap.count(smMessage) != 1 {
		t.Errorf("Bob didn't hear the legion")
	}
	ann.conn.legionRequest(packet(cmLegion, func(w *wire.Writer) { w.C(9); w.D(0); w.S("Meet at noon") }))
	if len(ann.legion.announcements) != 1 {
		t.Errorf("no announcement")
	}
	emblem := func() {
		ann.conn.legionModifyEmblem(packet(cmLegionModifyEmblem, func(w *wire.Writer) {
			w.D(ann.legion.ID)
			w.H(7)
			w.C(0xFF)
			w.C(1)
			w.C(2)
			w.C(3)
		}))
	}
	emblem() // level 1
	if ann.legion.EmblemID != 0 {
		t.Errorf("a level 1 legion got an emblem")
	}
	ann.legion.Level = 2
	ann.kinah.Count = 0
	emblem()
	if ann.legion.EmblemID != 0 || annTap.count(smSystemMessage) == 0 {
		t.Errorf("an emblem was bought without kinah")
	}
	ann.kinah.Count = 10000
	emblem()
	if l := ann.legion; l.EmblemID != 7 || l.EmblemB != 3 || ann.kinah.Count != 0 || bobTap.count(smLegionUpdateEmblem) != 1 {
		t.Errorf("no emblem: %+v %d", l.Legion, ann.kinah.Count)
	}
	bob.conn.legionSendEmblem(packet(cmLegionSendEmblem, func(w *wire.Writer) { w.D(ann.legion.ID) }))
	if bobTap.count(smLegionSendEmblem) != 1 {
		t.Errorf("Bob wasn't sent the emblem")
	}
	ann.conn.legionRequest(packet(cmLegion, func(w *wire.Writer) { w.C(4); w.D(0); w.S("Bob") }))
	if bob.legion != nil || len(ann.legion.members) != 1 {
		t.Errorf("Bob wasn't kicked")
	}
	// CREATE, two JOINs, the emblem and the KICK, newest first.
	if h := ann.legion.history; len(h) != 5 || h[0].Type != "KICK" || h[0].Name != "Bob" || h[4].Type != "CREATE" {
		t.Errorf("history %+v", h)
	}
	if annTap.count(smLegionTabs) != 4 {
		t.Errorf("the history was sent %d times", annTap.count(smLegionTabs))
	}
	ann.conn.legionTabs(packet(cmLegionTabs, func(w *wire.Writer) { w.D(0); w.C(0) }))
	if annTap.count(smLegionTabs) != 5 {
		t.Errorf("the history wasn't asked for")
	}
}
