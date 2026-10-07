package game

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

const aetherLabWorld = 310050000

// labCount counts the run's live npcs of a template.
func labCount(s *Server, in *instance, id int32) (n int) {
	s.visMu.Lock()
	defer s.visMu.Unlock()
	for _, o := range s.byID {
		if o.npc != nil && o.npc.ID == id && o.instance == in.id && !o.dead {
			n++
		}
	}
	return n
}

func TestAetherLabLepharistFleesThenCallsForHelp(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		p, o, in := instanceFight(t, s, aetherLabWorld, 212187)
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		var ally *object
		for _, x := range s.byID {
			if x != o && x.npc != nil && slices.Contains(lepharistFleers, x.npc.ID) && x.npc.Tribe == o.npc.Tribe && x.instance == in.id && !x.dead {
				ally = x
				break
			}
		}
		s.moveObject(ally, o.x-3, o.y, o.z, 0)
		s.npcHit(o, p, 0, statusDamage, 1)
		o.hp = o.maxHP * 30 / 100
		s.visMu.Unlock()
		var farthest float64
		for range 30 {
			time.Sleep(time.Second)
			synctest.Wait()
			s.visMu.Lock()
			if o.script.fleeing() {
				farthest = max(farthest, distance3D(o.x, o.y, o.z, p.X, p.Y, p.Z))
			}
			if ally.aggro[p.ID] == nil { // the call goes out from where it stops fleeing
				s.moveObject(ally, o.x, o.y, o.z, 0)
			}
			s.visMu.Unlock()
		}
		s.visMu.Lock()
		defer s.visMu.Unlock()
		if farthest < 5 {
			t.Errorf("it got only %.1f m away while fleeing below 35%% HP", farthest)
		}
		if ally.aggro[p.ID] == nil {
			t.Error("its ally within 10 m was not called on its target")
		}
	})
}

func TestAetherLabScholarSummonsAPretor(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		p, o, in := instanceFight(t, s, aetherLabWorld, lepharistScholars...)
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		s.npcHit(o, p, 0, statusDamage, 1)
		s.visMu.Unlock()
		time.Sleep(7 * time.Second)
		synctest.Wait()
		if n := labCount(s, in, summonedPretor); n != 0 {
			t.Fatalf("%d pretors above 70%% HP, want none", n)
		}
		s.visMu.Lock()
		o.hp = o.maxHP * 60 / 100
		s.visMu.Unlock()
		time.Sleep(7 * time.Second)
		synctest.Wait()
		if n := labCount(s, in, summonedPretor); n != 1 {
			t.Fatalf("%d pretors at 60%% HP, want 1", n)
		}
		time.Sleep(7 * time.Second)
		synctest.Wait()
		if n := labCount(s, in, summonedPretor); n != 1 {
			t.Fatalf("%d pretors after another timer, want still 1", n)
		}
	})
}

func TestAetherLabSniperDropsATrapAndFlees(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		p, o, in := instanceFight(t, s, aetherLabWorld, lepharistSnipers...)
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		s.npcHit(o, p, 0, statusDamage, 1)
		o.hp = o.maxHP * 30 / 100
		s.visMu.Unlock()
		trapped, fled := false, false
		for range 20 {
			time.Sleep(time.Second)
			synctest.Wait()
			trapped = trapped || labCount(s, in, lepharistSnare) > 0
			s.visMu.Lock()
			fled = fled || o.script.fleeing()
			s.visMu.Unlock()
		}
		if !trapped || !fled {
			t.Fatalf("trap=%v flee=%v below 35%% HP, want both", trapped, fled)
		}
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if n := labCount(s, in, lepharistSnare); n != 0 {
			t.Errorf("%d traps outlived their burst", n)
		}
	})
}

func TestAetherLabDeathSpawns(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		p, pretor, in := instanceFight(t, s, aetherLabWorld, perfectedPretor)
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		var boss *object
		for _, x := range s.byID {
			if x.npc != nil && x.npc.ID == rm78c && x.instance == in.id {
				boss = x
			}
		}
		s.npcHit(pretor, p, 0, statusDamage, pretor.hp)
		s.npcHit(boss, p, 0, statusDamage, boss.hp)
		s.visMu.Unlock()
		if labCount(s, in, perfectedMud) != 1 || labCount(s, in, strangeCreature) != 1 {
			t.Fatalf("mudthorn=%d creature=%d after the deaths, want 1 each",
				labCount(s, in, perfectedMud), labCount(s, in, strangeCreature))
		}
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if labCount(s, in, strangeCreature) != 0 {
			t.Error("RM-78c's burst outlived its cast")
		}
		time.Sleep(600 * time.Second)
		synctest.Wait()
		if labCount(s, in, perfectedMud) != 0 {
			t.Error("the mudthorn outlived its 600 seconds")
		}
	})
}

func TestAetherLabBossPatternsFireTheirThresholds(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, c := range []struct {
		npc   int32
		steps []struct{ hp, flag int32 } // HP set, then the once-flag that must fire
	}{
		{perfectedPretor, nil},
		{212193, []struct{ hp, flag int32 }{{50, 1}, {30, 2}}},
		{212341, []struct{ hp, flag int32 }{{50, 1}, {30, 2}}},
		{212342, []struct{ hp, flag int32 }{{50, 1}, {30, 2}}},
		{212181, []struct{ hp, flag int32 }{{100, 1}, {50, 2}, {30, 3}}},
		{212191, []struct{ hp, flag int32 }{{100, 1}, {50, 2}, {30, 3}}},
		{rm78c, []struct{ hp, flag int32 }{{70, 1}, {40, 2}, {10, 3}}},
	} {
		synctest.Test(t, func(t *testing.T) {
			s := testServer(d)
			s.visMu.Lock()
			p, boss, in := instanceFight(t, s, aetherLabWorld, c.npc)
			defer func() {
				s.visMu.Lock()
				p.fx.removeAll()
				s.destroyInstance(in)
				s.visMu.Unlock()
			}()
			tap := &tapped{counts: map[byte]int{}}
			p.conn.tap = tap.tap
			s.npcHit(boss, p, 0, statusDamage, 1)
			if boss.script == nil || boss.ai.has(&skillUseDesire{}) {
				t.Fatalf("%d: its pattern did not start, or it still casts at random", c.npc)
			}
			s.visMu.Unlock()
			time.Sleep(20 * time.Second)
			synctest.Wait()
			wandered := false
			for _, step := range c.steps {
				s.visMu.Lock()
				boss.hp = boss.maxHP * step.hp / 100
				s.visMu.Unlock()
				fired := false
				for range 40 {
					time.Sleep(time.Second)
					synctest.Wait()
					s.visMu.Lock()
					fired = fired || boss.script != nil && boss.script.used[int(step.flag)]
					wandered = wandered || boss.script.fleeing()
					s.visMu.Unlock()
				}
				if !fired {
					t.Fatalf("%d: at %d%% HP its step %d did not fire", c.npc, step.hp, step.flag)
				}
			}
			if c.npc == rm78c && !wandered {
				t.Errorf("RM-78c never ran about between 31%% and 50%% HP")
			}
			if tap.count(smCastspell) == 0 {
				t.Errorf("%d: its pattern cast nothing", c.npc)
			}
			s.visMu.Lock()
			s.npcHit(boss, p, 0, statusDamage, boss.hp)
			s.visMu.Unlock()
			if !boss.dead || boss.script != nil {
				t.Errorf("%d: its pattern outlived it", c.npc)
			}
		})
	}
}
