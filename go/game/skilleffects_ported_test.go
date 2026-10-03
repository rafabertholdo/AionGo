package game

import (
	"bytes"
	"encoding/binary"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// cast puts the skill's effects on the target the way Skill.endCast does.
func cast(s *Server, effector, target creature, id int32) *effect {
	tmpl := s.data.Skills[id]
	e := s.newEffect(effector, target, tmpl, tmpl.Level, 0)
	e.initialize()
	e.apply()
	return e
}

func TestSearchSeesThroughHideWhileItLasts(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		packets := &questPackets{}
		p.conn.tap = packets.tap
		s.spawn(p)
		cast(s, p, p, 693) // Mind's Eye: search value 1 for a minute
		if p.seeState != 1 {
			t.Fatalf("see state %d, want SEARCH1", p.seeState)
		}
		if got := packets.last(smPlayerState); !bytes.Equal(got, []byte{smPlayerState, 0, 0, 2, 0, 0, 1, 0}) {
			t.Fatalf("SM_PLAYER_STATE %x", got)
		}
		time.Sleep(61 * time.Second)
		synctest.Wait()
		if p.seeState != 0 || packets.last(smPlayerState)[6] != 0 {
			t.Fatal("search outlived its effect")
		}
	})
}

func TestReturnPointScrollGoesToItsPortalExit(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		p.appearance = &store.Appearance{}
		p.level = 10 // the scroll's level
		s.spawn(p)
		item := &store.Item{UniqueID: 0x40000, ItemID: 164000085, Owner: p.ID, Count: 1}
		p.cube = []*store.Item{item}
		s.useItem(p, item, nil)
		time.Sleep(6 * time.Second)
		synctest.Wait()
		if p.WorldID != 110010000 || p.X != 1476.3 || p.Y != 1595.5 || p.Z != 572.9 {
			t.Fatalf("scroll took the player to %d %v %v %v", p.WorldID, p.X, p.Y, p.Z)
		}
	})
	// The skill without an item does nothing.
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	if e := cast(s, p, p, 8198); len(e.success) != 0 {
		t.Fatal("returnpoint landed without a return item")
	}
}

func TestMpUseOverTimeDrainsUntilMpRunsOut(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.spawn(p)
		maxMP := p.stats.current(data.MaxMP)
		cost := maxMP * 4 / 100
		p.life.MP = cost*2 + cost/2
		p.restore = newTask()   // no regeneration here
		e := cast(s, p, p, 912) // Gale Move: 4% of max MP every 6 s
		if len(e.success) == 0 || len(p.fx.noshow) != 1 {
			t.Fatal("Gale Move did not start")
		}
		synctest.Wait()
		if p.life.MP != cost+cost/2 {
			t.Fatalf("MP %d after the first drain, want %d", p.life.MP, cost+cost/2)
		}
		time.Sleep(6 * time.Second)
		synctest.Wait()
		if p.life.MP != cost/2 || e.stopped {
			t.Fatalf("MP %d after the second drain, want %d", p.life.MP, cost/2)
		}
		time.Sleep(6 * time.Second)
		synctest.Wait()
		if !e.stopped {
			t.Fatal("Gale Move outlasted its MP")
		}
		p.life.MP = cost - 1
		if e := cast(s, p, p, 912); len(e.success) != 0 {
			t.Fatal("Gale Move started without the MP for it")
		}
	})
}

func TestOneTimeBoostEndsAfterItsPhysicalSkills(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.spawn(p)
		e := cast(s, p, p, 684) // True Shot Mind: the next five physical skills
		if len(p.fx.abnormal) == 0 || len(p.fxMods) == 0 {
			t.Fatal("boost did not start")
		}
		physical, magical := &skill{tmpl: &data.SkillTemplate{Type: "PHYSICAL"}}, &skill{tmpl: &data.SkillTemplate{Type: "MAGICAL"}}
		for range 4 {
			p.fx.usingSkill(physical)
			p.fx.usingSkill(magical)
		}
		if e.stopped {
			t.Fatal("boost ended before its fifth physical skill")
		}
		p.fx.usingSkill(physical)
		if !e.stopped || len(p.fx.skillHooks) != 0 || len(p.fxMods) != 0 {
			t.Fatal("boost outlasted its fifth physical skill")
		}
	})
}

func TestMagicCounterHurtsMagicUsers(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1002)
	var e *effect
	for e == nil || len(e.success) == 0 {
		e = cast(s, p, o, 1560) // Counter Magic I: 5% of max HP, at most 500
	}
	before := o.hp
	o.fx.usingSkill(&skill{tmpl: &data.SkillTemplate{Type: "PHYSICAL"}})
	if o.hp != before {
		t.Fatal("a physical skill set off the counter")
	}
	o.fx.usingSkill(&skill{tmpl: &data.SkillTemplate{Type: "MAGICAL"}})
	if want := before - min(o.maxHP/100*5, 500); o.hp != want {
		t.Fatalf("hp %d, want %d", o.hp, want)
	}
	e.end()
	if len(o.fx.skillHooks) != 0 {
		t.Fatal("counter outlived its effect")
	}
}

func TestPetOrderTellsTheSummonsSkill(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	packets := &questPackets{}
	p.conn.tap = packets.tap
	target, _ := fighter(t, s, 1001)
	target.ID = 0x20001
	if cast(s, p, target, 1601); packets.last(smSummonUseskill) != nil {
		t.Fatal("an order was sent without a summon")
	}
	p.summon = &object{id: 0x31000, npc: d.Npcs[201011]}
	cast(s, p, target, 1601)
	got := packets.last(smSummonUseskill)
	want := []byte{smSummonUseskill, 0, 0x10, 3, 0, 0, 0, 1, 1, 0, 2, 0}
	binary.LittleEndian.PutUint16(want[5:7], 18080)
	if !bytes.Equal(got, want) {
		t.Fatalf("SM_SUMMON_USESKILL %x, want %x", got, want)
	}
}
