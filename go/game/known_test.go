package game

import (
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// listener is a connection whose packets are counted by size instead of read.
func listener(t *testing.T, s *Server, id int32, race string, x float32) (*player, *atomic.Int32) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	frames := &atomic.Int32{}
	go func() {
		for {
			if _, err := wire.ReadFrame(client); err != nil {
				return
			}
			frames.Add(1)
		}
	}()
	crypt, _ := newGameCrypt(1)
	c := &conn{s: s, netConn: server, crypt: crypt, account: &account{}}
	ch := &character{Character: &store.Character{ID: id, Race: race, Class: "MAGE", WorldID: 210010000, X: x},
		appearance: &store.Appearance{}}
	p := &player{character: ch, stats: &gameStats{}, known: map[int32]*player{}, conn: c,
		settings: &store.Settings{}, abyss: &store.AbyssRank{}}
	c.player = p
	return p, frames
}

func waitFrames(t *testing.T, frames *atomic.Int32, want int32) {
	t.Helper()
	for range 100 {
		if frames.Load() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("got %d packets, want %d", frames.Load(), want)
}

func TestKnownLists(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	a, aFrames := listener(t, s, 1, "ELYOS", 100)
	b, bFrames := listener(t, s, 2, "ASMODIANS", 150)
	far, farFrames := listener(t, s, 3, "ELYOS", 1000)

	s.spawn(a)
	s.spawn(far)
	s.spawn(b)
	// b and a see each other (SM_PLAYER_INFO each); far sees nobody.
	waitFrames(t, aFrames, 1)
	waitFrames(t, bFrames, 1)
	waitFrames(t, farFrames, 0)
	if a.known[b.ID] == nil || b.known[a.ID] == nil || len(far.known) != 0 {
		t.Fatalf("known lists: a %v, b %v, far %v", a.known, b.known, far.known)
	}

	// b walks out of a's range: each is told to remove the other (SM_DELETE).
	s.visMu.Lock()
	s.updatePosition(b, 500, 0, 0, 0)
	s.visMu.Unlock()
	waitFrames(t, aFrames, 2)
	waitFrames(t, bFrames, 2)
	if len(a.known) != 0 || len(b.known) != 0 {
		t.Fatalf("still known: a %v, b %v", a.known, b.known)
	}

	// b comes back and then leaves: a sees it again, and is told it left.
	s.visMu.Lock()
	s.updatePosition(b, 120, 0, 0, 0)
	s.visMu.Unlock()
	waitFrames(t, aFrames, 3)
	s.despawn(b)
	waitFrames(t, aFrames, 4)
	if len(a.known) != 0 || len(s.spawned) != 2 {
		t.Fatalf("after leaving: a knows %v, %d spawned", a.known, len(s.spawned))
	}
}

func TestSpeechIsHeardByThoseWhoSee(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	a, aFrames := listener(t, s, 1, "ELYOS", 100)
	b, bFrames := listener(t, s, 2, "ELYOS", 150)
	far, farFrames := listener(t, s, 3, "ELYOS", 1000)
	s.spawn(a)
	s.spawn(b)
	s.spawn(far)
	waitFrames(t, aFrames, 1)
	waitFrames(t, bFrames, 1)

	w := wire.Packet(cmChatMessagePublic)
	w.C(chatNormal)
	w.S("hello")
	a.conn.chat(wire.NewReader(w.Data[1:]))
	waitFrames(t, aFrames, 2) // it hears itself
	waitFrames(t, bFrames, 2)
	waitFrames(t, farFrames, 0)
}

// testServer is a game server without its links, for tests.
func testServer(d *data.Data) *Server {
	return &Server{data: d, spawned: map[int32]*player{}, grid: map[cell][]*object{}, byID: map[int32]*object{},
		pcells: map[cell]map[int32]*player{}, players: map[int32]*conn{}, items: noItems{}, quests: noQuests{}, skillDB: noSkills{}, social: noSocial{}, mailDB: &memoryMail{}, legionDB: &memoryLegions{}, macroDB: noMacros{}, brokerDB: noBroker{}, ids: newIDFactory(nil), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// noItems keeps no items, for tests.
type noItems struct{}

func (noItems) InsertItem(*store.Item) error      { return nil }
func (noItems) UpdateItem(*store.Item) error      { return nil }
func (noItems) DeleteItem(int32) error            { return nil }
func (noItems) AddStone(int32, store.Stone) error { return nil }
func (noItems) DeleteStone(int32, int32) error    { return nil }
func (noItems) DeleteStones(int32) error          { return nil }

// noQuests keeps no quest state, for tests unrelated to quest persistence.
type noQuests struct{}

func (noQuests) SaveQuest(int32, store.Quest) error                            { return nil }
func (noQuests) CompleteQuest(int32, store.Quest, store.QuestCompletion) error { return nil }

// noSkills keeps no skills, for tests.
type noSkills struct{}

func (noSkills) SaveSkill(int32, store.Skill) error          { return nil }
func (noSkills) DeleteSkill(int32, int32) error              { return nil }
func (noSkills) AddRecipe(int32, int32) error                { return nil }
func (noSkills) DeleteRecipe(int32, int32) error             { return nil }
func (noSkills) SaveAbyssRank(int32, *store.AbyssRank) error { return nil }

type noSocial struct{}

func (noSocial) AddFriends(a, b int32) error               { return nil }
func (noSocial) DelFriends(a, b int32) error               { return nil }
func (noSocial) AddBlock(p, b int32, reason string) error  { return nil }
func (noSocial) DelBlock(p, b int32) error                 { return nil }
func (noSocial) SetBlockReason(p, b int32, r string) error { return nil }

type noMacros struct{}

func (noMacros) AddMacro(int32, int32, string) error { return nil }
func (noMacros) DeleteMacro(int32, int32) error      { return nil }

type noBroker struct{}

func (noBroker) InsertBroker(*store.BrokerRow) error { return nil }
func (noBroker) UpdateBroker(*store.BrokerRow) error { return nil }
func (noBroker) DeleteBroker(*store.BrokerRow) error { return nil }
