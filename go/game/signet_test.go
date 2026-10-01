package game

import (
	"slices"
	"testing"
	"time"
)

// TestSignets has an assassin's carve skill put a signet on a monster, level it up, and a burst use it up.
func TestSignets(t *testing.T) {
	d := staticDataOrSkip(t)
	var carve, burst int32
	for _, id := range slices.Sorted(mapKeys(d.Skills)) {
		sk := d.Skills[id]
		if carve == 0 && skillHasEffect(sk, "carvesignet") && sk.Effects[0].Node.Str("signet") == "SYSTEM_SKILL_SIGNET1" {
			carve = id
		}
		if burst == 0 && skillHasEffect(sk, "signetburst") && sk.Effects[0].Node.Str("signet") == "SYSTEM_SKILL_SIGNET1" {
			burst = id
		}
	}
	if carve == 0 || burst == 0 {
		t.Skip("no signet skills in the data")
	}
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o.maxHP, o.hp = 1000000, 1000000
	apply := func(id int32) {
		e := s.newEffect(p, o, d.Skills[id], 1, 0)
		e.initialize()
		e.apply()
	}
	signet := func() int32 {
		if e := o.fx.abnormal["SYSTEM_SKILL_SIGNET1"]; e != nil {
			return e.level
		}
		return 0
	}
	for range 200 {
		if signet() == 2 {
			break
		}
		apply(carve)
	}
	if signet() != 2 {
		t.Fatalf("the signet is level %d", signet())
	}
	hp := o.hp
	apply(burst)
	if signet() != 0 || o.hp >= hp {
		t.Errorf("signet %d, hp %d → %d", signet(), hp, o.hp)
	}
}

// TestTrap has a trap wait where a hunter puts it, and explode into its skill when a monster comes near.
func TestTrap(t *testing.T) {
	d := staticDataOrSkip(t)
	var npc, skillID, seconds int32
	for _, id := range slices.Sorted(mapKeys(d.Skills)) {
		sk := d.Skills[id]
		if !skillHasEffect(sk, "summontrap") {
			continue
		}
		n := sk.Effects[0].Node
		if tmpl := d.Npcs[n.Int("npc_id")]; tmpl != nil && tmpl.SRange > 0 && d.Skills[n.Int("skill_id")] != nil {
			npc, skillID, seconds = n.Int("npc_id"), n.Int("skill_id"), n.Int("time")
			break
		}
	}
	if npc == 0 {
		t.Skip("no trap in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	monster(t, s, 1000+float32(d.Npcs[npc].SRange)/2) // before the trap's timers run
	s.visMu.Lock()
	s.spawnTrap(p, npc, skillID, seconds)
	var trap *object
	for _, o := range s.byID {
		if o.owner == p {
			trap = o
		}
	}
	s.visMu.Unlock()
	if trap == nil {
		t.Fatalf("no trap appeared: %v", tap.counts)
	}
	time.Sleep(3 * time.Second)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if tap.count(smCastspell) == 0 {
		t.Errorf("the trap never went off: %v", tap.counts)
	}
}
