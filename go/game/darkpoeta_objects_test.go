package game

import (
	"encoding/binary"
	"testing"

	"aionlightning/wire"
)

func TestDarkPoetaBarricadeDamage(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, id := range []int32{700517, 700556, 700558} {
		t.Run(d.Npcs[id].Name, func(t *testing.T) {
			s := testServer(d)
			p, _ := fighter(t, s, 1000)
			var packet *wire.Writer
			p.conn.tap = func(w *wire.Writer) {
				if w.Data[0] == smAttackStatus {
					packet = w
				}
			}
			p.WorldID, p.instance = darkPoetaWorld, 1
			o := &object{id: 0x30000, worldID: darkPoetaWorld, instance: 1, npc: d.Npcs[id], x: 1002, y: 1000, z: 130}
			s.initNpc(o)
			s.visMu.Lock()
			defer s.visMu.Unlock()
			defer o.ai.stop()
			o.watchers[p.ID] = p
			// Multiple regular hits each retain their own one-point damage in packets.
			for range 16 {
				for _, hit := range s.physicalAttack(p, o) {
					if hit.damage < 0 || hit.damage > 1 {
						t.Fatalf("regular hit damage=%d", hit.damage)
					}
				}
			}
			for _, hit := range s.splitDamage(p, o, 3, 10000, statusDodge) {
				if darkPoetaDamage(o, hit.damage) != 0 {
					t.Fatal("miss dealt damage")
				}
			}
			e := &effect{effected: o}
			e.setResult(9999, statusNormalHit)
			if e.r1 != 1 {
				t.Fatal("skill packet damage was not capped")
			}
			e.setResult(0, statusResist)
			if e.r1 != 0 {
				t.Fatal("resisted skill dealt damage")
			}
			hp := o.hp
			s.applyAttackHits(o, p, []attackResult{{damage: 1}, {damage: 1}, {damage: 1}})
			for _, kind := range []byte{statusRegular, statusDamage} {
				s.npcHit(o, p, 0, kind, 9999)
			}
			if o.hp != hp-5 {
				t.Fatalf("HP loss=%d, want 5", hp-o.hp)
			}
			if info := o.aggro[p.ID]; info == nil || info.damage != 5 {
				t.Fatalf("aggro damage=%+v", info)
			}
			if packet == nil || int32(binary.LittleEndian.Uint32(packet.Data[5:])) != -1 {
				t.Fatalf("attack status did not report one damage: %v", packet)
			}
			o.worldID = 210010000
			if darkPoetaDamage(o, 9999) != 9999 {
				t.Fatal("cap leaked to another world")
			}
		})
	}
}

func TestDarkPoetaBombWallSuccessfulCastAndIsolation(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	p.WorldID, p.instance, p.spawned = darkPoetaWorld, 1, true
	in := &instance{world: darkPoetaWorld, id: 1, dp: &darkPoeta{}}
	s.instances = map[[2]int32]*instance{{darkPoetaWorld, 1}: in}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	walls := []*object{}
	for i := 0; i < 3; i++ {
		o := &object{id: int32(0x30000 + i), worldID: darkPoetaWorld, instance: 1, npc: d.Npcs[700516], x: 1003, y: 1000, z: 130, noRespawn: true}
		if i == 1 {
			o.instance = 2
		}
		if i == 2 {
			o.x = 1008
		}
		s.initNpc(o)
		s.byID[o.id] = o
		s.addObject(o)
		walls = append(walls, o)
	}
	sk := &skill{s: s, tmpl: d.Skills[18130], effector: p, first: p, level: 1, item: d.Items[164000096]}
	sk.cancelled = true
	p.cast = sk
	sk.endCast()
	if s.byID[walls[0].id] == nil {
		t.Fatal("cancelled bomb removed wall")
	}
	sk.cancelled = false
	sk.endCast()
	if s.byID[walls[0].id] != nil {
		t.Fatal("successful bomb did not remove nearby wall")
	}
	if s.byID[walls[1].id] == nil || s.byID[walls[2].id] == nil {
		t.Fatal("bomb removed another run or distant wall")
	}
	// Duplicate completion and non-item casts cannot remove additional walls.
	sk.endCast()
	sk.item = nil
	s.darkPoetaSpell(sk)
	if s.byID[walls[1].id] == nil || s.byID[walls[2].id] == nil {
		t.Fatal("duplicate bomb removed protected walls")
	}
}
