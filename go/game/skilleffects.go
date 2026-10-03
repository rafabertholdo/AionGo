package game

import (
	"math"
	"strconv"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// The effects skills have, by the name of their element in the skill template (AL-Game's skillengine.effect).

// noResist is the stat of an effect nothing resists in particular.
const noResist = data.Stat(255)

var effectHandlers map[string]effectHandler

func init() {
	damage := func(magical bool) effectHandler {
		return effectHandler{
			calculate: func(e *effect, t *effectTemplate) { e.calculateDamage(t, magical) },
			apply:     applyDamage,
		}
	}
	buff := effectHandler{apply: (*effect).applyAdd, start: startBuff, end: endBuff}
	// state effects: the creature is put in a state the effect's duration long, if it doesn't resist.
	state := func(resist data.Stat, mask uint32, hasResist bool, extra func(e *effect)) effectHandler {
		return effectHandler{
			calculate: func(e *effect, t *effectTemplate) {
				if !hasResist || e.resisted(resist) {
					e.addSuccess(t)
				}
			},
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				if extra != nil {
					extra(e)
				}
				e.effected.fxc().set(mask)
				startBuff(e, t)
			},
			end: func(e *effect, t *effectTemplate) {
				e.effected.fxc().unset(mask)
				endBuff(e, t)
			},
		}
	}
	freeze := func(e *effect) {
		e.s.cancelSkill(e.effected)
		e.effected.broadcast(immobilize(e.effected), true)
	}
	breakOnHit := func(e *effect) {
		fx := e.effected.fxc()
		if fx.attackedHooks == nil {
			fx.attackedHooks = map[*effect][]func(creature){}
		}
		id := e.tmpl.ID
		fx.attackedHooks[e] = append(fx.attackedHooks[e], func(creature) { fx.removeEffect(id) })
	}
	heal := func(stat, maxStat string) effectHandler {
		return effectHandler{calculate: func(e *effect, t *effectTemplate) { e.calculateHeal(t, stat); e.addSuccess(t) },
			apply: func(e *effect, t *effectTemplate) { e.applyHeal(stat) }}
	}
	effectHandlers = map[string]effectHandler{
		"return": {
			calculate: func(e *effect, t *effectTemplate) {
				if p, ok := e.effected.(*player); ok && p.spawned {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				if p, ok := e.effector.(*player); ok {
					e.s.moveToBind(p, true, 500*time.Millisecond)
				}
			},
		},
		"spellatk": damage(true),
		"skillatk": damage(false),
		"heal":     heal("hp", ""), "itemheal": heal("hp", ""),
		"healmp": heal("mp", ""), "itemhealmp": heal("mp", ""),
		"healfp": heal("fp", ""), "itemhealfp": heal("fp", ""),
		"healdp": heal("dp", ""), "itemhealdp": heal("dp", ""),
		"statup": buff, "statboost": buff, "boostheal": buff, "boosthate": buff, "boostskillcastingtime": buff,
		"statdown": {
			calculate: func(e *effect, t *effectTemplate) {
				if e.resisted(noResist) {
					e.addSuccess(t)
				}
			},
			apply: (*effect).applyAdd, start: startBuff, end: endBuff,
		},
		"snare":    state(data.SnareResistance, effectSnare, true, nil),
		"slow":     state(data.SlowResistance, effectSlow, true, nil),
		"stun":     state(data.StunResistance, effectStun, true, func(e *effect) { freeze(e) }),
		"paralyze": state(data.ParalyzeResistance, effectParalyze, true, func(e *effect) { freeze(e) }),
		"root":     state(data.RootResistance, effectRoot, true, func(e *effect) { e.effected.broadcast(immobilize(e.effected), true); breakOnHit(e) }),
		"sleep":    state(data.SleepResistance, effectSleep, true, func(e *effect) { e.s.cancelSkill(e.effected); breakOnHit(e) }),
		"silence": state(data.SilenceResistance, effectSilence, true, func(e *effect) {
			if sk := e.effected.casting(); sk != nil && sk.tmpl.Type == "MAGICAL" {
				e.s.cancelSkill(e.effected)
			}
		}),
		"fear": {
			calculate: func(e *effect, t *effectTemplate) {
				if e.resisted(data.FearResistance) {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) { e.duration /= 2; e.addToEffected() },
			start: func(e *effect, t *effectTemplate) {
				e.s.cancelSkill(e.effected)
				e.effected.fxc().set(effectFear)
				e.effected.broadcast(immobilize(e.effected), true)
				if o, ok := e.effected.(*object); ok {
					e.s.stopMoving(o)
				}
			},
			end: func(e *effect, t *effectTemplate) { e.effected.fxc().unset(effectFear) },
		},
		"blind": {
			calculate: func(e *effect, t *effectTemplate) {
				if e.resisted(data.BlindResistance) {
					e.addSuccess(t)
				}
			},
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) { e.effected.fxc().blinded = append(e.effected.fxc().blinded, e) },
			end: func(e *effect, t *effectTemplate) {
				fx := e.effected.fxc()
				fx.blinded = removeEffect(fx.blinded, e)
			},
		},
		"confuse": {calculate: func(e *effect, t *effectTemplate) {
			if e.resisted(data.ConfuseResistance) {
				e.addSuccess(t)
			}
		}},
		"curse": {calculate: func(e *effect, t *effectTemplate) {
			if e.resisted(data.CurseResistance) {
				e.addSuccess(t)
			}
		}},
		"bind": {apply: (*effect).applyAdd, start: func(e *effect, t *effectTemplate) {
			e.effected.fxc().set(effectCannotMove)
			e.effected.broadcast(immobilize(e.effected), true)
		}, end: func(e *effect, t *effectTemplate) { e.effected.fxc().unset(effectCannotMove) }},
		"dot":    dotHandler(0, noResist),
		"poison": dotHandler(effectPoison, data.PoisonResistance),
		"bleed":  dotHandler(effectBleed, data.BleedResistance),
		"hot": {
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				e.startPeriodic(t, func() {
					value := t.value(e.level)
					switch t.n.Str("type") {
					case "MP":
						e.s.increaseMP(e.effected, statusMP, value)
					default:
						e.s.increaseHP(e.effected, statusNaturalHP, value)
					}
				})
			},
		},
		"hostileup": {
			calculate: func(e *effect, t *effectTemplate) { e.taunt = t.value(e.level); e.addSuccess(t) },
			apply: func(e *effect, t *effectTemplate) {
				if o, ok := e.effected.(*object); ok {
					e.s.addHate(o, e.effector, e.taunt)
				}
			},
		},
		"dispelbuff": {apply: func(e *effect, t *effectTemplate) {
			e.effected.fxc().removeByTargetSlot("BUFF", t.n.Int("count"))
		}},
		"dispel":       {},
		"dispeldebuff": {},
		"shield": {
			calculate: func(e *effect, t *effectTemplate) {
				e.shieldTotal = t.value(e.level)
				e.shieldHit = t.n.Int("hitvalue") + t.n.Int("hitdelta")*e.level
				e.addSuccess(t)
			},
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) { e.effected.fxc().shields = append(e.effected.fxc().shields, e) },
			end: func(e *effect, t *effectTemplate) {
				fx := e.effected.fxc()
				fx.shields = removeEffect(fx.shields, e)
			},
		},
	}
	// The effects of the same kind as a buff: they change stats the way statup does.
	for _, kind := range []string{"wpnmastery", "armormastery", "weaponstatup", "weaponstatboost", "changempconsumption"} {
		if _, ok := effectHandlers[kind]; !ok {
			effectHandlers[kind] = effectHandler{}
		}
	}
}

