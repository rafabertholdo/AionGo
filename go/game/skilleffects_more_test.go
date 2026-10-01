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
