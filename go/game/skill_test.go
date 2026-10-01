package game

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// TestFlameBolt has the level 1 mage cast Flame Bolt (skill 1351) on a monster, as the 1.9 client did in
// testdata/play-session-1.9.txt: the cast is announced, two seconds later it ends and the monster is hurt.
func TestFlameBolt(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.startWorldTasks()
	p, tap := fighter(t, s, 1000)
	p.skills = append(p.skills, store.Skill{ID: 1351, Level: 1})
	s.spawn(p)
	o := monster(t, s, 1005)
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.targetID = o.id
	mp := p.life.MP
	tmpl := d.Skills[1351]
	if tmpl == nil {
		s.visMu.Unlock()
		t.Fatal("no Flame Bolt")
	}
	s.playerUseSkill(p, tmpl, 0, 0, 0, 0)
	if p.cast == nil || tap.count(smCastspell) != 1 {
		s.visMu.Unlock()
		t.Fatalf("the cast didn't begin: %d packets", tap.count(smCastspell))
	}
	// The skill can't be used again while it is cast and cooling down.
	s.playerUseSkill(p, tmpl, 0, 0, 0, 0)
	s.visMu.Unlock()
	if tap.count(smCastspell) != 1 {
		t.Errorf("the skill was cast twice")
	}
	time.Sleep(2500 * time.Millisecond)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if tap.count(smCastspellEnd) != 1 {
		t.Fatalf("the cast didn't end: %d packets", tap.count(smCastspellEnd))
	}
	if p.life.MP >= mp {
		t.Errorf("no mana was used: %d, had %d", p.life.MP, mp)
	}
	if o.hp >= o.maxHP {
		t.Errorf("the monster is unhurt: %d/%d", o.hp, o.maxHP)
	}
	if p.cast != nil {
		t.Errorf("the player is still casting")
	}
	if !p.skillDisabled(1351) {
		t.Errorf("the skill isn't cooling down")
	}
}

// TestCastPacketsMatchClient19 rebuilds the first spell the 1.9 server saw Wrathchild cast.
func TestCastPacketsMatchClient19(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	p.ID = 0x10577
	o := monster(t, s, 1005)
	var begin, end string
	for _, row := range playSession(t) {
		if row[0] == "server" && row[1] == "SM_CASTSPELL" && begin == "" {
			begin = row[2]
		}
		if row[0] == "server" && row[1] == "SM_CASTSPELL_END" && end == "" {
			end = row[2]
		}
	}
	if begin == "" || end == "" {
		t.Fatal("no spell in the session")
	}
	// The monster's id and life are the ones of the recording.
	o.id = 0x4f5
	o.hp = o.maxHP * 42 / 100
	for o.hp*100/o.maxHP != 42 {
		o.hp++
	}
	sk := &skill{s: s, tmpl: d.Skills[1351], effector: p, level: 1, first: o, duration: 2000, chain: true}
	if got := hex.EncodeToString(sk.castStart().Data[1:]); got != begin {
		t.Errorf("SM_CASTSPELL differs from the 1.9 server's:\n%s", diffHex(got, begin))
	}
	payload, _ := hex.DecodeString(end)
	damage := int32(binary.LittleEndian.Uint32(payload[36:]))
	got := sk.castEnd(0, []*effect{{effected: o, r1: damage, attackStatus: statusNormalHit}})
	if got := hex.EncodeToString(got.Data[1:]); got != end {
		t.Errorf("SM_CASTSPELL_END differs from the 1.9 server's:\n%s", diffHex(got, end))
	}
}

// TestRootSkill has the mage root a monster with the skill of the recording (1373, an instant cast): it is held, and
// its client shown; the roots can be resisted, so it tries until one holds.
func TestRootSkill(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.startWorldTasks()
	p, tap := fighter(t, s, 1000)
	p.skills = append(p.skills, store.Skill{ID: 1373, Level: 1})
	s.spawn(p)
	o := monster(t, s, 1005)
	tmpl := d.Skills[1373]
	s.visMu.Lock()
	s.updateNpcKnown(o)
	p.targetID = o.id
	for try := 0; try < 50 && !o.fx.isSet(effectRoot); try++ {
		delete(p.cooldowns, 1373)
		p.life.MP = p.stats.current(data.MaxMP)
		s.playerUseSkill(p, tmpl, 0, 0, 0, 0)
	}
	rooted := o.fx.isSet(effectRoot)
	s.visMu.Unlock()
	if !rooted {
		t.Fatal("the monster resisted fifty roots")
	}
	time.Sleep(400 * time.Millisecond)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if tap.count(smTargetImmobilize) == 0 || tap.count(smAbnormalEffect) == 0 {
		t.Errorf("the player was sent %d immobilizes and %d icon lists", tap.count(smTargetImmobilize), tap.count(smAbnormalEffect))
	}
	// The icon list has the shape of the recording's (whose monster was 0x505).
	want := "05050000" + "01" + "00000000" + "10000000" + "0100" + "5d05" + "01" + "01"
	o.id = 0x505
	if got := hex.EncodeToString(s.abnormalEffect(o).Data[1:]); got[:len(want)] != want {
		t.Errorf("icons %s, want %s…", got, want)
	}
	o.fx.removeEffect(1373)
	if o.fx.isState(effectRoot) {
		t.Errorf("the root didn't end")
	}
}
