package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// TestDeathCostsExperience has a level 10 player killed by a monster: a third of what dying costs is lost for good
// and the rest can be recovered.
func TestDeathCostsExperience(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.setExp(p, d.ExpStart(10)+1000000)
	before := p.Exp
	s.calculateExpLoss(p)
	if p.Exp >= before || p.RecoverExp == 0 {
		t.Fatalf("exp %d → %d, recoverable %d", before, p.Exp, p.RecoverExp)
	}
	recoverable, after := p.RecoverExp, p.Exp
	s.resetRecoverableExp(p)
	if p.RecoverExp != 0 || p.Exp != after+recoverable {
		t.Errorf("exp %d after healing, want %d", p.Exp, after+recoverable)
	}
	if permanent := before - p.Exp; permanent*2 < recoverable/2 || permanent > recoverable {
		t.Errorf("%d lost for good, %d recoverable", permanent, recoverable)
	}
}

// TestSelfReviveStone has a dead player with a self resurrection stone use it: it is alive with 15% of its life,
// and the stone is used up.
func TestSelfReviveStone(t *testing.T) {
	d := staticDataOrSkip(t)
	if d.Items[161001001] == nil {
		t.Skip("no resurrection stone in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	s.visMu.Lock()
	s.addItem(p, 161001001, 1)
	s.reducePlayerHP(p, p.life.HP, nil)
	dead := p.dead
	s.visMu.Unlock()
	if !dead || tap.count(smDie) != 1 {
		t.Fatalf("dead %v, %d smDie", dead, tap.count(smDie))
	}
	p.conn.revive(wire.NewReader([]byte{reviveItem}))
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.dead || len(p.cube) != 0 {
		t.Errorf("dead %v, %d items left", p.dead, len(p.cube))
	}
	if want := p.stats.current(data.MaxHP) * 15 / 100; p.life.HP != want {
		t.Errorf("hp %d, want %d", p.life.HP, want)
	}
}
