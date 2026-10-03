package game

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

type recordedRollItems struct {
	noItems
	calls   int
	err     error
	updates []store.LootStack
	inserts []store.Item
}

func (db *recordedRollItems) ReceiveLoot(ctx context.Context, owner int32, updates []store.LootStack, inserts []store.Item) error {
	if _, ok := ctx.Deadline(); !ok {
		panic("receipt must have a deadline")
	}
	db.calls++
	db.updates = updates
	db.inserts = inserts
	return db.err
}

type rollFixture struct {
	s       *Server
	g       *group
	players []*player
	packets []*questPackets
	corpse  *object
	item    *dropItem
	db      *recordedRollItems
}

func newRollFixture() *rollFixture {
	d := &data.Data{Items: map[int32]*data.ItemTemplate{100: {ID: 100, NameID: 123, Quality: "RARE", MaxStack: 1}}}
	s := testServer(d)
	f := &rollFixture{s: s, g: &group{id: 500, rule: lootRoundRobin}, db: &recordedRollItems{}}
	s.items = f.db
	for i := range 3 {
		p := &player{character: &character{Character: &store.Character{ID: int32(10 + i), Name: fmt.Sprintf("Player%d", i), WorldID: 1}}, stats: &gameStats{}, life: store.LifeStats{}, state: stateActive}
		packets := &questPackets{}
		p.conn = &conn{s: s, player: p, tap: packets.tap}
		p.group = f.g
		s.spawned[p.ID] = p
		f.g.members = append(f.g.members, p)
		f.players = append(f.players, p)
		f.packets = append(f.packets, packets)
	}
	f.g.leader = f.players[0]
	f.item = &dropItem{index: 1, item: 100, count: 1}
	f.corpse = &object{id: 600, worldID: 1, dead: true, npc: &data.NpcTemplate{}, watchers: map[int32]*player{}}
	f.corpse.ai = &npcAI{s: s, o: f.corpse}
	f.corpse.loot = &lootState{items: []*dropItem{f.item}, allowed: map[int32]bool{10: true}, group: f.g, eligible: append([]*player(nil), f.players...)}
	s.byID[f.corpse.id] = f.corpse
	return f
}
func (f *rollFixture) start() {
	f.s.openLoot(f.players[0], f.corpse.id)
	f.s.takeLoot(f.players[0], f.corpse.id, 1)
}
func (f *rollFixture) answer(player int, score int32) {
	f.s.answerLootRoll(f.corpse, f.players[player], score)
}
func rollRequest(groupID, itemID, npcID int32, kind byte, choice int32) []byte {
	w := wire.Packet(cmGroupLoot)
	w.D(groupID)
	w.D(1)
	w.D(1)
	w.D(itemID)
	w.C(0)
	w.D(npcID)
	w.C(kind)
	w.D(choice)
	w.Q(0)
	return w.Data[1:]
}
func TestGroupRollPromptMatchesJava(t *testing.T) {
	f := newRollFixture()
	f.start()
	want := []byte{0xa5, 244, 1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 100, 0, 0, 0, 0, 88, 2, 0, 0, 2, 0, 0, 0, 0, 1, 0, 0, 0}
	for i, packets := range f.packets {
		if got := packets.last(smGroupLoot); !bytes.Equal(got, want) {
			t.Fatalf("player %d prompt = %x, want %x", i, got, want)
		}
	}
	if len(f.corpse.loot.allowed) != 1 || len(f.corpse.loot.active.pending) != 3 || f.db.calls != 0 {
		t.Fatal("opening rights must not limit roll eligibility or grant the item")
	}
	// Trying again cannot start a second roll or take a different drop during this roll.
	f.corpse.loot.items = append(f.corpse.loot.items, &dropItem{index: 2, item: 100, count: 1})
	f.s.takeLoot(f.players[0], f.corpse.id, 1)
	f.s.takeLoot(f.players[0], f.corpse.id, 2)
	if len(f.corpse.loot.items) != 2 || f.db.calls != 0 {
		t.Fatal("pending roll did not protect corpse")
	}
}
func TestGroupRollWinnerAndTie(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scores [3]int32
		winner int
	}{
		{"highest", [3]int32{20, 90, 30}, 1}, {"first response wins tie", [3]int32{80, 80, 0}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRollFixture()
			f.start()
			f.answer(0, tc.scores[0])
			f.answer(1, tc.scores[1])
			if f.db.calls != 0 {
				t.Fatal("awarded before everyone responded")
			}
			f.answer(2, tc.scores[2])
			if f.db.calls != 1 || len(f.players[tc.winner].cube) != 1 || f.corpse.loot != nil || len(f.s.lootRolls) != 0 {
				t.Fatal("winner grant or cleanup failed")
			}
			for i, p := range f.players {
				if i != tc.winner && len(p.cube) != 0 {
					t.Fatal("loser got the item")
				}
			}
			if !bytes.Equal(f.packets[tc.winner].last(smSystemMessage), systemMessage(1390180, descriptionID(123)).Data) {
				t.Fatal("winner message differs from Java")
			}
			f.players[0].conn.groupLoot(wire.NewReader(rollRequest(500, 100, 600, 2, 1)))
			if f.db.calls != 1 {
				t.Fatal("replayed response duplicated reward")
			}
		})
	}
}
func TestGroupRollAllPass(t *testing.T) {
	f := newRollFixture()
	f.start()
	for i := range 3 {
		f.answer(i, 0)
	}
	if !f.item.free || f.item.roll != nil || f.corpse.loot.active != nil || f.db.calls != 0 || len(f.s.lootRolls) != 0 {
		t.Fatal("all passes did not restore ordinary looting")
	}
	if !bytes.Equal(f.packets[2].last(smSystemMessage), systemMessage(1390164).Data) {
		t.Fatal("pass-self message differs from Java")
	}
	if !bytes.Equal(f.packets[0].last(smSystemMessage), systemMessage(1390165, "Player2").Data) {
		t.Fatal("pass-other message differs from Java")
	}
	f.s.takeLoot(f.players[0], 600, 1)
	if len(f.players[0].cube) != 1 || f.corpse.loot != nil {
		t.Fatal("all-pass item could not be collected")
	}
}
func TestGroupRollFullInventoryAndRetry(t *testing.T) {
	f := newRollFixture()
	f.start()
	winner := f.players[1]
	for i := range winner.cubeLimit() {
		winner.cube = append(winner.cube, &store.Item{UniqueID: int32(1000 + i), ItemID: 999, Count: 1})
	}
	f.answer(0, 0)
	f.answer(1, 100)
	f.answer(2, 0)
	if f.item.roll == nil || f.item.roll.winner != winner || f.corpse.loot.active != nil || f.db.calls != 0 {
		t.Fatal("full inventory lost winner reservation")
	}
	if !bytes.Equal(f.packets[1].last(smSystemMessage), systemMessage(msgInventoryFull).Data) {
		t.Fatal("winner did not get inventory-full message")
	}
	f.s.takeLoot(f.players[0], 600, 1)
	if !bytes.Equal(f.packets[0].last(smSystemMessage), systemMessage(1390220).Data) {
		t.Fatal("other looter bypassed reservation")
	}
	f.s.closeLoot(f.players[0], 600)
	winner.cube = winner.cube[:0]
	f.s.openLoot(winner, 600)
	f.s.takeLoot(winner, 600, 1)
	if f.db.calls != 1 || len(winner.cube) != 1 || f.corpse.loot != nil {
		t.Fatal("winner could not retry after making room")
	}
}
func TestGroupRollPersistenceFailureAndStackMerge(t *testing.T) {
	f := newRollFixture()
	f.s.data.Items[100].MaxStack = 10
	f.item.count = 4
	winner := f.players[1]
	stack := &store.Item{UniqueID: 700, ItemID: 100, Count: 8, Owner: winner.ID}
	winner.cube = []*store.Item{stack}
	f.db.err = errors.New("commit failed")
	f.start()
	f.answer(0, 0)
	f.answer(1, 100)
	f.answer(2, 0)
	if stack.Count != 8 || len(winner.cube) != 1 || f.corpse.loot == nil || f.item.roll.winner != winner {
		t.Fatal("failed receipt changed memory or removed the drop")
	}
	if len(f.db.updates) != 1 || f.db.updates[0].Count != 10 || len(f.db.inserts) != 1 || f.db.inserts[0].Count != 2 {
		t.Fatal("incorrect complete stack plan")
	}
	if f.packets[1].last(smAddItems) != nil || f.packets[1].last(smUpdateItem) != nil {
		t.Fatal("failed write announced inventory changes")
	}
	f.db.err = nil
	f.s.closeLoot(f.players[0], 600)
	f.s.openLoot(winner, 600)
	f.s.takeLoot(winner, 600, 1)
	if stack.Count != 10 || len(winner.cube) != 2 || winner.cube[1].Count != 2 || f.corpse.loot != nil {
		t.Fatal("retry did not grant exact stack count")
	}
}
func TestGroupRollRejectsInvalidResponses(t *testing.T) {
	for _, tc := range []struct {
		name             string
		group, item, npc int32
		kind             byte
		choice           int32
	}{
		{"wrong group", 501, 100, 600, 2, 1}, {"wrong item", 500, 101, 600, 2, 1}, {"wrong corpse", 500, 100, 601, 2, 1},
		{"bid", 500, 100, 600, 3, 1}, {"invalid choice", 500, 100, 600, 2, 2}, {"negative choice", 500, 100, 600, 2, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRollFixture()
			f.start()
			before := len(f.packets[0].frames)
			f.players[0].conn.groupLoot(wire.NewReader(rollRequest(tc.group, tc.item, tc.npc, tc.kind, tc.choice)))
			if len(f.corpse.loot.active.pending) != 3 || len(f.packets[0].frames) != before {
				t.Fatal("invalid response changed roll")
			}
		})
	}
	for size := range 34 {
		t.Run(fmt.Sprintf("truncated_%d", size), func(t *testing.T) {
			f := newRollFixture()
			f.start()
			payload := rollRequest(500, 100, 600, 2, 1)
			f.players[0].conn.groupLoot(wire.NewReader(payload[:size]))
			if len(f.corpse.loot.active.pending) != 3 {
				t.Fatal("truncated response changed roll")
			}
		})
	}
	f := newRollFixture()
	f.start()
	outsider := &player{character: &character{Character: &store.Character{ID: 99}}, group: f.g}
	outsider.conn = &conn{s: f.s, player: outsider, tap: func(*wire.Writer) { t.Fatal("outsider got a roll message") }}
	outsider.conn.groupLoot(wire.NewReader(rollRequest(500, 100, 600, 2, 1)))
	if len(f.corpse.loot.active.pending) != 3 {
		t.Fatal("outsider responded")
	}
	f.players[0].conn.groupLoot(wire.NewReader(rollRequest(500, 100, 600, 2, 1)))
	frames := len(f.packets[0].frames)
	f.players[0].conn.groupLoot(wire.NewReader(rollRequest(500, 100, 600, 2, 1)))
	if len(f.corpse.loot.active.pending) != 2 || len(f.packets[0].frames) != frames {
		t.Fatal("duplicate response changed score")
	}
	// The score announced to the rolling player is generated in the inclusive 1..100 range.
	msg := f.packets[0].last(smSystemMessage)
	if int32(binary.LittleEndian.Uint32(msg[7:11])) != msgRollMe {
		t.Fatalf("roll-self packet %x", msg)
	}
	value, err := strconv.Atoi(wire.NewReader(msg[12:]).S())
	if err != nil || value < 1 || value > 100 {
		t.Fatalf("invalid server-generated roll: %d, %v", value, err)
	}
}
func TestGroupRollDepartureAndDespawn(t *testing.T) {
	t.Run("pending member leaves", func(t *testing.T) {
		f := newRollFixture()
		f.start()
		f.answer(0, 90)
		f.answer(1, 10)
		f.s.leaveGroup(f.players[2])
		if len(f.players[0].cube) != 1 || len(f.s.lootRolls) != 0 {
			t.Fatal("departure stalled roll")
		}
	})
	t.Run("uncollected winner leaves", func(t *testing.T) {
		f := newRollFixture()
		f.db.err = errors.New("write failed")
		f.start()
		f.answer(0, 0)
		f.answer(1, 100)
		f.answer(2, 0)
		f.players[1].group = nil
		f.s.leaveLootRolls(f.players[1])
		if !f.item.free || f.item.roll != nil || len(f.s.lootRolls) != 0 {
			t.Fatal("departed winner retained reservation")
		}
	})
	t.Run("corpse despawns", func(t *testing.T) {
		f := newRollFixture()
		f.start()
		f.s.despawnNpc(f.corpse, true)
		if f.corpse.loot != nil || len(f.s.lootRolls) != 0 {
			t.Fatal("despawn retained roll")
		}
		f.players[0].conn.groupLoot(wire.NewReader(rollRequest(500, 100, 600, 2, 1)))
		if f.db.calls != 0 {
			t.Fatal("despawned corpse accepted roll")
		}
	})
}
func TestGroupRollQualityAndEligibility(t *testing.T) {
	for _, quality := range []string{"COMMON", "RARE", "LEGEND", "UNIQUE", "EPIC", "MYTHIC", "JUNK"} {
		t.Run(quality, func(t *testing.T) {
			f := newRollFixture()
			f.s.data.Items[100].Quality = quality
			f.start()
			shouldRoll := quality != "COMMON" && quality != "JUNK"
			if (f.corpse.loot != nil && f.corpse.loot.active != nil) != shouldRoll {
				t.Fatal("incorrect quality distribution")
			}
		})
	}
	t.Run("one eligible member", func(t *testing.T) {
		f := newRollFixture()
		f.corpse.loot.eligible = f.players[:1]
		f.start()
		if len(f.players[0].cube) != 1 {
			t.Fatal("single member should loot without a roll")
		}
	})
	t.Run("offline member", func(t *testing.T) {
		f := newRollFixture()
		delete(f.s.spawned, f.players[2].ID)
		f.start()
		if len(f.corpse.loot.active.pending) != 2 || f.packets[2].last(smGroupLoot) != nil {
			t.Fatal("offline member entered roll")
		}
	})
}
func TestGroupRollDistributionSettings(t *testing.T) {
	f := newRollFixture()
	qualities := [7]int32{2, 0, 2, 0, 2, 0, 0}
	request := wire.Packet(cmDistributionSettings)
	request.D(2)
	request.D(2)
	for _, rule := range qualities {
		request.D(rule)
	}
	for size := range 36 {
		f.players[0].conn.distributionSettings(wire.NewReader(request.Data[1 : 1+size]))
		if f.g.rule != lootRoundRobin || f.g.qualityRules != nil {
			t.Fatal("truncated settings changed group")
		}
	}
	f.players[1].conn.distributionSettings(wire.NewReader(request.Data[1:]))
	if f.g.qualityRules != nil {
		t.Fatal("member changed leader's settings")
	}
	f.players[0].conn.distributionSettings(wire.NewReader(request.Data[1:]))
	if f.g.rule != lootLeader || f.g.distribution != 2 || !reflect.DeepEqual(f.g.lootQualityRules(), qualities) {
		t.Fatal("settings not retained")
	}
	got := f.packets[1].last(smGroupInfo)
	if int32(binary.LittleEndian.Uint32(got[13:17])) != 2 || int32(binary.LittleEndian.Uint32(got[17:21])) != 2 {
		t.Fatalf("settings not reflected in group info: %x", got)
	}
	if handlers[cmGroupLoot] == nil || clientPacketStates[cmGroupLoot] != inGame {
		t.Fatal("group roll handler must require an in-game client")
	}
}

