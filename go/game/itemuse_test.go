package game

import (
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestUseHallowedStrikeBook(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		name, class, race string
		level             int
		known, learns     bool
	}{
		{"level 3 priest elyos", "PRIEST", "ELYOS", 3, false, true},
		{"level 3 priest asmodian", "PRIEST", "ASMODIANS", 3, false, true},
		{"cleric inherits priest book", "CLERIC", "ELYOS", 10, false, true},
		{"chanter inherits priest book", "CHANTER", "ASMODIANS", 10, false, true},
		{"below required level", "PRIEST", "ELYOS", 2, false, false},
		{"wrong class", "MAGE", "ELYOS", 3, false, false},
		{"already learned", "PRIEST", "ELYOS", 3, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testServer(d)
			p, tap := fighter(t, s, 1000)
			p.Class, p.Race, p.level = tc.class, tc.race, tc.level
			p.skills = nil
			if tc.known {
				p.skills = []store.Skill{{ID: 962, Level: 1}}
			}
			db := &restoredSkills{}
			s.skillDB = db
			book := &store.Item{UniqueID: 0x40000, ItemID: 169500354, Owner: p.ID, Count: 1}
			p.cube = []*store.Item{book}
			use := wire.Packet(cmUseItem)
			use.D(book.UniqueID)
			use.C(0)
			p.conn.useItem(wire.NewReader(use.Data[1:]))
			if tc.learns {
				if len(p.cube) != 0 || !p.hasSkill(962) || len(db.writes) != 1 || db.writes[0] != (store.Skill{ID: 962, Level: 1}) {
					t.Fatalf("book use: cube=%v skills=%v saved=%v", p.cube, p.skills, db.writes)
				}
				if tap.count(smSkillList) != 1 || tap.count(smItemUsageAnimation) != 1 || tap.count(smDeleteItem) != 1 {
					t.Fatal("missing skill, animation or inventory notification")
				}
			} else if len(p.cube) != 1 || len(db.writes) != 0 || tap.count(smSkillList) != 0 || tap.count(smItemUsageAnimation) != 0 {
				t.Fatal("ineligible book use changed inventory or skills")
			}
		})
	}
}

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
