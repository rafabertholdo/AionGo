package game

import (
	"encoding/binary"
	"fmt"
	"testing"

	"aionlightning/wire"
)

func TestCampaignAerialBattle(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, world := range []int32{310010000, 320010000} {
		t.Run(fmt.Sprint(world), func(t *testing.T) {
			s := testServer(d)
			s.visMu.Lock()
			defer s.visMu.Unlock()
			in := s.newInstance(world)
			defer s.destroyInstance(in)
			other := s.newInstance(world)
			defer s.destroyInstance(other)
			p, _ := fighter(t, s, 1000)
			attacks := 0
			p.conn.tap = func(w *wire.Writer) {
				if w.Data[0] != smAttack {
					return
				}
				attacks++
				attacker := s.byID[int32(binary.LittleEndian.Uint32(w.Data[1:5]))]
				target := s.byID[int32(binary.LittleEndian.Uint32(w.Data[9:13]))]
				if attacker == nil || target == nil {
					t.Fatal("scene attack references a missing actor")
				}
				if target.instance != in.id || attacker.npc.Race == target.npc.Race || !aerialBattleActor(target) {
					t.Fatal("scene attack targets a friend, quest NPC, or another instance")
				}
				if damage := binary.LittleEndian.Uint32(w.Data[18:22]); damage != 0 {
					t.Fatalf("scene attack inflicted %d damage", damage)
				}
			}
			var actors []*object
			for _, o := range s.byID {
				if o.instance != in.id {
					continue
				}
				if !aerialBattleActor(o) {
					if o.ai.scheduled() {
						t.Fatalf("quest NPC %d has scene AI", o.npc.ID)
					}
					continue
				}
				if !o.ai.scheduled() {
					t.Fatalf("flying soldier %d is idle", o.npc.ID)
				}
				o.watchers[p.ID] = p
				actors = append(actors, o)
			}
			if len(actors) == 0 {
				t.Fatal("campaign scene has no soldiers")
			}
			for range 6 {
				for _, o := range actors {
					o.ai.run()
				}
			}
			if attacks != len(actors)*3 {
				t.Fatalf("got %d attacks, want %d", attacks, len(actors)*3)
			}
			for _, o := range actors {
				if o.hp != o.maxHP || len(o.aggro) != 0 || o.move.scheduled() {
					t.Fatal("scene changed life, hate, or position")
				}
			}
			s.destroyInstance(in)
			for _, o := range actors {
				if o.ai.scheduled() || len(o.ai.desires) != 0 {
					t.Fatal("destroyed instance left a battle running")
				}
			}
		})
	}
}

func TestAerialBattleActorScope(t *testing.T) {
	d := staticDataOrSkip(t)
	o := &object{npc: d.Npcs[205003], worldID: 310010000}
	if aerialBattleActor(o) {
		t.Fatal("ground soldier is a battle actor")
	}
	o.spawnSpot.Fly = 1
	o.worldID = 310020000
	if aerialBattleActor(o) {
		t.Fatal("later campaign instance has introductory battle AI")
	}
	o.worldID = 310010000
	o.npc = d.Npcs[205000]
	if aerialBattleActor(o) {
		t.Fatal("quest NPC is a battle actor")
	}
}
