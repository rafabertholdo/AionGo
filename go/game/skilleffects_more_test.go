package game

import "testing"

// TestTransformSkill has a player use a transform skill (Grave Knight food) on itself: it looks like the model, and
// it changes back when the effect ends.
func TestTransformSkill(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	tmpl := d.Skills[9100]
	if tmpl == nil {
		t.Skip("no skill 9100")
	}
	(&skill{s: s, tmpl: tmpl, effector: p, level: 1, first: p}).use()
	if p.transformed != 202515 || tap.count(smTransform) != 1 {
		t.Fatalf("model %d, %d transform packets", p.transformed, tap.count(smTransform))
	}
	if !p.fx.isSet(effectShapeChange) {
		t.Errorf("no shape change state")
	}
	p.fx.removeEffect(9100)
	if p.transformed != 0 || tap.count(smTransform) != 2 || p.fx.isSet(effectShapeChange) {
		t.Errorf("didn't change back: model %d, %d packets", p.transformed, tap.count(smTransform))
	}
}

// TestAlwaysBlock has the effect guarantee as many blocks as it says.
func TestAlwaysBlock(t *testing.T) {
	var fx effectController
	fx.always = []*alwaysHit{{status: statusBlock, left: 2}}
	if fx.guarantees(statusDodge) || !fx.guarantees(statusBlock) || !fx.guarantees(statusBlock) || fx.guarantees(statusBlock) {
		t.Errorf("wrong guarantees")
	}
}

// TestTargetRaceModifier adds skill 1135's bonus against Asmodian players only (TargetRaceDamageModifier).
func TestTargetRaceModifier(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	tmpl := d.Skills[1135]
	if tmpl == nil {
		t.Skip("no skill 1135")
	}
	p, _ := fighter(t, s, 1000)
	other, _ := fighter(t, s, 1005)
	fx := s.skillEffects(tmpl)[0]
	e := &effect{s: s, tmpl: tmpl, level: 1, effector: p, effected: other}
	for _, c := range []struct {
		race string
		want int32
	}{{"ELYOS", 100}, {"ASMODIANS", 100 + 2043 + 53}} {
		other.Race = c.race
		if got := e.applyActionModifiers(fx, 100); got != c.want {
			t.Errorf("against %s: %d, want %d", c.race, got, c.want)
		}
	}
	e.effected = monster(t, s, 1005)
	if got := e.applyActionModifiers(fx, 100); got != 100 {
		t.Errorf("against an npc: %d, want 100", got)
	}
}
