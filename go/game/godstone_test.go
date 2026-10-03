package game

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

type recordedGodstone struct {
	noItems
	calls                int
	err                  error
	weapon, stone, kinah store.Item
	price                int64
}

func (db *recordedGodstone) SocketGodstone(ctx context.Context, weapon, stone, kinah store.Item, price int64) error {
	if _, ok := ctx.Deadline(); !ok {
		panic("socket persistence must have a deadline")
	}
	db.calls++
	db.weapon, db.stone, db.kinah, db.price = weapon, stone, kinah, price
	return db.err
}

func godstoneFixture() (*Server, *player, *object, *recordedGodstone, *questPackets) {
	d := &data.Data{Items: map[int32]*data.ItemTemplate{
		100:        {ID: 100, EquipmentType: "WEAPON", Slot: 1, NameID: 123},
		168000002:  {ID: 168000002, Godstone: &data.Godstone{SkillID: 8255}},
		data.Kinah: {ID: data.Kinah},
	}}
	s := testServer(d)
	p := &player{character: &character{Character: &store.Character{ID: 99, WorldID: 210010000}}, stones: map[int32][]store.Stone{}}
	p.kinah = &store.Item{UniqueID: 3, ItemID: data.Kinah, Owner: p.ID, Count: 250000}
	p.cube = []*store.Item{{UniqueID: 1, ItemID: 100, Owner: p.ID, Count: 1}, {UniqueID: 2, ItemID: 168000002, Owner: p.ID, Count: 2}}
	packets := &questPackets{}
	p.conn = &conn{s: s, player: p, tap: packets.tap}
	o := &object{id: 4, worldID: p.WorldID, npc: &data.NpcTemplate{}}
	s.byID[o.id] = o
	db := &recordedGodstone{}
	s.items = db
	return s, p, o, db, packets
}

func godstoneRequest(p *player, npc int32) {
	w := wire.Packet(cmGodstoneSocket)
	w.D(npc)
	w.D(1)
	w.D(2)
	handlers[cmGodstoneSocket](p.conn, wire.NewReader(w.Data[1:]))
}

func TestGodstoneSocket(t *testing.T) {
	for _, count := range []int64{1, 2} {
		t.Run(fmt.Sprintf("stack_%d", count), func(t *testing.T) {
			s, p, o, db, packets := godstoneFixture()
			weapon, stone := p.cube[0], p.cube[1]
			stone.Count = count
			weapon.Godstone = 168000003
			p.stones[weapon.UniqueID] = []store.Stone{{ItemID: 167000001, Slot: 0}}
			godstoneRequest(p, o.id)
			if db.calls != 1 || db.price != 100000 || db.stone.Count != count || db.kinah.Count != 250000 || db.weapon.Godstone != 168000003 {
				t.Fatalf("incorrect persistence request: %+v", db)
			}
			if weapon.Godstone != stone.ItemID || stone.Count != count-1 || p.kinah.Count != 150000 || len(p.stones[weapon.UniqueID]) != 1 {
				t.Fatal("socket did not replace godstone, preserve manastone and consume resources")
			}
			if (p.cubeItem(2) == nil) != (count == 1) {
				t.Fatal("incorrect stone removal")
			}
			want := [][]byte{s.updateItemPacket(p, p.kinah).Data, systemMessage(1300508, descriptionID(123)).Data}
			if count == 1 {
				want = append(want, deleteItemPacket(2).Data)
			}
			want = append(want, s.updateItemPacket(p, stone).Data, s.updateItemPacket(p, weapon).Data)
			if !reflect.DeepEqual(packets.frames, want) {
				t.Fatalf("packet order or bytes mismatch: %x", packets.frames)
			}
			if got := s.updateItemPacket(p, weapon).Data; len(got) < 59 || !bytes.Equal(got[55:59], []byte{2, 122, 3, 10}) {
				t.Fatalf("godstone must occupy Java's weapon item field: %x", got)
			}
		})
	}
}

func TestGodstoneSocketRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Server, *player, *object, *recordedGodstone)
	}{
		{"missing_npc", func(s *Server, p *player, o *object, db *recordedGodstone) { delete(s.byID, o.id) }},
		{"non_npc", func(s *Server, p *player, o *object, db *recordedGodstone) { o.npc = nil }},
		{"different_map", func(s *Server, p *player, o *object, db *recordedGodstone) { o.worldID++ }},
		{"different_instance", func(s *Server, p *player, o *object, db *recordedGodstone) { o.instance++ }},
		{"range_boundary", func(s *Server, p *player, o *object, db *recordedGodstone) { o.x = 15 }},
		{"nonfinite_position", func(s *Server, p *player, o *object, db *recordedGodstone) { o.x = float32(math.NaN()) }},
		{"dead", func(s *Server, p *player, o *object, db *recordedGodstone) { p.dead = true }},
		{"equipped", func(s *Server, p *player, o *object, db *recordedGodstone) {
			p.equipment = p.cube[:1]
			p.cube = p.cube[1:]
		}},
		{"armor", func(s *Server, p *player, o *object, db *recordedGodstone) { s.data.Items[100].EquipmentType = "ARMOR" }},
		{"missing_stone", func(s *Server, p *player, o *object, db *recordedGodstone) { p.cube = p.cube[:1] }},
		{"not_godstone", func(s *Server, p *player, o *object, db *recordedGodstone) { s.data.Items[168000002].Godstone = nil }},
		{"empty_stone", func(s *Server, p *player, o *object, db *recordedGodstone) { p.cube[1].Count = 0 }},
		{"insufficient_kinah", func(s *Server, p *player, o *object, db *recordedGodstone) { p.kinah.Count = 99999 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, o, db, packets := godstoneFixture()
			tc.change(s, p, o, db)
			before := p.kinah.Count
			godstoneRequest(p, o.id)
			if db.calls != 0 || p.kinah.Count != before {
				t.Fatal("invalid request reached persistence or spent kinah")
			}
			if bytes.Equal(packets.last(smSystemMessage), systemMessage(msgGodstoneSuccess, descriptionID(123)).Data) {
				t.Fatal("invalid request announced success")
			}
		})
	}
}

