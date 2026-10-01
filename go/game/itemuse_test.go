package game

import (
	"testing"

	"aionlightning/game/store"
)

// TestUsePotion has a wounded player drink a healing potion: it is used up and cools down.
func TestUsePotion(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	// The lowest numbered item of level 1 that heals.
	var id int32
	for candidate, tmpl := range s.data.Items {
		if tmpl.Level > 1 || tmpl.UseDelay == 0 || len(tmpl.Actions) != 1 || tmpl.Actions[0].Name != "skilluse" || (id != 0 && candidate > id) {
			continue
		}
		if skill := s.data.Skills[tmpl.Actions[0].Int("skillid")]; skill != nil && (skillHasEffect(skill, "heal") || skillHasEffect(skill, "itemheal")) {
			id = candidate
		}
	}
	if id == 0 {
		t.Skip("no low level healing item")
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, id, 2)
	var item *store.Item
	if len(p.cube) > 0 {
		item = p.cube[0]
	}
	tmpl := s.data.Items[id]
	if item == nil {
		t.Fatalf("item %d wasn't added", id)
	}
	p.life.HP = 1
	s.useItem(p, item, nil)
	if tap.count(smItemUsageAnimation) != 1 {
		t.Fatalf("no usage animation for item %d (classes %v, %d actions)", id, tmpl.Restrict, len(tmpl.Actions))
	}
	if p.life.HP <= 1 {
		t.Errorf("the potion didn't heal")
	}
	if item.Count != 1 {
		t.Errorf("%d left, want 1", item.Count)
	}
	if tap.count(smItemCooldown) != 1 {
		t.Errorf("no cooldown was sent")
	}
	s.useItem(p, item, nil)
	if item.Count != 1 {
		t.Errorf("it was used again while cooling down")
	}
}
