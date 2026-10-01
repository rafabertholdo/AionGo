package game

import (
	"encoding/binary"
	"math"
	"slices"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// finish steps an interaction until it is over, as the timer would.
func finish(t *testing.T, in *interaction) {
	t.Helper()
	for range 200 {
		if in.step() {
			in.stop()
			return
		}
	}
	t.Fatal("the interaction never ended")
}

// TestGathering has a player gather a plant, which teaches its skill and is used up after its harvest count.
func TestGathering(t *testing.T) {
	d := staticDataOrSkip(t)
	var template *data.GatherableTemplate
	for _, id := range slices.Sorted(mapKeys(d.Gatherables)) {
		if g := d.Gatherables[id]; g.HarvestSkill == 30002 && len(g.Materials) == 1 && d.Items[g.Materials[0].ItemID] != nil && g.HarvestCount > 0 {
			template = g
			break
		}
	}
	if template == nil {
		t.Skip("no plant in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	p.skills = append(p.skills, store.Skill{ID: 30002, Level: template.SkillLevel})
	s.spawn(p)
	o := &object{id: 0x40000, worldID: p.WorldID, x: 1002, y: 1000, z: 130, gatherable: template, interval: 5}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.byID[o.id] = o
	s.addObject(o)
	p.targetID = o.id
	p.seen[o.id] = o
	s.startGathering(p, o)
	if o.gathering == nil || tap.count(smGatherUpdate) == 0 || tap.count(smGatherStatus) != 2 {
		t.Fatalf("no gathering began: %v", tap.counts)
	}
	finish(t, o.gathering)
	if o.gatherCount != 1 || o.gathering != nil || p.interaction != nil {
		t.Errorf("gathered %d times, task %v", o.gatherCount, o.gathering)
	}
}

// TestCrafting has a player craft the first recipe of a skill it has, from the components it has.
func TestCrafting(t *testing.T) {
	d := staticDataOrSkip(t)
	var recipe *data.Recipe
	for _, id := range slices.Sorted(mapKeys(d.Recipes)) {
		r := d.Recipes[id]
		if r.Race == "PC_LIGHT" && r.SkillPoint == 1 && r.DP == 0 && d.Items[r.ProductID] != nil && len(r.Components) > 0 && r.Autolearn != 0 {
			ok := true
			for _, c := range r.Components {
				ok = ok && d.Items[c.ItemID] != nil
			}
			if ok {
				recipe = r
				break
			}
		}
	}
	if recipe == nil {
		t.Skip("no recipe in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	p.skills = append(p.skills, store.Skill{ID: recipe.SkillID, Level: 1})
	s.spawn(p)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.autolearnRecipes(p, recipe.SkillID, 1)
	if !slices.Contains(p.recipes, recipe.ID) || tap.count(smLearnRecipe) == 0 {
		t.Fatalf("the recipe wasn't learned automatically")
	}
	for _, c := range recipe.Components {
		s.addItem(p, c.ItemID, int64(c.Quantity))
	}
	s.startCrafting(p, recipe.ID, 0)
	if p.interaction == nil {
		t.Fatalf("no crafting began")
	}
	finish(t, p.interaction)
	// It ended in success or failure: on success the product is in the cube.
	made := s.countItems(p, recipe.ProductID)
	if tap.count(smCraftUpdate) == 0 || (made != 0 && made != int64(recipe.Quantity)) {
		t.Errorf("made %d of item %d, %d updates", made, recipe.ProductID, tap.count(smCraftUpdate))
	}
	for _, c := range recipe.Components {
		if recipe.ProductID != c.ItemID && s.countItems(p, c.ItemID) != 0 {
			t.Errorf("the components weren't used up")
		}
	}
}

// TestRecipeAndDyeItems has a player learn a recipe from an item, and dye a robe with a scroll.
func TestRecipeAndDyeItems(t *testing.T) {
	d := staticDataOrSkip(t)
	var book, scroll, robe int32
	for _, id := range slices.Sorted(mapKeys(d.Items)) {
		it := d.Items[id]
		for _, a := range it.Actions {
			if a.Name == "craftlearn" && book == 0 {
				if r := d.Recipes[a.Int("recipeid")]; r != nil && r.Race != "PC_DARK" && r.SkillPoint <= 1 && it.Race != "ASMODIANS" {
					book = id
				}
			}
			if a.Name == "dye" && scroll == 0 && a.Str("color") != "no" {
				scroll = id
			}
		}
		if it.Dye && robe == 0 && it.IsArmor() {
			robe = id
		}
	}
	if book == 0 || scroll == 0 || robe == 0 {
		t.Skip("no recipe book, dye or robe in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	recipe := d.Recipes[d.Items[book].Actions[0].Int("recipeid")]
	p.skills = append(p.skills, store.Skill{ID: recipe.SkillID, Level: 1})
	p.level = 60
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.addItem(p, book, 1)
	s.addItem(p, scroll, 1)
	s.addItem(p, robe, 1)
	s.useItem(p, p.cubeItem(p.cube[0].UniqueID), nil)
	if !slices.Contains(p.recipes, recipe.ID) || len(p.cube) != 2 {
		t.Errorf("recipes %v, %d items", p.recipes, len(p.cube))
	}
	target := p.cube[1]
	s.useItem(p, p.cube[0], target)
	if target.Color == 0 || len(p.cube) != 1 {
		t.Errorf("color %x, %d items (%v)", target.Color, len(p.cube), tap.counts)
	}
}

// recordMessages makes the player's system message codes be collected as they are sent.
func recordMessages(p *player, tap *tapped) *[]int32 {
	var codes []int32
	p.conn.tap = func(w *wire.Writer) {
		tap.tap(w)
		if w.Data[0] == smSystemMessage {
			codes = append(codes, int32(binary.LittleEndian.Uint32(w.Data[7:])))
		}
	}
	return &codes
}

// gatherFixture is a player with the skill at level, next to a plant that needs skillLevel of it.
func gatherFixture(t *testing.T, skill, level, skillLevel int32) (*Server, *player, *tapped, *object) {
	t.Helper()
	d := staticDataOrSkip(t)
	var template *data.GatherableTemplate
	for _, id := range slices.Sorted(mapKeys(d.Gatherables)) {
		g := d.Gatherables[id]
		if g.HarvestSkill == skill && g.SkillLevel == skillLevel && len(g.Materials) == 1 && d.Items[g.Materials[0].ItemID] != nil && g.HarvestCount > 1 {
			template = g
			break
		}
	}
	if template == nil {
		t.Skip("no such plant in the data")
	}
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	if level > 0 {
		p.skills = append(p.skills, store.Skill{ID: skill, Level: level})
	}
	s.spawn(p)
	o := &object{id: 0x40000, worldID: p.WorldID, x: 1002, y: 1000, z: 130, gatherable: template, interval: 3600}
	s.visMu.Lock() // the caller unlocks
	s.byID[o.id] = o
	s.addObject(o)
	p.targetID = o.id
	return s, p, tap, o
}

// TestGatheringSeveralInARow has a player gather a plant until it is used up, which the players near are told, and
// again once it has grown back.
func TestGatheringSeveralInARow(t *testing.T) {
	s, p, tap, o := gatherFixture(t, 30002, 1, 1)
	defer s.visMu.Unlock()
	if p.seen[o.id] != o || tap.count(smGatherableInfo) != 1 {
		t.Fatalf("the player doesn't see the plant: %v", tap.counts)
	}
	for round := 1; round <= 2; round++ {
		for n := int32(1); n <= o.gatherable.HarvestCount; n++ {
			s.startGathering(p, o)
			if o.gathering == nil {
				t.Fatalf("round %d, try %d: gathering didn't begin", round, n)
			}
			finish(t, o.gathering)
			if o.gatherCount != n && n != o.gatherable.HarvestCount {
				t.Fatalf("round %d: %d harvests after %d tries", round, o.gatherCount, n)
			}
		}
		if p.seen[o.id] != nil || tap.count(smDelete) != round || len(s.grid[cellAt(o.worldID, 0, o.x, o.y)]) != 0 || o.respawn == nil {
			t.Fatalf("round %d: the used up plant is still there: %v", round, tap.counts)
		}
		s.respawnGatherable(o)
		if p.seen[o.id] != o || o.gatherCount != 0 || tap.count(smGatherableInfo) != round+1 {
			t.Fatalf("round %d: the plant didn't grow back: %v", round, tap.counts)
		}
	}
}

// TestGatheringSkillLevels has a player need the skill, at the level the plant asks.
func TestGatheringSkillLevels(t *testing.T) {
	for _, c := range []struct {
		name         string
		level        int32
		gathers      bool
		skillOfPlant int32
	}{{"no skill", 0, false, 30002}, {"too low", 4, false, 30002}, {"just enough", 5, true, 30002}, {"more", 60, true, 30002}} {
		t.Run(c.name, func(t *testing.T) {
			s, p, tap, o := gatherFixture(t, c.skillOfPlant, c.level, 5)
			defer s.visMu.Unlock()
			s.startGathering(p, o)
			if (o.gathering != nil) != c.gathers || (tap.count(smGatherUpdate) != 0) != c.gathers {
				t.Errorf("gathering %v, wanted %v", o.gathering != nil, c.gathers)
			}
			if o.gathering != nil {
				o.gathering.stop()
			}
		})
	}
}

// TestGatheringOneAtATime has a second player, or a second try, wait for the first to end.
func TestGatheringOneAtATime(t *testing.T) {
	s, p, _, o := gatherFixture(t, 30002, 1, 1)
	defer s.visMu.Unlock()
	s.startGathering(p, o)
	first := o.gathering
	s.startGathering(p, o)
	other := *p
	other.interaction = nil
	s.startGathering(&other, o)
	if o.gathering != first {
		t.Errorf("a second interaction took the plant")
	}
	s.finishGathering(&other, o)
	if !first.inProgress() {
		t.Errorf("someone else stopped the gathering")
	}
	s.finishGathering(p, o)
	if first.inProgress() || o.gathering != nil || p.interaction != nil {
		t.Errorf("the player couldn't stop")
	}
}

// TestGatheringFullCube has a player with no room be told so, instead of losing what it gathers.
func TestGatheringFullCube(t *testing.T) {
	s, p, tap, o := gatherFixture(t, 30002, 1, 1)
	defer s.visMu.Unlock()
	codes := recordMessages(p, tap)
	for len(p.cube) < p.cubeLimit() {
		p.cube = append(p.cube, &store.Item{UniqueID: int32(0x50000 + len(p.cube)), ItemID: 100000001, Count: 1})
	}
	s.startGathering(p, o)
	if o.gathering != nil || !slices.Contains(*codes, msgGatherFull) {
		t.Errorf("gathering began with a full cube (messages %v)", *codes)
	}
}

// TestGatheringStops has the player who dies, or is taken away, stop gathering by itself.
func TestGatheringStops(t *testing.T) {
	for name, stop := range map[string]func(p *player, o *object){
		"dies":     func(p *player, o *object) { p.dead = true },
		"teleport": func(p *player, o *object) { delete(p.seen, o.id) },
		"despawn":  func(p *player, o *object) { p.spawned = false },
	} {
		t.Run(name, func(t *testing.T) {
			s, p, tap, o := gatherFixture(t, 30002, 1, 1)
			defer s.visMu.Unlock()
			s.startGathering(p, o)
			in := o.gathering
			in.tick()
			if !in.inProgress() {
				t.Fatal("it stopped for nothing")
			}
			stop(p, o)
			in.tick()
			if in.inProgress() || o.gathering != nil || p.interaction != nil || tap.count(smGatherUpdate) < 2 {
				t.Errorf("it went on, or was left behind")
			}
		})
	}
}

// TestGatheringExperience has gathering teach the skill by the formula of AL-Game, up to the level where a master must.
func TestGatheringExperience(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	codes := recordMessages(p, tap)
	p.skills = []store.Skill{{ID: 30002, Level: 1}, {ID: 30001, Level: 49}, {ID: 40001, Level: 99}}
	s.spawn(p)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	template := &data.GatherableTemplate{SkillLevel: 1, HarvestSkill: 30002}
	// 0.008*101^2+60 = 141 a time; it takes more than 0.15*31^2 = 144.15 to rise.
	s.rewardGatherer(p, template)
	if p.skills[0].Level != 1 || p.skillXP[30002] != 141 || !slices.Contains(*codes, msgGatherExp) {
		t.Fatalf("first: level %d xp %d messages %v", p.skills[0].Level, p.skillXP[30002], *codes)
	}
	lists := tap.count(smSkillList)
	s.rewardGatherer(p, template)
	if p.skills[0].Level != 2 || p.skillXP[30002] != 0 || tap.count(smSkillList) != lists+1 {
		t.Errorf("second: level %d xp %d, %d skill lists", p.skills[0].Level, p.skillXP[30002], tap.count(smSkillList)-lists)
	}
	for _, id := range []int32{30001, 40001} {
		if s.addSkillXP(p, id, 1000) {
			t.Errorf("skill %d rose past where a master must teach it", id)
		}
	}
	template.HarvestSkill = 30001
	*codes = nil
	s.rewardGatherer(p, template)
	if slices.Contains(*codes, msgGatherExp) || !slices.Contains(*codes, msgNoProductionExp) || p.skills[1].Level != 49 {
		t.Errorf("at the limit: messages %v", *codes)
	}
}

// TestStartingGatherables has each new character know Collection, and find plants for it near where it starts.
func TestStartingGatherables(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, c := range []struct {
		race  string
		start data.Location
		world int32
	}{{"ELYOS", d.Initial.Elyos, 210010000}, {"ASMODIANS", d.Initial.Asmodians, 220010000}} {
		known := false
		for _, learn := range d.SkillsAt("WARRIOR", c.race, 1) {
			known = known || (learn.SkillID == 30001 && learn.Autolearn)
		}
		if !known {
			t.Errorf("%s characters don't start with Collection", c.race)
		}
		near := 0
		for _, group := range d.Spawns[c.world] {
			g := d.Gatherables[group.NpcID]
			if g == nil || g.HarvestSkill != 30001 || g.SkillLevel != 1 {
				continue
			}
			for _, spot := range group.Spots[:group.Pool] {
				if math.Hypot(float64(spot.X-c.start.X), float64(spot.Y-c.start.Y)) < 100 {
					near++
				}
			}
		}
		if near == 0 {
			t.Errorf("no plant of Collection near where %s characters start", c.race)
		}
	}
}

// TestLegacyCharacterGathers has a character made without the gathering skills get them on entering the world, and
// gather the plant nearest to where its race starts.
func TestLegacyCharacterGathers(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, c := range []struct {
		race  string
		start data.Location
	}{{"ELYOS", d.Initial.Elyos}, {"ASMODIANS", d.Initial.Asmodians}} {
		t.Run(c.race, func(t *testing.T) {
			s := testServer(d)
			p, tap := fighter(t, s, 1000)
			p.Race, p.level, p.skills = c.race, 1, nil
			p.WorldID, p.X, p.Y, p.Z = c.start.MapID, c.start.X, c.start.Y, c.start.Z
			s.grantStarterSkills(p)
			if len(p.skills) != 1 || p.skills[0].ID != 30001 || p.skills[0].Level != 1 {
				t.Fatalf("a level 1 character got %v", p.skills)
			}
			s.spawn(p)
			s.visMu.Lock()
			defer s.visMu.Unlock()
			var best *data.Spot
			var template *data.GatherableTemplate
			for _, group := range d.Spawns[c.start.MapID] {
				if g := d.Gatherables[group.NpcID]; g != nil && g.HarvestSkill == 30001 && g.SkillLevel == 1 && len(g.Materials) > 0 {
					for i := range group.Spots[:group.Pool] {
						spot := &group.Spots[i]
						if best == nil || math.Hypot(float64(spot.X-c.start.X), float64(spot.Y-c.start.Y)) < math.Hypot(float64(best.X-c.start.X), float64(best.Y-c.start.Y)) {
							best, template = spot, g
						}
					}
				}
			}
			if best == nil {
				t.Fatal("no plant for Collection")
			}
			o := &object{id: 0x40000, worldID: p.WorldID, x: best.X, y: best.Y, z: best.Z, staticID: best.StaticID, gatherable: template, interval: 230}
			s.byID[o.id] = o
			s.addObject(o)
			if p.seen[o.id] != o || tap.count(smGatherableInfo) != 1 {
				t.Fatalf("the plant at %v isn't in view of %v", best, c.start)
			}
			p.targetID = o.id
			s.startGathering(p, o)
			if o.gathering == nil {
				t.Fatal("a level 1 character couldn't gather the nearest plant")
			}
			finish(t, o.gathering)
			// Level 10 turns Collection into Essencetapping, and adds the others.
			p.level = 10
			p.skills = []store.Skill{{ID: 30001, Level: 5}}
			s.switchCraftingSkill(p)
			s.learnSkillsFor(p, 10, "WARRIOR", false)
			var ids []int32
			for _, k := range p.skills {
				ids = append(ids, k.ID)
			}
			slices.Sort(ids)
			if !slices.Equal(ids, []int32{30002, 30003, 40009}) {
				t.Errorf("a level 10 character has skills %v", ids)
			}
			p.skills = nil
			s.grantStarterSkills(p)
			ids = ids[:0]
			for _, k := range p.skills {
				ids = append(ids, k.ID)
			}
			slices.Sort(ids)
			if !slices.Equal(ids, []int32{30002, 30003, 40009}) {
				t.Errorf("a legacy level 10 character got skills %v", ids)
			}
		})
	}
}
