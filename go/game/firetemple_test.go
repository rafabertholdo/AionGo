package game

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestFireTempleKromedeIsMostlyTheCorrupt(t *testing.T) {
	hard := 0
	for range 10000 {
		switch fireTempleVariant(fireTempleWorld, vileJudgeKromede) {
		case vileJudgeKromede:
			hard++
		case kromedeTheCorrupt:
		default:
			t.Fatal("Kromede's spawn became another npc")
		}
	}
	if hard < 800 || hard > 1200 {
		t.Errorf("Vile Judge %d in 10000 runs, want about 1000", hard)
	}
	if fireTempleVariant(nochsanaWorld, vileJudgeKromede) != vileJudgeKromede || fireTempleVariant(fireTempleWorld, 212808) != 212808 {
		t.Error("the roll changed a spawn it should not")
	}
}

// instanceFight puts an immortal player beside the first of the npcs in a fresh run of the map.
func instanceFight(t *testing.T, s *Server, world int32, ids ...int32) (*player, *object, *instance) {
	t.Helper()
	p, _ := fighter(t, s, 1000)
	in := s.newInstance(world)
	var o *object
	for _, x := range s.byID {
		for _, id := range ids {
			if x.npc != nil && x.npc.ID == id && x.instance == in.id && !x.dead && o == nil {
				o = x
			}
		}
	}
	if o == nil {
		t.Fatalf("no %v in the run", ids)
	}
	p.WorldID, p.instance = in.world, in.id
	p.X, p.Y, p.Z = o.x+2, o.y, o.z
	p.adminInvulnerable = true
	s.spawnLocked(p)
	return p, o, in
}

func TestFireTempleKromedeTrapsAndLastStand(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		p, k, in := instanceFight(t, s, fireTempleWorld, kromedeTheCorrupt, vileJudgeKromede)
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll() // her bleeds leave periodic effects on the player
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		traps := func() (n int) {
			s.visMu.Lock()
			defer s.visMu.Unlock()
			for _, o := range s.byID {
				if o.npc != nil && o.npc.ID == kromedeTrap && o.instance == in.id {
					n++
				}
			}
			return n
		}
		s.npcHit(k, p, 0, statusDamage, 1)
		if k.script == nil || k.ai.has(&skillUseDesire{}) {
			t.Fatal("Kromede's script did not start, or she still casts at random")
		}
		s.visMu.Unlock()
		time.Sleep(30 * time.Second)
		synctest.Wait()
		if n := traps(); n != 2 {
			t.Fatalf("%d traps at 30s above half HP, want 2", n)
		}
		s.visMu.Lock()
		k.hp = k.maxHP * 40 / 100
		s.visMu.Unlock()
		time.Sleep(35 * time.Second)
		synctest.Wait()
		if n := traps(); n != 3 {
			t.Fatalf("%d traps 35s later below half HP, want 3", n)
		}
		s.visMu.Lock()
		k.hp = k.maxHP / 10
		s.visMu.Unlock()
		time.Sleep(6 * time.Second)
		synctest.Wait()
		s.visMu.Lock()
		if !k.script.used[20] {
			t.Error("her last stand below 20% did not fire")
		}
		s.npcHit(k, p, 0, statusDamage, k.hp)
		s.visMu.Unlock()
		if !k.dead || k.script != nil {
			t.Fatal("the script outlived Kromede")
		}
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if n := traps(); n != 0 {
			t.Errorf("%d traps outlived their burst", n)
		}
	})
}

func TestFireTempleObscuraFleesOnceBelowHalfHP(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		p, o, in := instanceFight(t, s, fireTempleWorld, fireTempleObscuras...)
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll() // their skills leave periodic effects on the player
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		s.npcHit(o, p, 0, statusDamage, 1)
		if o.fled {
			t.Fatal("it fled above half HP")
		}
		o.hp = o.maxHP / 3
		for i := 0; i < 200 && !o.fled; i++ {
			s.npcHit(o, p, 0, statusDamage, 1)
		}
		if !o.fled || o.script == nil {
			t.Fatal("it never fled below half HP")
		}
		s.visMu.Unlock()
		var farthest float64
		for range 40 {
			time.Sleep(time.Second)
			synctest.Wait()
			s.visMu.Lock()
			if o.script.fleeing() {
				farthest = max(farthest, distance3D(o.x, o.y, o.z, p.X, p.Y, p.Z))
			}
			s.visMu.Unlock()
		}
		if farthest < 5 {
			t.Errorf("it got only %.1f m from its target while fleeing", farthest)
		}
		s.visMu.Lock()
		defer s.visMu.Unlock()
		if o.script != nil && !o.dead {
			t.Error("its flee sequence never ended")
		}
	})
}