func TestGroupRollConcurrentResponses(t *testing.T) {
	f := newRollFixture()
	f.start()
	var wg sync.WaitGroup
	for _, p := range f.players {
		wg.Go(func() {
			for range 10 {
				p.conn.groupLoot(wire.NewReader(rollRequest(500, 100, 600, 2, 1)))
			}
		})
	}
	wg.Wait()
	count := 0
	for _, p := range f.players {
		count += len(p.cube)
	}
	if f.db.calls != 1 || count != 1 || f.corpse.loot != nil || len(f.s.lootRolls) != 0 {
		t.Fatal("concurrent/repeated responses did not grant exactly one item")
	}
}

func TestGroupRollStackFitsFullCube(t *testing.T) {
	f := newRollFixture()
	f.s.data.Items[100].MaxStack = 10
	winner := f.players[1]
	winner.cube = []*store.Item{{UniqueID: 700, ItemID: 100, Owner: winner.ID, Count: 9}}
	for i := range winner.cubeLimit() - 1 {
		winner.cube = append(winner.cube, &store.Item{UniqueID: int32(1000 + i), ItemID: 999, Count: 1})
	}
	f.start()
	f.answer(0, 0)
	f.answer(1, 100)
	f.answer(2, 0)
	if f.db.calls != 1 || len(f.db.inserts) != 0 || winner.cube[0].Count != 10 || f.corpse.loot != nil {
		t.Fatal("full cube could not receive loot in an existing stack")
	}
}

func TestGroupRollClosingByNonLooter(t *testing.T) {
	f := newRollFixture()
	f.start()
	f.s.closeLoot(f.players[1], 600)
	if f.corpse.loot.looting != 10 || f.corpse.loot.allowed == nil {
		t.Fatal("non-looter changed corpse rights")
	}
	f.s.closeLoot(f.players[0], 600)
	// Closing/reopening the corpse cannot bypass a pending roll.
	f.s.openLoot(f.players[1], 600)
	f.s.takeLoot(f.players[1], 600, 1)
	if f.db.calls != 0 || len(f.corpse.loot.active.pending) != 3 {
		t.Fatal("reopening bypassed pending roll")
	}
	f.answer(0, 10)
	f.answer(1, 90)
	f.answer(2, 30)
	if len(f.players[1].cube) != 1 || f.corpse.loot != nil {
		t.Fatal("roll did not complete after changing looter")
	}
}