func TestGodstoneSocketPersistenceFailure(t *testing.T) {
	_, p, o, db, packets := godstoneFixture()
	db.err = errors.New("commit failed")
	godstoneRequest(p, o.id)
	if db.calls != 1 || p.kinah.Count != 250000 || p.cube[0].Godstone != 0 || p.cube[1].Count != 2 || len(packets.frames) != 0 {
		t.Fatal("failed commit changed inventory or sent success")
	}
}

func TestGodstoneSocketTruncated(t *testing.T) {
	for size := range 12 {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			_, p, _, db, packets := godstoneFixture()
			p.conn.godstoneSocket(wire.NewReader(make([]byte, size)))
			if db.calls != 0 || len(packets.frames) != 0 {
				t.Fatal("truncated request had side effects")
			}
		})
	}
	handlers[cmGodstoneSocket](&conn{}, wire.NewReader(make([]byte, 12)))
	if clientPacketStates[cmGodstoneSocket] != inGame {
		t.Fatal("socket request must require an in-game client")
	}
}

func TestGodstoneMetadata(t *testing.T) {
	d := staticDataOrSkip(t)
	stone := d.Items[168000002].Godstone
	if stone == nil || *stone != (data.Godstone{SkillID: 8255, SkillLevel: 1, Probability: 100}) {
		t.Fatalf("godstone metadata: %+v", stone)
	}
	if d.Items[168000001].Godstone != nil {
		t.Fatal("item without godstone metadata became socketable")
	}
}

func TestGodstoneSocketUsesJavaHorizontalRange(t *testing.T) {
	_, p, o, db, _ := godstoneFixture()
	o.x, o.z = 14.99, 1000
	godstoneRequest(p, o.id)
	if db.calls != 1 {
		t.Fatal("Java's socket range checks XY independently of height")
	}
}

func TestGodstoneAppearancePacket(t *testing.T) {
	s, p, _, _, _ := godstoneFixture()
	item := p.cube[0]
	item.Godstone = 168000002
	item.Equipped, item.Slot = true, 1
	p.equipment = []*store.Item{item}
	want := []byte{smUpdatePlayerAppearance, 99, 0, 0, 0, 1, 0, 100, 0, 0, 0, 2, 122, 3, 10, 0, 0, 0, 0, 0, 0}
	if got := s.appearancePacket(p).Data; !bytes.Equal(got, want) {
		t.Fatalf("weapon appearance = %x, want %x", got, want)
	}
}

func TestGodstoneSocketRequiresAtomicPersistence(t *testing.T) {
	s, p, o, _, packets := godstoneFixture()
	s.items = noItems{}
	godstoneRequest(p, o.id)
	if p.kinah.Count != 250000 || p.cube[0].Godstone != 0 || p.cube[1].Count != 2 || len(packets.frames) != 0 {
		t.Fatal("socket mutated without atomic persistence")
	}
}

func TestGodstoneProcsUseTheirSkillOnTheTarget(t *testing.T) {
	d := staticDataOrSkip(t)
	dd := *d
	dd.Items = maps.Clone(d.Items)
	dd.Items[1] = &data.ItemTemplate{ID: 1, Godstone: &data.Godstone{SkillID: 8255, SkillLevel: 1, Probability: 1001}}
	dd.Items[2] = &data.ItemTemplate{ID: 2, Godstone: &data.Godstone{SkillID: 8256, SkillLevel: 1}}
	s := testServer(&dd)
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1002)
	p.targetID = o.id
	var used []int32
	p.fx.onSkillUse(&effect{}, func(sk *skill) {
		used = append(used, sk.tmpl.ID)
		if sk.first != o || sk.level != 1 {
			t.Errorf("godstone skill on %v at level %d", sk.first, sk.level)
		}
	})
	weapon := p.equipment[0]
	offHand := &store.Item{UniqueID: 0x1057c, ItemID: 100600034, Count: 1, Equipped: true, Slot: data.SlotMainOff, Godstone: 1}
	p.equipment = append(p.equipment, offHand)
	s.attacking(p, o)
	if len(used) != 0 {
		t.Fatalf("a godstone procced without a socketed weapon in hand: %v", used)
	}
	weapon.Godstone = 2 // probability 0
	s.attacking(p, o)
	if len(used) != 0 {
		t.Fatal("a godstone of probability 0 procced")
	}
	weapon.Godstone = 1
	s.attacking(p, o)
	if !reflect.DeepEqual(used, []int32{8255}) {
		t.Fatalf("godstone skills used %v, want [8255]", used)
	}
}
