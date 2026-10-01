package game

import (
	"math/rand/v2"
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// Gathering and crafting: AL-Game's AbstractCraftTask with its GatheringTask and CraftingTask, GatherableController,
// CraftService and the learning of recipes.
// ponytail: crafting doesn't check that the player stands at a crafting station, as static objects aren't spawned;
// gathering an object that is used up counts an aborted try, as AL-Game does.

func init() {
	handlers[cmGather] = (*conn).gather
	handlers[cmCraft] = (*conn).craft
}

// System messages of gathering and crafting.
const (
	msgGatherSuccess    = 1330078
	msgGatherFull       = 1330081
	msgGatherExp        = 1330082
	msgNoProductionExp  = 1390221
	msgCraftFull        = 1330037
	msgRecipeLearned    = 1330061
	msgRecipeKnown      = 1330060
	msgRecipeNoSkill    = 1330062
	msgRecipeLowSkill   = 1330063
	msgUseItem          = 1300423
	msgItemError        = 1300514
	interactionFirst    = time.Second
	interactionInterval = 2500 * time.Millisecond
)

// interaction is AbstractCraftTask: every two and a half seconds the player makes progress, or fails a little, until
// it has succeeded or failed enough.
type interaction struct {
	s                     *Server
	p                     *player
	task                  *task
	success, failure      int32 // what it takes
	currentSuccess        int32
	currentFailure        int32
	skillDiff             int32
	abortOnMove           bool // moving stops it, as when gathering
	critical, setCritical bool
	sendUpdate            func(in *interaction)
	valid                 func() bool // false once the player can't go on: it died, left, or lost sight of the object
	onStart, onAbort      func(in *interaction)
	onSuccess, onFailure  func(in *interaction)
	onFinish              func(in *interaction)
}

func (s *Server) newInteraction(p *player, success, failure, skillDiff int32) *interaction {
	return &interaction{s: s, p: p, success: success, failure: failure, skillDiff: skillDiff, critical: rnd(0, 99) <= 15}
}

func (in *interaction) inProgress() bool { return in.task != nil }

// start is AbstractInteractionTask.start.
func (in *interaction) start() {
	in.onStart(in)
	in.task = in.s.every(interactionFirst, interactionInterval, in.tick)
}

// tick is what the timer runs each time. AL-Game's validateParticipants only checks that the player exists;
// here it also stops one who died, teleported away or otherwise no longer sees the object, so it can't finish
// from afar.
func (in *interaction) tick() {
	if in.valid != nil && !in.valid() {
		in.abort()
		return
	}
	if in.step() {
		in.stop()
	}
}

func (in *interaction) stop() {
	in.onFinish(in)
	in.task.cancel()
	in.task = nil
}

func (in *interaction) abort() {
	in.onAbort(in)
	in.stop()
}

// step is AbstractCraftTask.onInteraction: it says whether the interaction is over.
func (in *interaction) step() bool {
	if in.currentSuccess == in.success {
		in.onSuccess(in)
		return true
	}
	if in.currentFailure == in.failure {
		in.onFailure(in)
		return true
	}
	in.analyze()
	in.sendUpdate(in)
	return false
}

func (in *interaction) analyze() {
	multi := max(0, 33-in.skillDiff*5)
	if rnd(0, 99) > multi { // Rnd.get(100) is 0 to 99
		if in.critical && rnd(0, 99) < 30 {
			in.setCritical = true
		}
		in.currentSuccess += rnd(in.success/(multi+1)/2, in.success)
	} else {
		in.currentFailure += rnd(in.failure/(multi+1)/2, in.failure)
	}
	if in.currentSuccess >= in.success {
		if in.critical {
			in.setCritical = true
		}
		in.currentSuccess = in.success
	} else if in.currentFailure >= in.failure {
		in.currentFailure = in.failure
	}
}

// ---- skill experience

// gatherSkills are the skills whose level rises with use, and the levels where a master must teach the next.
var craftSkills = []int32{30001, 30002, 30003, 40001, 40002, 40003, 40004, 40007, 40008}

// addSkillXP is SkillList.addSkillXp: false if the skill can't rise further without a master.
func (s *Server) addSkillXP(p *player, id, xp int32) bool {
	i := slices.IndexFunc(p.skills, func(k store.Skill) bool { return k.ID == id })
	if i < 0 {
		return false
	}
	level := p.skills[i].Level
	if slices.Contains(craftSkills, id) {
		if id == 30001 && level == 49 {
			return false
		}
		switch level {
		case 99, 199, 299, 399, 450, 499:
			return false
		}
		s.autolearnRecipes(p, id, level)
	}
	if p.skillXP == nil {
		p.skillXP = map[int32]int32{}
	}
	p.skillXP[id] += xp
	if float64(p.skillXP[id]) > 0.15*float64(level+30)*float64(level+30) {
		p.skillXP[id] = 0
		s.addSkill(p, id, level+1, true)
	}
	return true
}

// grantStarterSkills gives a character the gathering skills its level comes with (Collection at 1, which is Essencetapping
// from level 10, Extract Aether and Morph Substances at 10): characters made while the craft skill tree wasn't loaded
// have none, and the client greys every plant out for them. AL-Game has no such thing: its characters always got them.
func (s *Server) grantStarterSkills(p *player) {
	for _, learn := range s.data.SkillTree {
		if !learn.Autolearn || p.level < learn.MinLevel || learn.Class != "ALL" || (learn.Race != "ALL" && learn.Race != p.Race) {
			continue
		}
		id := learn.SkillID
		switch id {
		case 30001:
			if p.level >= 10 {
				id = 30002
			}
		case 30003, 40009:
		default:
			continue
		}
		if !p.hasSkill(id) {
			s.addSkill(p, id, learn.SkillLevel, false)
		}
	}
}

// ---- recipes

// learnRecipe is RecipeList.addRecipe.
func (s *Server) learnRecipe(p *player, r *data.Recipe) {
	if slices.Contains(p.recipes, r.ID) {
		return
	}
	p.recipes = append(p.recipes, r.ID)
	if err := s.skillDB.AddRecipe(p.ID, r.ID); err != nil {
		s.log.Error("saving recipe", "recipe", r.ID, "err", err)
	}
	w := wire.Packet(smLearnRecipe)
	w.D(r.ID)
	p.conn.send(w)
	p.conn.send(systemMessage(msgRecipeLearned, descriptionID(r.NameID)))
}

// deleteRecipe is RecipeList.deleteRecipe, including the 1.9 client update.
func (s *Server) deleteRecipe(p *player, id int32) {
	index := slices.Index(p.recipes, id)
	if index < 0 {
		return
	}
	if err := s.skillDB.DeleteRecipe(p.ID, id); err != nil {
		s.log.Error("deleting recipe", "recipe", id, "err", err)
		return
	}
	p.recipes = slices.Delete(p.recipes, index, index+1)
	w := wire.Packet(smRecipeDelete)
	w.D(id)
	p.conn.send(w)
}

// autolearnRecipes is RecipeList.autoLearnRecipe.
func (s *Server) autolearnRecipes(p *player, skill, level int32) {
	race := "PC_LIGHT"
	if p.Race == "ASMODIANS" {
		race = "PC_DARK"
	}
	for _, r := range s.data.AutolearnRecipes(race, skill, level) {
		s.learnRecipe(p, r)
	}
}

// canLearnRecipe is CraftLearnAction.canAct.
func (s *Server) canLearnRecipe(p *player, id int32) bool {
	r := s.data.Recipes[id]
	if r == nil {
		return false
	}
	race := int32(0)
	if p.Race == "ASMODIANS" {
		race = 1
	}
	if (r.Race == "PC_LIGHT" && race != 0) || (r.Race == "PC_DARK" && race != 1) {
		return false
	}
	if slices.Contains(p.recipes, id) {
		p.conn.send(systemMessage(msgRecipeKnown))
		return false
	}
	level := int32(-1)
	for _, k := range p.skills {
		if k.ID == r.SkillID {
			level = k.Level
		}
	}
	if level < 0 {
		name := int32(0)
		if t := s.data.Skills[r.SkillID]; t != nil {
			name = t.NameID
		}
		p.conn.send(systemMessage(msgRecipeNoSkill, descriptionID(name)))
		return false
	}
	if r.SkillPoint > level {
		p.conn.send(systemMessage(msgRecipeLowSkill))
		return false
	}
	return true
}

// ---- gathering

// gather is CM_GATHER: the player starts to gather the object it has selected, or stops.
func (c *conn) gather(r *wire.Reader) {
	action := r.D()
	p := c.player
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := p.seen[p.targetID]
	if o == nil || o.gatherable == nil {
		return
	}
	if action == 0 {
		s.startGathering(p, o)
	} else {
		s.finishGathering(p, o)
	}
}

func gatherUpdate(t *data.GatherableTemplate, m data.Material, success, failure int32, action byte) *wire.Writer {
	w := wire.Packet(smGatherUpdate)
	w.H(uint16(t.SkillLevel))
	w.C(action)
	w.D(m.ItemID)
	switch action {
	case 0:
		w.D(t.SuccessAdj)
		w.D(t.FailureAdj)
		w.D(0)
		w.D(1200)
		w.D(1330011)
		w.H(0x24)
		w.D(m.NameID)
		w.H(0)
	case 1:
		w.D(success)
		w.D(failure)
		w.D(700)
		w.D(1200)
		w.D(0)
		w.H(0)
	case 2, 6:
		w.D(t.SuccessAdj)
		w.D(failure)
		w.D(700)
		w.D(1200)
		w.D(0)
		w.H(0)
	case 5:
		w.D(0)
		w.D(0)
		w.D(700)
		w.D(1200)
		w.D(1330080)
		w.H(0)
	case 7:
		w.D(success)
		w.D(t.FailureAdj)
		w.D(0)
		w.D(1200)
		w.D(1330079)
		w.H(0x24)
		w.D(m.NameID)
		w.H(0)
	}
	return w
}

func gatherStatus(p *player, o *object, status byte) *wire.Writer {
	w := wire.Packet(smGatherStatus)
	w.D(p.ID)
	w.D(o.id)
	w.H(0)
	w.C(status)
	return w
}

// startGathering is GatherableController.onStartUse.
func (s *Server) startGathering(p *player, o *object) {
	t := o.gatherable
	level := int32(-1)
	for _, k := range p.skills {
		if k.ID == t.HarvestSkill {
			level = k.Level
		}
	}
	if level < 0 || level < t.SkillLevel || len(t.Materials) == 0 || p.dead || o.gathering != nil || p.interaction != nil {
		return
	}
	// AL-Game only asks for room when there are several materials; a full cube would swallow the single one,
	// so it is asked for always.
	if p.cubeFull() {
		p.conn.send(systemMessage(msgGatherFull))
		return
	}
	material := t.Materials[0]
	if len(t.Materials) > 1 {
		material = pickMaterial(t.Materials)
	}
	in := s.newInteraction(p, t.SuccessAdj, t.FailureAdj, level-t.SkillLevel)
	o.gathering, p.interaction = in, in
	in.abortOnMove = true
	in.valid = func() bool { return p.spawned && !p.dead && p.seen[o.id] == o }
	in.onStart = func(in *interaction) {
		p.conn.send(gatherUpdate(t, material, 0, 0, 0))
		p.broadcast(gatherStatus(p, o, 0), true)
		p.broadcast(gatherStatus(p, o, 1), true)
	}
	in.sendUpdate = func(in *interaction) {
		p.conn.send(gatherUpdate(t, material, in.currentSuccess, in.currentFailure, 1))
	}
	in.onAbort = func(in *interaction) {
		p.conn.send(gatherUpdate(t, material, 0, 0, 5))
		p.broadcast(gatherStatus(p, o, 2), false)
	}
	in.onFailure = func(in *interaction) {
		p.conn.send(gatherUpdate(t, material, in.currentSuccess, in.currentFailure, 1))
		p.conn.send(gatherUpdate(t, material, in.currentSuccess, in.currentFailure, 7))
		p.broadcast(gatherStatus(p, o, 3), true)
	}
	in.onSuccess = func(in *interaction) {
		p.conn.send(gatherUpdate(t, material, in.currentSuccess, in.currentFailure, 2))
		p.conn.send(gatherUpdate(t, material, in.currentSuccess, in.currentFailure, 6))
		p.broadcast(gatherStatus(p, o, 2), true)
		p.conn.send(systemMessage(msgGatherSuccess, descriptionID(material.NameID)))
		if !s.addItem(p, material.ItemID, 1) {
			p.conn.send(systemMessage(msgGatherFull)) // the cube filled up meanwhile
		}
		s.rewardGatherer(p, t)
	}
	in.onFinish = func(in *interaction) {
		p.interaction = nil
		o.gathering = nil
		o.gatherCount++
		if o.gatherCount == t.HarvestCount {
			s.despawnGatherable(o)
		}
	}
	in.start()
}

// pickMaterial is the choice GatherableController makes among a gatherable's materials.
func pickMaterial(materials []data.Material) data.Material {
	sorted := slices.Clone(materials)
	slices.SortFunc(sorted, func(a, b data.Material) int { return int(a.Rate) - int(b.Rate) })
	var total float32
	for _, m := range materials {
		total += float32(m.Rate)
	}
	for _, m := range sorted {
		if rand.Float32()*100 < 100*float32(m.Rate)/total {
			return m
		}
	}
	return materials[0]
}

// rewardGatherer is GatherableController.rewardPlayer: gathering teaches the skill, and gives experience.
func (s *Server) rewardGatherer(p *player, t *data.GatherableTemplate) {
	xp := int32((0.008*float64(t.SkillLevel+100)*float64(t.SkillLevel+100) + 60))
	if s.addSkillXP(p, t.HarvestSkill, xp) {
		p.conn.send(systemMessage(msgGatherExp))
		s.giveExp(p, int64(xp))
		return
	}
	name := int32(0)
	if sk := s.data.Skills[t.HarvestSkill]; sk != nil {
		name = sk.NameID
	}
	p.conn.send(systemMessage(msgNoProductionExp, descriptionID(name)))
}

// finishGathering is GatherableController.finishGathering: the player stops.
func (s *Server) finishGathering(p *player, o *object) {
	if in := o.gathering; in != nil && in.p == p && in.inProgress() {
		in.abort()
	}
}

// despawnGatherable is GatherableController.onDie: the object is gone until it respawns.
func (s *Server) despawnGatherable(o *object) {
	// Gatherables keep no watchers list: whoever has it in view is told it is gone.
	for _, p := range s.spawned {
		if p.seen[o.id] == o {
			s.forgetObject(p, o, deleteLeaving)
		}
	}
	s.removeFromGrid(o)
	o.respawn = s.later(time.Duration(o.interval)*time.Second, func() { s.respawnGatherable(o) })
}

// respawnGatherable is RespawnService's task for a gatherable: it is whole again, and those near see it.
// ponytail: it comes back at its own spot; AL-Game rotates pooled groups through their spots (one gatherable group).
func (s *Server) respawnGatherable(o *object) {
	o.gatherCount = 0
	s.addObject(o)
}

// ---- crafting

// craft is CM_CRAFT.
func (c *conn) craft(r *wire.Reader) {
	r.C()
	r.D()
	recipe, target := r.D(), r.D()
	p := c.player
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.spawned {
		s.startCrafting(p, recipe, target)
	}
}

func craftUpdate(skill int32, item *data.ItemTemplate, success, failure int32, action byte) *wire.Writer {
	w := wire.Packet(smCraftUpdate)
	w.H(uint16(skill))
	w.C(action)
	w.D(item.ID)
	switch action {
	case 0:
		w.D(success)
		w.D(failure)
		w.D(0)
		w.D(1200)
		w.D(1330048)
		w.H(0x24)
		w.D(item.NameID)
		w.H(0)
	case 1, 2, 5, 6:
		w.D(success)
		w.D(failure)
		w.D(700)
		w.D(1200)
		w.D(0)
		w.H(0)
	case 7:
		w.D(success)
		w.D(failure)
		w.D(0)
		w.D(1200)
		w.D(1330050)
		w.H(0x24)
		w.D(item.NameID)
		w.H(0)
	}
	return w
}

func craftAnimation(p *player, target int32, skill uint16, action byte) *wire.Writer {
	w := wire.Packet(smCraftAnimation)
	w.D(p.ID)
	w.D(target)
	w.H(skill)
	w.C(action)
	return w
}

// startCrafting is CraftService.startCrafting.
func (s *Server) startCrafting(p *player, recipeID, target int32) {
	if p.interaction != nil || p.dead {
		return
	}
	r := s.data.Recipes[recipeID]
	if r == nil || !slices.Contains(p.recipes, recipeID) || (r.DP != 0 && p.dp < r.DP) {
		return
	}
	if p.cubeFull() {
		p.conn.send(systemMessage(msgCraftFull))
		return
	}
	for _, c := range r.Components {
		if s.countItems(p, c.ItemID) < int64(c.Quantity) {
			return
		}
	}
	product := s.data.Items[r.ProductID]
	if product == nil {
		return
	}
	crit := product
	if combo := s.data.Items[r.ComboProduct()]; combo != nil {
		crit = combo
	}
	p.dp -= r.DP
	for _, c := range r.Components {
		s.decreaseItemsByID(p, c.ItemID, int64(c.Quantity))
	}
	level := int32(0)
	for _, k := range p.skills {
		if k.ID == r.SkillID {
			level = k.Level
		}
	}
	in := s.newInteraction(p, 100, 100, level-r.SkillPoint)
	in.valid = func() bool { return p.spawned && !p.dead }
	p.interaction = in
	skill := r.SkillID
	in.onStart = func(in *interaction) {
		p.conn.send(craftUpdate(skill, product, 100, 100, 0))
		p.conn.send(craftUpdate(skill, product, 0, 0, 1))
		p.broadcast(craftAnimation(p, target, uint16(skill), 0), true)
		p.broadcast(craftAnimation(p, target, uint16(skill), 1), true)
	}
	in.sendUpdate = func(in *interaction) {
		action := byte(1)
		if in.setCritical {
			action = 2
		}
		p.conn.send(craftUpdate(skill, product, in.currentSuccess, in.currentFailure, action))
	}
	in.onAbort = func(in *interaction) {
		p.conn.send(craftUpdate(skill, product, 0, 0, 4))
		p.broadcast(craftAnimation(p, target, 0, 2), true)
	}
	in.onFailure = func(in *interaction) {
		p.conn.send(craftUpdate(skill, product, in.currentSuccess, in.currentFailure, 6))
		p.broadcast(craftAnimation(p, target, 0, 3), true)
	}
	in.onSuccess = func(in *interaction) {
		shown := product
		if in.setCritical {
			shown = crit
		}
		p.conn.send(craftUpdate(skill, shown, in.currentSuccess, in.currentFailure, 5))
		p.broadcast(craftAnimation(p, target, 0, 2), true)
		s.finishCrafting(p, r, in.critical)
	}
	in.onFinish = func(in *interaction) { p.interaction = nil }
	in.start()
}

// finishCrafting is CraftService.finishCrafting.
func (s *Server) finishCrafting(p *player, r *data.Recipe, critical bool) {
	product := r.ProductID
	if combo := r.ComboProduct(); critical && combo != 0 {
		product = combo
	}
	if product == 0 {
		return
	}
	xp := int32(0.008*float64(r.SkillPoint+100)*float64(r.SkillPoint+100) + 60)
	s.addItem(p, product, int64(r.Quantity))
	if s.addSkillXP(p, r.SkillID, xp) {
		s.giveExp(p, int64(xp))
	} else {
		p.conn.send(systemMessage(msgNoProductionExp, descriptionID(r.SkillID)))
	}
}

// countItems is Storage.getItemCountByItemId for the cube.
func (s *Server) countItems(p *player, id int32) int64 {
	var n int64
	for _, item := range p.cube {
		if item.ItemID == id {
			n += item.Count
		}
	}
	return n
}

// decreaseItemsByID is ItemService.decreaseItemCountByItemId.
func (s *Server) decreaseItemsByID(p *player, id int32, count int64) {
	for _, item := range slices.Clone(p.cube) {
		if count == 0 {
			return
		}
		if item.ItemID == id {
			count = s.decreaseItemCount(p, item, count)
		}
	}
}
