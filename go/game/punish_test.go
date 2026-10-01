package game

import (
	"slices"
	"testing"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// memoryPunish keeps prison sentences and petitions, for tests.
type memoryPunish struct {
	timers    map[int32]int64
	petitions []*store.Petition
	replied   []int32
}

func (m *memoryPunish) PrisonTimer(id int32) (int64, error) { return m.timers[id], nil }
func (m *memoryPunish) PunishPlayer(id int32, _ int, ms int64) error {
	m.timers[id] = ms
	return nil
}
func (m *memoryPunish) UnpunishPlayer(id int32) error { delete(m.timers, id); return nil }
func (m *memoryPunish) SavePunishment(id int32, _ int, ms int64) error {
	if _, ok := m.timers[id]; ok {
		m.timers[id] = ms
	}
	return nil
}
func (m *memoryPunish) AccountIDByName(string) (int32, error) { return 0, nil }
func (m *memoryPunish) Petitions() ([]*store.Petition, error) { return m.petitions, nil }
func (m *memoryPunish) NextPetitionID() (int32, error)        { return int32(len(m.petitions)) + 1, nil }
func (m *memoryPunish) InsertPetition(p *store.Petition) error {
	m.petitions = append(m.petitions, p)
	return nil
}
func (m *memoryPunish) SetPetitionReplied(id int32) error {
	m.replied = append(m.replied, id)
	return nil
}
func (m *memoryPunish) PlayerName(int32) (string, error) { return "", nil }
func (m *memoryPunish) PetitionByID(id int32) (*store.Petition, error) {
	for _, p := range m.petitions {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}
func (m *memoryPunish) DeletePetition(playerID int32) error {
	for i, p := range m.petitions {
		if p.PlayerID == playerID && !slices.Contains(m.replied, p.ID) {
			m.petitions = append(m.petitions[:i], m.petitions[i+1:]...)
			break
		}
	}
	return nil
}

func punishServer(t *testing.T) (*Server, *memoryPunish, *player, *player, *tapped, *tapped) {
	s := testServer(staticDataOrSkip(t))
	m := &memoryPunish{timers: map[int32]int64{}}
	s.punishDB, s.petitionDB = m, m
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	ann.conn.account.accessLevel = 3
	return s, m, ann, bob, annTap, bobTap
}

func say(p *player, text string) {
	p.conn.chat(packet(cmChatMessagePublic, func(w *wire.Writer) { w.C(chatNormal); w.S(text) }))
}

// TestGag has a game master gag Bob, whose words then reach no one until it ungags him.
func TestGag(t *testing.T) {
	_, _, ann, bob, annTap, bobTap := punishServer(t)
	say(ann, "//gag Bob 1")
	if !bob.punish.gagged || bobTap.count(smMessage) != 1 {
		t.Fatalf("Bob isn't gagged")
	}
	say(bob, "hello")
	if annTap.count(smMessage) != 1 { // the gag confirmation and nothing more
		t.Errorf("a gagged player was heard: %d", annTap.count(smMessage))
	}
	say(ann, "//ungag Bob")
	say(bob, "hello")
	if bob.punish.gagged || annTap.count(smMessage) != 3 {
		t.Errorf("Bob wasn't ungagged: %d", annTap.count(smMessage))
	}
	say(ann, "//gag Nobody")
	say(ann, "//gag Bob x")
	if bob.punish.gagged {
		t.Errorf("a bad command gagged Bob")
	}
}

// TestPrison sends Bob to prison for ten minutes: he is moved, can't chat, and serves his time while online only.
func TestPrison(t *testing.T) {
	s, m, ann, bob, _, bobTap := punishServer(t)
	say(ann, "//sprison Bob 10")
	if bob.WorldID != prisonMap || bob.punish.prisonTimer != 10*time.Minute || m.timers[bob.ID] != 600000 {
		t.Fatalf("Bob is in %d with %v left, saved %d", bob.WorldID, bob.punish.prisonTimer, m.timers[bob.ID])
	}
	before := bobTap.count(smMessage)
	say(bob, "let me out")
	if !bob.inPrison() || bobTap.count(smMessage) != before+1 {
		t.Errorf("chat in prison wasn't refused")
	}
	if !s.restrictedInPrison(bob, "attack") {
		t.Errorf("Bob can attack")
	}
	// He logs out four minutes in: six are left, and the clock is stopped.
	bob.punish.prisonStart = time.Now().Add(-4 * time.Minute)
	s.visMu.Lock()
	s.prisonLogout(bob)
	s.visMu.Unlock()
	if left := bob.punish.prisonTimer; left < 6*time.Minute-time.Second || left > 6*time.Minute {
		t.Errorf("%v left", left)
	}
	if s.saveSentence(bob); m.timers[bob.ID] < 359000 || m.timers[bob.ID] > 360000 {
		t.Errorf("saved %d", m.timers[bob.ID])
	}
	// Back in the world, the sentence goes on; a game master frees him.
	s.visMu.Lock()
	s.prisonLogin(bob)
	s.visMu.Unlock()
	if bob.punish.prisonTask == nil {
		t.Errorf("the sentence isn't running")
	}
	s.players[bob.ID] = bob.conn // the map is loading, so Bob isn't spawned
	s.visMu.Lock()
	s.spawnLocked(bob)
	s.visMu.Unlock()
	say(ann, "//rprison Bob")
	if bob.inPrison() || bob.WorldID == prisonMap || len(m.timers) != 0 {
		t.Errorf("Bob is still in prison: %d, %v, %v", bob.WorldID, bob.punish.prisonTimer, m.timers)
	}
}

// TestLoadPunishment puts a prisoner in the prison map as he loads.
func TestLoadPunishment(t *testing.T) {
	s, m, _, bob, _, _ := punishServer(t)
	m.timers[bob.ID] = 90000
	if err := s.loadPunishment(bob); err != nil || bob.WorldID != prisonMap || bob.punish.prisonTimer != 90*time.Second {
		t.Errorf("Bob is in %d with %v left (%v)", bob.WorldID, bob.punish.prisonTimer, err)
	}
}

// TestPetitions has Bob file a petition, the game master read and answer it, and Bob file another and cancel it.
func TestPetitions(t *testing.T) {
	s, m, ann, bob, annTap, bobTap := punishServer(t)
	file := func(action uint16, text string) {
		bob.conn.petition(packet(cmPetition, func(w *wire.Writer) { w.H(action); w.S(text) }))
	}
	file(petitionBug, "Broken/It is broken/12:00/Poeta 1,1")
	if len(m.petitions) != 1 || bobTap.count(smPetition) != 1 || annTap.count(smMessage) != 1 {
		t.Fatalf("%d petitions, %d packets, %d notices", len(m.petitions), bobTap.count(smPetition), annTap.count(smMessage))
	}
	file(petitionBug, "Again/again/x/y")
	if len(m.petitions) != 1 {
		t.Errorf("a second petition was accepted")
	}
	say(ann, "//petition")
	say(ann, "//petition 1")
	if annTap.count(smMessage) != 1+1+1+1+1 {
		t.Errorf("%d messages", annTap.count(smMessage))
	}
	if got := petitionExtra(m.petitions[0]); got != "Time Occured: 12:00\nZone and Coords: Poeta 1,1" {
		t.Errorf("extra %q", got)
	}
	ann.Race = bob.Race
	ann.kinah.Count = 1000
	bob.Name = "Bob"
	m2 := &memoryMail{characters: map[string]*store.Character{"Bob": bob.Character}}
	s.mailDB = m2
	say(ann, "//petition 1 reply Try again")
	if len(m2.letters) != 1 || len(m.replied) != 1 || len(s.petitions) != 0 || bobTap.count(smPetition) != 2 {
		t.Errorf("%d letters, %v replied, %d open, %d packets", len(m2.letters), m.replied, len(s.petitions), bobTap.count(smPetition))
	}
	file(petitionQuest, "Quest/text/The quest")
	file(petitionCancel, "")
	bob.conn.petition(packet(cmPetition, func(w *wire.Writer) { w.H(petitionCancel); w.D(0) }))
	if len(s.petitions) != 0 || len(m.petitions) != 1 {
		t.Errorf("the petition wasn't cancelled")
	}
	say(ann, "//petition 99")
}
