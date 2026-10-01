package game

import (
	"testing"
	"time"

	"aionlightning/wire"
)

// TestSummon has a spirit master call a fire spirit, order it to attack a monster, and let it go.
func TestSummon(t *testing.T) {
	d := staticDataOrSkip(t)
	const spirit = 201022 // Fire Spirit I
	npc := d.Npcs[spirit]
	if npc == nil || d.SummonStatsFor(spirit, npc.Level) == nil {
		t.Skip("no fire spirit in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	m := monster(t, s, 1005)
	s.visMu.Lock()
	s.updateNpcKnown(m)
	s.createSummon(p, spirit, 1)
	o := p.summon
	s.visMu.Unlock()
	if o == nil || tap.count(smSummonPanel) != 1 || tap.count(smSummonUpdate) == 0 {
		t.Fatalf("no summon: %v", tap.counts)
	}
	p.conn.summonCommand(packet(cmSummonCommand, func(w *wire.Writer) { w.C(summonAttack); w.D(0); w.D(0); w.D(m.id) }))
	if o.summonMode != summonAttack {
		t.Errorf("mode %d", o.summonMode)
	}
	for range 50 {
		s.visMu.Lock()
		o.lastAttack = time.Time{}
		hp := m.hp
		s.visMu.Unlock()
		p.conn.summonAttack(packet(cmSummonAttack, func(w *wire.Writer) { w.D(o.id); w.D(m.id); w.C(0); w.H(0); w.C(0) }))
		s.visMu.Lock()
		hurt := m.hp < hp
		s.visMu.Unlock()
		if hurt {
			break
		}
	}
	s.visMu.Lock()
	hurt := m.hp < m.maxHP
	s.visMu.Unlock()
	if !hurt {
		t.Errorf("the summon never hurt the monster")
	}
	p.conn.summonCommand(packet(cmSummonCommand, func(w *wire.Writer) { w.C(summonRelease); w.D(0); w.D(0); w.D(0) }))
	time.Sleep(summonReleaseDelay + 500*time.Millisecond)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.summon != nil || tap.count(smSummonPanelRemove) != 1 {
		t.Errorf("the summon wasn't let go: %v", tap.counts)
	}
}
