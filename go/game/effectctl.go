package game

import (
	"math"
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Abnormal states, EffectId: the bits of the mask a creature's client is told, and the skills check.
const (
	effectPoison      uint32 = 1
	effectBleed       uint32 = 2
	effectParalyze    uint32 = 4
	effectSleep       uint32 = 8
	effectRoot        uint32 = 16
	effectBlind       uint32 = 32
	effectDisease     uint32 = 128
	effectSilence     uint32 = 256
	effectFear        uint32 = 512
	effectCurse       uint32 = 1024
	effectStun        uint32 = 4096
	effectStumble     uint32 = 16384
	effectStagger     uint32 = 32768
	effectAerial      uint32 = 65536
	effectSnare       uint32 = 131072
	effectSlow        uint32 = 262144
	effectSpin        uint32 = 524288
	effectBlockade    uint32 = 1048576
	effectCannotMove  uint32 = 4194304
	effectShapeChange uint32 = 8388608   // cannot fly
	effectInvisible   uint32 = 536870912 // INVISIBLE_RELATED

	// CANT_ATTACK_STATE and CANT_MOVE_STATE: what stops a creature from attacking or moving.
	stateCantAttack = effectSpin | effectSleep | effectStun | effectStumble | effectStagger | effectAerial |
		effectParalyze | effectFear | effectCannotMove
	stateCantMove = effectSpin | effectRoot | effectSleep | effectStumble | effectStun | effectStagger | effectAerial |
		effectParalyze | effectCannotMove
)

// Skill target slots (SkillTargetSlot), in order.
var targetSlots = []string{"BUFF", "DEBUFF", "CHANT", "SPEC", "SPEC2", "BOOST", "NOSHOW", "NONE"}

func slotOf(name string) int32 {
	if i := slices.Index(targetSlots, name); i >= 0 {
		return int32(i)
	}
	return int32(len(targetSlots) - 1)
}

// effectController is EffectController: the effects a creature is under, and the states they put it in.
type effectController struct {
	owner     creature
	passive   map[string]*effect
	noshow    map[string]*effect
	abnormal  map[string]*effect
	abnormals uint32
	// What the effects watch for.
	attackedHooks map[*effect][]func(attacker creature)
	attackHooks   map[*effect][]func(target creature)
	shields       []*effect
	// always are the hits guaranteed by effects (AlwaysBlock, AlwaysDodge, AlwaysParry, AlwaysResist), each with how many are left.
	always  []*alwaysHit
	blinded []*effect
}

func newEffectController(owner creature) effectController {
	return effectController{owner: owner, passive: map[string]*effect{}, noshow: map[string]*effect{}, abnormal: map[string]*effect{}}
}

// isSet is EffectController.isAbnoramlSet: all of the state's bits are set.
func (c *effectController) isSet(mask uint32) bool { return c.abnormals&mask == mask }

// isState is EffectController.isAbnormalState: any of the bits is set.
func (c *effectController) isState(mask uint32) bool { return c.abnormals&mask != 0 }

func (c *effectController) set(mask uint32)   { c.abnormals |= mask }
func (c *effectController) unset(mask uint32) { c.abnormals &^= mask }

func (c *effectController) mapFor(e *effect) map[string]*effect {
	switch {
	case e.tmpl.Passive():
		return c.passive
	case e.tmpl.IsToggle():
		return c.noshow
	}
	return c.abnormal
}

// add is EffectController.addEffect: the effect takes hold, replacing a weaker one of the same stack.
func (c *effectController) add(e *effect) {
	m := c.mapFor(e)
	if old := m[e.tmpl.Stack]; old != nil {
		if old.tmpl.Level > e.tmpl.Level || (old.tmpl.Level == e.tmpl.Level && old.level > e.level) {
			return
		}
		old.end()
	}
	if e.tmpl.IsToggle() && len(m) >= 3 {
		for stack, other := range m {
			other.end()
			delete(m, stack)
			break
		}
	}
	m[e.tmpl.Stack] = e
	e.start(false)
	if !e.tmpl.Passive() {
		e.s.broadcastEffects(c.owner)
	}
}

// clear is EffectController.clearEffect.
func (c *effectController) clear(e *effect) {
	delete(c.abnormal, e.tmpl.Stack)
	e.s.broadcastEffects(c.owner)
}

// list is the effects that show as icons.
func (c *effectController) list() []*effect {
	list := make([]*effect, 0, len(c.abnormal))
	for _, e := range c.abnormal {
		list = append(list, e)
	}
	slices.SortFunc(list, func(a, b *effect) int { return int(a.tmpl.ID - b.tmpl.ID) })
	return list
}

func (c *effectController) removeEffect(skillID int32) {
	for stack, e := range c.abnormal {
		if e.tmpl.ID == skillID {
			e.end()
			delete(c.abnormal, stack)
		}
	}
}

func (c *effectController) removeByEffectID(id int32) {
	for stack, e := range c.abnormal {
		for _, t := range e.success {
			if t.n.Int("e") == id {
				e.end()
				delete(c.abnormal, stack)
				break
			}
		}
	}
}

func (c *effectController) removeByTargetSlot(slot string, count int32) {
	for stack, e := range c.abnormal {
		if count == 0 {
			break
		}
		if e.tmpl.TargetSlot == slot {
			e.end()
			delete(c.abnormal, stack)
			count--
		}
	}
}

func (c *effectController) removeBySetNumber(set int32) {
	for _, m := range []map[string]*effect{c.abnormal, c.passive, c.noshow} {
		for stack, e := range m {
			if e.tmpl.SetException == set {
				e.end()
				delete(m, stack)
			}
		}
	}
}

// removeAll is removeAllEffects: what a death or a logout takes off.
func (c *effectController) removeAll() {
	for _, m := range []map[string]*effect{c.abnormal, c.noshow} {
		for _, e := range m {
			e.end()
		}
		clear(m)
	}
}

// attacked tells the effects watching for the creature being hit.
func (c *effectController) attacked(attacker creature) {
	for e, hooks := range c.attackedHooks {
		_ = e
		for _, hook := range slices.Clone(hooks) {
			hook(attacker)
		}
	}
}

// attacking tells the effects watching for the creature hitting something.
func (c *effectController) attacking(target creature) {
	for _, hooks := range c.attackHooks {
		for _, hook := range slices.Clone(hooks) {
			hook(target)
		}
	}
}

// applyShields is ObserveController.checkShieldStatus: shields take their part of each hit.
func (c *effectController) applyShields(list []attackResult) {
	for _, shield := range slices.Clone(c.shields) {
		shield.absorb(list)
	}
}

// ---- what an effect is

// effectTemplate is one of a skill's <effects>: what it does, and where it stands in the skill.
type effectTemplate struct {
	kind     string
	n        *data.Node
	position int32
	handler  effectHandler
}

// value is the template's value for a skill level.
func (t *effectTemplate) value(level int32) int32 { return t.n.Int("value") + t.n.Int("delta")*level }

// element is the skill element the effect deals damage of.
func (t *effectTemplate) element() string { return t.n.Str("element") }

// effectHandler is what an effect kind does at each of the effect's stages; nil for nothing.
type effectHandler struct {
	calculate func(e *effect, t *effectTemplate)
	apply     func(e *effect, t *effectTemplate)
	start     func(e *effect, t *effectTemplate)
	end       func(e *effect, t *effectTemplate)
	periodic  func(e *effect, t *effectTemplate)
}

// effect is Effect: a skill's effect on a creature.
type effect struct {
	s         *Server
	tmpl      *data.SkillTemplate
	templates []*effectTemplate
	level     int32
	effector  creature
	effected  creature
	duration  int32
	endTime   time.Time

	r1, r2, r3   int32
	spellStatus  int32
	attackStatus int8
	shield       byte
	success      []*effectTemplate
	sub          *effect
	hate, taunt  int32
	task         *task
	periodic     [5]*task
	added        bool
	stopped      bool
	statKeys     []string
	shieldLeft   int32
	shieldHit    int32
	shieldTotal  int32
}

// SM_ABNORMAL_EFFECT's slot of an effect, and how much of it is left, in milliseconds.
func (e *effect) slot() int32 { return slotOf(e.tmpl.TargetSlot) }

func (e *effect) elapsed() int32 {
	left := int32(time.Until(e.endTime) / time.Millisecond)
	return max(left, 0)
}

// skillEffects is the effect templates of every skill, made once.
func (s *Server) skillEffects(tmpl *data.SkillTemplate) []*effectTemplate {
	if list, ok := s.fxTemplates[tmpl]; ok {
		return list
	}
	list := make([]*effectTemplate, 0, len(tmpl.Effects))
	for i, e := range tmpl.Effects {
		list = append(list, &effectTemplate{kind: e.Kind, n: e.Node, position: int32(i + 1), handler: effectHandlers[e.Kind]})
	}
	if s.fxTemplates == nil {
		s.fxTemplates = map[*data.SkillTemplate][]*effectTemplate{}
	}
	s.fxTemplates[tmpl] = list
	return list
}

func (s *Server) newEffect(effector, effected creature, tmpl *data.SkillTemplate, level, duration int32) *effect {
	return &effect{s: s, tmpl: tmpl, templates: s.skillEffects(tmpl), level: level, effector: effector, effected: effected,
		duration: duration, attackStatus: statusNormalHit}
}

// initialize is Effect.initialize: each template works out what it does to the target, and which of them take hold.
func (e *effect) initialize() {
	if len(e.templates) == 0 {
		return
	}
	damaging := false
	for _, t := range e.templates {
		e.calculate(t)
		if isDamageKind(t.kind) {
			damaging = true
		}
	}
	for _, t := range e.success {
		e.calculateSub(t)
		e.calculateHate(t)
	}
	switch {
	case damaging:
		if e.attackStatus == statusResist || e.attackStatus == statusDodge {
			e.success = nil
			return
		}
		if len(e.success) != len(e.templates) {
			e.success = slices.DeleteFunc(e.success, func(t *effectTemplate) bool { return !isDamageKind(t.kind) })
		}
	case len(e.success) != len(e.templates):
		e.success = nil
		if e.tmpl.Type == "MAGICAL" {
			e.attackStatus = statusResist
		} else {
			e.attackStatus = statusDodge
		}
	}
}

// isDamageKind is whether the kind is a DamageEffect that isn't over time.
func isDamageKind(kind string) bool {
	return kind == "spellatk" || kind == "skillatk" || kind == "spellatkdraininstant" || kind == "skillatkdraininstant"
}

func (e *effect) calculate(t *effectTemplate) {
	if t.handler.calculate != nil {
		t.handler.calculate(e, t)
		return
	}
	e.success = append(e.success, t)
}

func (e *effect) addSuccess(t *effectTemplate) {
	for _, other := range e.success {
		if other == t {
			return
		}
	}
	e.success = append(e.success, t)
}

// calculateSub is EffectTemplate.calculateSubEffect: a skill that comes with this one.
func (e *effect) calculateSub(t *effectTemplate) {
	sub := t.n.Child("subeffect")
	if sub == nil {
		return
	}
	tmpl := e.s.data.Skills[sub.Int("skill_id")]
	if tmpl == nil {
		return
	}
	other := e.s.newEffect(e.effector, e.effected, tmpl, tmpl.Level, tmpl.EffectsDuration())
	other.initialize()
	e.spellStatus = other.spellStatus
	e.sub = other
}

// calculateHate is EffectTemplate.calculateHate: what the effect makes those it hurts hate the effector.
func (e *effect) calculateHate(t *effectTemplate) {
	hop := t.n.Str("hoptype")
	if hop == "" || len(e.success) == 0 {
		return
	}
	current := e.hate
	switch hop {
	case "DAMAGE":
		current += e.r1
	case "SKILLLV":
		current += t.n.Int("hopb") + t.n.Int("hopa")*e.level
	}
	if current == 0 {
		current = 1
	}
	e.hate = data.Round(float32(current) * float32(e.effector.gameStats().current(data.BoostHate)) / 100)
}

// apply is Effect.applyEffect.
func (e *effect) apply() {
	if len(e.templates) == 0 || len(e.success) == 0 {
		return
	}
	for _, t := range e.success {
		if t.handler.apply != nil {
			t.handler.apply(e, t)
		}
		if e.sub != nil {
			e.sub.apply()
		}
	}
	if e.hate != 0 {
		e.s.broadcastHate(e.effector, e.hate)
	}
}

// start is Effect.startEffect: the effect begins, and ends after its duration.
func (e *effect) start(restored bool) {
	if len(e.success) == 0 {
		return
	}
	for _, t := range e.success {
		if t.handler.start != nil {
			t.handler.start(e, t)
		}
	}
	if e.tmpl.IsToggle() {
		if p, ok := e.effector.(*player); ok {
			p.conn.send(skillActivation(e.tmpl.ID, true))
		}
	}
	if !restored {
		e.duration = e.effectsDuration()
	}
	if e.duration == 0 {
		return
	}
	e.endTime = time.Now().Add(time.Duration(e.duration) * time.Millisecond)
	e.task = e.s.later(time.Duration(e.duration)*time.Millisecond, e.end)
}

func skillActivation(id int32, active bool) *wire.Writer {
	w := wire.Packet(smSkillActivation)
	w.H(uint16(id))
	w.D(0)
	w.Bool(active)
	return w
}

// effectsDuration is Effect.getEffectsDuration.
func (e *effect) effectsDuration() int32 {
	var duration int32
	for _, t := range e.success {
		d := t.n.Int("duration")
		if r := t.n.Int("randomtime"); r > 0 {
			d -= rnd(0, r-1)
		}
		duration = max(duration, d)
	}
	if _, ok := e.effected.(*player); ok && e.tmpl.PvpDuration != 0 {
		duration = duration * e.tmpl.PvpDuration / 100
	}
	return duration
}

// end is Effect.endEffect.
func (e *effect) end() {
	if e.stopped {
		return
	}
	e.stopped = true
	for _, t := range e.success {
		if t.handler.end != nil {
			t.handler.end(e, t)
		}
	}
	if e.tmpl.IsToggle() {
		if p, ok := e.effector.(*player); ok {
			p.conn.send(skillActivation(e.tmpl.ID, false))
		}
	}
	e.task.cancel()
	for _, t := range e.periodic {
		t.cancel()
	}
	for _, key := range e.statKeys {
		e.s.endStatEffect(e.effected, key)
	}
	fx := e.effected.fxc()
	delete(fx.attackedHooks, e)
	delete(fx.attackHooks, e)
	fx.clear(e)
}

// addToEffected is Effect.addToEffectedController.
func (e *effect) addToEffected() {
	if !e.added {
		e.effected.fxc().add(e)
		e.added = true
	}
}

// absorb is AttackShieldObserver.checkShield: what the shield takes of each hit of an attack.
func (e *effect) absorb(list []attackResult) {
	percent := e.success[0].n.Bool("percent")
	for i := range list {
		damage := list[i].damage
		var absorbed int32
		switch {
		case percent:
			absorbed = damage * e.shieldHit / 100
		case damage >= e.shieldHit:
			absorbed = e.shieldHit
		default:
			absorbed = damage
		}
		absorbed = min(absorbed, e.shieldTotal)
		e.shieldTotal -= absorbed
		if absorbed > 0 {
			list[i].shield = 2
		}
		list[i].damage = damage - absorbed
		if e.shieldTotal <= 0 {
			e.end()
			return
		}
	}
}

// ---- telling clients

// broadcastEffects is EffectController.broadCastEffects: everyone who sees the creature gets its icons, within 100 ms.
func (s *Server) broadcastEffects(c creature) {
	if s.fxDirty == nil {
		s.fxDirty = map[creature]bool{}
	}
	s.fxDirty[c] = true
}

// flushEffects sends the icons of the creatures whose effects changed.
func (s *Server) flushEffects() {
	for c := range s.fxDirty {
		if p, ok := c.(*player); ok {
			p.conn.send(s.abnormalStatePacket(p))
			p.broadcast(s.abnormalEffect(p), false)
			s.updateGroupOf(p, groupUpdate)
		} else {
			c.broadcast(s.abnormalEffect(c), true)
		}
		delete(s.fxDirty, c)
	}
}

// abnormalEffect is SM_ABNORMAL_EFFECT.
func (s *Server) abnormalEffect(c creature) *wire.Writer {
	fx := c.fxc()
	list := fx.list()
	w := wire.Packet(smAbnormalEffect)
	w.D(c.cid())
	w.C(1)
	w.D(0)
	w.D(int32(fx.abnormals))
	w.H(uint16(len(list)))
	for _, e := range list {
		w.H(uint16(e.tmpl.ID))
		w.C(byte(e.level))
		w.C(byte(e.slot()))
		w.D(e.elapsed())
	}
	return w
}

// abnormalState is SM_ABNORMAL_STATE: a player's own icons.
func (s *Server) abnormalStatePacket(c creature) *wire.Writer {
	fx := c.fxc()
	list := fx.list()
	w := wire.Packet(smAbnormalState)
	w.D(int32(fx.abnormals))
	w.H(uint16(len(list)))
	for _, e := range list {
		w.D(e.effector.cid())
		w.H(uint16(e.tmpl.ID))
		w.C(byte(e.level))
		w.C(byte(e.slot()))
		w.D(e.elapsed())
	}
	return w
}

// immobilize is SM_TARGET_IMMOBILIZE: the creature stands where it is.
func immobilize(c creature) *wire.Writer {
	_, x, y, z := c.loc()
	w := wire.Packet(smTargetImmobilize)
	w.D(c.cid())
	w.F(x)
	w.F(y)
	w.F(z)
	w.C(c.cheading())
	return w
}

// broadcastHate is CreatureController.broadcastHate: the npcs that hate the creature hate it more.
func (s *Server) broadcastHate(c creature, value int32) {
	for _, o := range s.creaturesNear(c, visibilityDistance) {
		if npc, ok := o.(*object); ok {
			if _, hating := npc.aggro[c.cid()]; hating {
				s.addHate(npc, c, value)
			}
		}
	}
}

// ---- stats of skills

func (s *Server) recomputeStats(c creature) {
	switch x := c.(type) {
	case *player:
		before := x.stats.current(data.Speed) + x.stats.current(data.FlySpeed) + x.stats.current(data.AttackSpeed)
		x.stats.recompute(x.weaponType(data.SlotSubHand) != "")
		x.conn.send(s.statsInfo(x))
		if before != x.stats.current(data.Speed)+x.stats.current(data.FlySpeed)+x.stats.current(data.AttackSpeed) {
			x.broadcast(s.playerEmotion(x, emoteStartEmote2, 0, 0, 0, 0, 0), true)
		}
	case *object:
		speed := x.stats.current(data.Speed)
		x.stats.recompute(false)
		if speed != x.stats.current(data.Speed) {
			x.move.speed = float32(x.stats.current(data.Speed)) / 1000
			x.broadcast(s.emote(x, emoteStartEmote2, 0), true)
		}
	}
}

// addStatEffect is CreatureGameStats.addModifiers: the modifiers a skill puts on the creature, until ended.
func (s *Server) addStatEffect(e *effect, key string, mods data.Modifiers) {
	c := e.effected
	g := c.gameStats()
	g.addKeyed(key, mods)
	e.statKeys = append(e.statKeys, key)
	if p, ok := c.(*player); ok {
		p.fxMods = append(p.fxMods, keyedMods{key, mods})
	}
	s.recomputeStats(c)
}

// endStatEffect is CreatureGameStats.endEffect.
func (s *Server) endStatEffect(c creature, key string) {
	c.gameStats().removeKeyed(key)
	if p, ok := c.(*player); ok {
		p.fxMods = slices.DeleteFunc(p.fxMods, func(k keyedMods) bool { return k.key == key })
	}
	s.recomputeStats(c)
}

// keyedMods are the modifiers a skill effect put on a player: they are put on again when its stats are rebuilt.
type keyedMods struct {
	key  string
	mods data.Modifiers
}

// ---- reducing and healing, for players and npcs alike

func (s *Server) reduceHP(c creature, value int32, attacker creature) {
	switch x := c.(type) {
	case *player:
		s.reducePlayerHP(x, value, attacker)
	case *object:
		s.reduceNpcHP(x, value, attacker)
	}
}

func (s *Server) reduceMP(c creature, value int32) {
	if p, ok := c.(*player); ok {
		s.reducePlayerMP(p, value)
	}
}

func (s *Server) increaseHP(c creature, kind byte, value int32) {
	switch x := c.(type) {
	case *player:
		s.increasePlayerHP(x, kind, value)
	case *object:
		s.increaseNpcHP(x, kind, value)
	}
}

func (s *Server) increaseMP(c creature, kind byte, value int32) {
	if p, ok := c.(*player); ok {
		s.increasePlayerMP(p, kind, value)
	}
}

// useDP is DpUseAction.
func (s *Server) useDP(p *player, value int32) {
	if p.dp <= 0 || p.dp < value {
		return
	}
	p.dp -= value
	p.conn.send(s.statsInfo(p))
}

// removeItemsByID is ItemService.decreaseItemCountByItemId.
func (s *Server) removeItemsByID(p *player, itemID int32, count int64) bool {
	if count < 1 {
		return false
	}
	for _, item := range slices.Clone(p.cube) {
		if item.ItemID != itemID {
			continue
		}
		count = s.decreaseItemCount(p, item, count)
		if count == 0 {
			break
		}
	}
	return count >= 0
}

var _ = math.Pi