func removeEffect(list []*effect, e *effect) []*effect {
	for i, other := range list {
		if other == e {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// applyAdd is what most effects do to apply: they go on the creature.
func (e *effect) applyAdd(*effectTemplate) { e.addToEffected() }

// startBuff is BufEffect.startEffect: the effect's stat changes are put on.
func startBuff(e *effect, t *effectTemplate) {
	var effectNode *data.SkillEffect
	for i := range e.tmpl.Effects {
		if e.tmpl.Effects[i].Node == t.n {
			effectNode = &e.tmpl.Effects[i]
		}
	}
	if effectNode == nil || len(effectNode.Changes) == 0 {
		return
	}
	if mods := effectNode.Modifiers(e.level); len(mods) > 0 {
		e.s.addStatEffect(e, statKey(e.tmpl.ID, t), mods)
	}
}

// endBuff is BufEffect.endEffect: handled by the effect, which takes off every stat change it put on.
func endBuff(e *effect, t *effectTemplate) {}

func statKey(skill int32, t *effectTemplate) string {
	return strconv.Itoa(int(skill)) + ":" + strconv.Itoa(int(t.n.Int("e"))) + ":" + strconv.Itoa(int(t.position))
}

// resisted is EffectTemplate.calculateEffectResistRate: whether the effect lands.
func (e *effect) resisted(resist data.Stat) bool {
	power := float32(1000)
	if resist != noResist {
		power -= float32(e.effected.gameStats().current(resist))
	}
	attackerLevel, targetLevel := e.effector.clevel(), e.effected.clevel()
	differ := targetLevel - attackerLevel
	switch {
	case differ > 0 && differ < 8:
		power -= float32(data.Round(power * float32(differ) / 10))
	case differ >= 8:
		power -= float32(data.Round(power * 0.80))
	}
	if o, ok := e.effected.(*object); ok {
		gauge := float32(o.npc.HPGauge)
		power -= 200 * (1 + gauge/10)
	}
	return float32(rnd(0, 999))+float32(rnd(0, 999))/1000 < power
}

// dotHandler is DamageOverTimeEffect, PoisonEffect and BleedEffect: damage every checktime, with a state for poison and bleeding.
func dotHandler(mask uint32, resist data.Stat) effectHandler {
	return effectHandler{
		calculate: func(e *effect, t *effectTemplate) {
			if e.resisted(resist) {
				e.addSuccess(t)
			}
		},
		apply: (*effect).applyAdd,
		start: func(e *effect, t *effectTemplate) {
			if mask != 0 {
				e.effected.fxc().set(mask)
			}
			e.startPeriodic(t, func() {
				value := t.value(e.level)
				d := e.s.magicDamage(e.effector, e.effected, value, t.element())
				e.s.gotHit(e.effected, e.effector, e.tmpl.ID, statusDamage, d)
			})
		},
		end: func(e *effect, t *effectTemplate) {
			if mask != 0 {
				e.effected.fxc().unset(mask)
			}
		},
	}
}

// startPeriodic is what scheduleEffectAtFixedRate does: fn runs every checktime while the effect lasts.
func (e *effect) startPeriodic(t *effectTemplate, fn func()) {
	interval := time.Duration(t.n.Int("checktime")) * time.Millisecond
	if interval <= 0 {
		return
	}
	e.periodic[t.position] = e.s.every(interval, interval, func() {
		if !e.stopped {
			fn()
		}
	})
}

// ---- damage

// calculateDamage is DamageEffect.calculate.
func (e *effect) calculateDamage(t *effectTemplate, magical bool) {
	s := e.s
	value := t.value(e.level)
	value = e.applyActionModifiers(t, value)
	if _, ok := e.effected.(*player); ok && e.tmpl.PvpDamage != 0 {
		value = data.Round(float32(value) * float32(e.tmpl.PvpDamage) / 100)
	}
	if magical {
		damage := s.magicDamage(e.effector, e.effected, value, t.element())
		status := statusNormalHit
		if e.effected.fxc().guarantees(statusResist) || chance(float64(data.Round(float32(e.effected.gameStats().current(data.MagicalResist)-e.effector.gameStats().current(data.MagicalAccuracy))/10))) {
			status = statusResist
			damage = 0
		}
		e.setResult(damage, status)
	} else {
		damage := s.physicalDamage(e.effector, e.effected, value)
		status := s.physicalStatus(e.effector, e.effected)
		switch status {
		case statusBlock:
			reduce := e.effected.gameStats().current(data.DamageReduce)
			damage -= data.Round(float32(damage*reduce) / 100)
		case statusDodge:
			damage = 0
		case statusCritical:
			damage *= 2
		case statusParry:
			damage = int32(float64(damage) * 0.5)
		}
		e.setResult(damage, status)
	}
	if e.attackStatus != statusResist && e.attackStatus != statusDodge {
		e.addSuccess(t)
	}
}

// setResult is AttackUtil.calculateEffectResult: the hit goes through the target's shields.
func (e *effect) setResult(damage int32, status int8) {
	list := []attackResult{{damage: damage, status: status}}
	e.effected.fxc().applyShields(list)
	e.r1, e.attackStatus, e.shield = list[0].damage, list[0].status, list[0].shield
}

// applyActionModifiers is EffectTemplate.applyActionModifiers: the first modifier that fits changes the value.
func (e *effect) applyActionModifiers(t *effectTemplate, value int32) int32 {
	mods := t.n.Child("modifiers")
	if mods == nil {
		return value
	}
	for _, m := range mods.Children {
		fits := false
		switch m.Name {
		case "frontdamage":
			fits = isInFrontOf(e.effector, e.effected)
		case "backdamage":
			fits = isBehind(e.effector, e.effected)
		case "stundamage":
			fits = e.effected.fxc().isSet(effectStun)
		case "stumbledamage":
			fits = e.effected.fxc().isSet(effectStumble)
		case "poisondamage":
			fits = e.effected.fxc().isSet(effectPoison)
		case "targetracedamage":
			fits = e.targetRace(m)
		}
		if fits {
			return value + m.Int("value") + e.level*m.Int("delta")
		}
	}
	return value
}

// targetRace is TargetRaceDamageModifier.check: the target's race (of an npc) is the one the modifier is for.
func (e *effect) targetRace(m *data.Node) bool {
	o, ok := e.effected.(*object)
	return ok && o.npc.Race == m.Str("race")
}

// isBehind is PositionUtil.isBehindTarget.
func isBehind(a, b creature) bool {
	_, ax, ay, _ := a.loc()
	_, bx, by, _ := b.loc()
	return angleDiffFits(angleFrom(ax, ay, bx, by), float32(b.cheading())*3)
}

// isInFrontOf is PositionUtil.isInFrontOfTarget.
func isInFrontOf(a, b creature) bool {
	_, ax, ay, _ := a.loc()
	_, bx, by, _ := b.loc()
	return angleDiffFits(float32(b.cheading())*3, angleFrom(bx, by, ax, ay))
}

func angleFrom(x1, y1, x2, y2 float32) float32 {
	angle := float32(math.Atan2(float64(y2-y1), float64(x2-x1)) * 180 / math.Pi)
	if angle < 0 {
		angle += 360
	}
	return angle
}

func angleDiffFits(a1, a2 float32) bool {
	diff := a1 - a2
	if diff <= -360+90 {
		diff += 360
	}
	if diff >= 360-90 {
		diff -= 360
	}
	return math.Abs(float64(diff)) <= 90
}

// magicDamage is StatFunctions.calculateMagicDamageToTarget.
func (s *Server) magicDamage(speller, target creature, base int32, element string) int32 {
	sgs, tgs := speller.gameStats(), target.gameStats()
	boost := sgs.current(data.BoostMagicalSkill)
	damage := data.Round(float32(base) * (float32(sgs.current(data.Knowledge))/100 + float32(boost)/1000))
	damage = s.adjustDamage(speller, target, damage)
	var resistance int32
	switch element {
	case "EARTH":
		resistance = tgs.current(data.EarthResistance)
	case "FIRE":
		resistance = tgs.current(data.FireResistance)
	case "WATER":
		resistance = tgs.current(data.WaterResistance)
	case "WIND":
		resistance = tgs.current(data.WindResistance)
	}
	damage = data.Round(float32(damage) * (1 - float32(resistance)/1000))
	if damage <= 0 {
		damage = 1
	}
	return damage
}

// applyDamage is DamageEffect.applyEffect: the target is hit.
func applyDamage(e *effect, t *effectTemplate) {
	e.s.gotHit(e.effected, e.effector, e.tmpl.ID, statusRegular, e.r1)
	e.s.attacking(e.effector, e.effected)
}

// ---- healing

// calculateHeal is AbstractHealEffect.calculate: how much the effect heals, kept in r1 as a negative damage.
func (e *effect) calculateHeal(t *effectTemplate, stat string) {
	value := t.value(e.level)
	heal := value
	if t.n.Bool("percent") {
		current, maximum := e.statNow(stat)
		possible := maximum * value / 100
		heal = min(maximum-current, possible)
	}
	boost := e.effector.gameStats().current(data.BoostHeal)/10 + 100
	e.r1 = data.Round(float32(-heal) * float32(boost) / 100)
}

func (e *effect) statNow(stat string) (current, maximum int32) {
	c := e.effected
	switch stat {
	case "mp":
		if p, ok := c.(*player); ok {
			return p.life.MP, p.stats.current(data.MaxMP)
		}
		return 0, c.gameStats().current(data.MaxMP)
	case "fp":
		if p, ok := c.(*player); ok {
			return p.life.FP, p.stats.current(data.FlyTime)
		}
	case "dp":
		if p, ok := c.(*player); ok {
			return p.dp, p.stats.current(data.MaxDP)
		}
	default:
		hp, maxHP := c.hitPoints()
		return hp, maxHP
	}
	return 0, 0
}

// applyHeal is what the heal effects do when applied.
func (e *effect) applyHeal(stat string) {
	amount := -e.r1
	switch stat {
	case "hp":
		e.s.increaseHP(e.effected, statusNaturalHP, amount)
	case "mp":
		e.s.increaseMP(e.effected, statusNaturalMP, amount)
	case "fp":
		if p, ok := e.effected.(*player); ok {
			p.life.FP = min(p.life.FP+amount, p.stats.current(data.FlyTime))
			p.conn.send(statUpdate(smFlyTime, p.life.FP, p.stats.current(data.FlyTime)))
		}
	case "dp":
		if p, ok := e.effected.(*player); ok {
			p.dp = min(p.dp+amount, p.stats.current(data.MaxDP))
			p.conn.send(s2dp(p))
		}
	}
}

func s2dp(p *player) *wire.Writer {
	w := wire.Packet(smStatupdateDp)
	w.D(p.dp)
	return w
}
