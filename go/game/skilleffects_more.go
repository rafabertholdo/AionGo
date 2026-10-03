package game

import (
	"math"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// More of the effects skills have; they register beside those of skilleffects.go.

// alwaysHit is an AttackStatusObserver: the next hits of the status that land on the creature, as many as the effect says.
type alwaysHit struct {
	status int8
	left   int32
	effect *effect
}

// guarantees is ObserveController.checkAttackStatus: whether an effect makes this hit the status, using one up.
func (c *effectController) guarantees(status int8) bool {
	for _, a := range c.always {
		if a.status == status && a.left > 0 {
			a.left--
			return true
		}
	}
	return false
}

// transformFlyingOK are the transform skills that keep the creature able to fly.
var transformFlyingOK = map[int32]bool{689: true, 690: true, 780: true, 781: true, 782: true, 789: true, 790: true, 791: true,
	9737: true, 9738: true, 9739: true, 9740: true, 9741: true, 9742: true, 9743: true, 9744: true, 9745: true, 9746: true}

// transformPacket is SM_TRANSFORM: the creature looks like the model.
func transformPacket(c creature) *wire.Writer {
	w := wire.Packet(smTransform)
	w.D(c.cid())
	var model int32
	var state uint16
	switch x := c.(type) {
	case *player:
		model, state = x.transformed, x.state
	case *object:
		model, state = x.transformed, x.state
		if model == 0 {
			model = x.npc.ID
		}
	}
	w.D(model)
	w.H(state)
	w.F(0.55)
	w.F(1.5)
	w.C(0)
	return w
}

// SpellStatus: what the client is told a skill did to its target.
const (
	spellStumble     = 1
	spellStagger     = 2
	spellOpenAerial  = 4
	spellCloseAerial = 8
	spellSpin        = 16
)

// forcedMove is SM_FORCED_MOVE: the creature is thrown to where the target is.
func forcedMove(from, target creature) *wire.Writer {
	w := wire.Packet(smForcedMove)
	w.D(from.cid())
	w.D(target.cid())
	w.C(16)
	_, x, y, z := target.loc()
	w.F(x)
	w.F(y)
	w.F(z + 0.25)
	return w
}

func setModel(c creature, model int32) {
	switch x := c.(type) {
	case *player:
		x.transformed = model
	case *object:
		x.transformed = model
	}
}

// hideVisualState is the visual state of a hide effect's value.
func hideVisualState(value int32) byte {
	switch value {
	case 1, 2, 3, 10, 13, 20:
		return byte(value)
	}
	return 0
}

// removeByTypeAndSlot is EffectController.removeEffectBySkillTypeAndTargetSlot.
func (c *effectController) removeByTypeAndSlot(kind, slot string, count int32) {
	for stack, e := range c.abnormal {
		if count == 0 {
			break
		}
		if e.tmpl.Type == kind && e.tmpl.TargetSlot == slot {
			e.end()
			delete(c.abnormal, stack)
			count--
		}
	}
}

func init() {
	always := func(status int8) effectHandler {
		return effectHandler{
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply:     (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				fx := e.effected.fxc()
				fx.always = append(fx.always, &alwaysHit{status: status, left: t.n.Int("value"), effect: e})
			},
			end: func(e *effect, t *effectTemplate) {
				fx := e.effected.fxc()
				fx.always = filterAlways(fx.always, e)
			},
		}
	}
	drain := func(magical bool) effectHandler {
		h := effectHandler{
			calculate: func(e *effect, t *effectTemplate) { e.calculateDamage(t, magical) },
			apply: func(e *effect, t *effectTemplate) {
				applyDamage(e, t)
				value := e.r1 * t.n.Int("percent") / 100
				if t.n.Str("heal_type") == "MP" {
					e.s.increaseMP(e.effector, statusNaturalMP, value)
				} else {
					e.s.increaseHP(e.effector, statusNaturalHP, value)
				}
			},
		}
		return h
	}
	dispelDebuff := func(kind string) effectHandler {
		return effectHandler{
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply: func(e *effect, t *effectTemplate) {
				e.effected.fxc().removeByTypeAndSlot(kind, "DEBUFF", t.n.Int("value"))
			},
		}
	}
	// The states that throw the creature: it stops what it casts and is moved to where the effector is.
	knock := func(resist data.Stat, mask uint32, status int32, forced bool) effectHandler {
		return effectHandler{
			calculate: func(e *effect, t *effectTemplate) {
				if e.resisted(resist) {
					e.addSuccess(t)
					e.spellStatus = status
				}
			},
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				if mask != effectSpin {
					e.s.cancelSkill(e.effected)
				}
				e.effected.fxc().set(mask)
				if forced {
					e.effected.broadcast(forcedMove(e.effector, e.effected), true)
				}
			},
			end: func(e *effect, t *effectTemplate) { e.effected.fxc().unset(mask) },
		}
	}
	more := map[string]effectHandler{
		"summon": {
			calculate: func(e *effect, t *effectTemplate) {
				if _, ok := e.effected.(*player); ok {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				if p, ok := e.effected.(*player); ok {
					e.s.createSummon(p, t.n.Int("npc_id"), e.level)
				}
			},
		},
		"summontrap": {
			calculate: func(e *effect, t *effectTemplate) {
				if _, ok := e.effected.(*player); ok {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				if p, ok := e.effector.(*player); ok {
					e.s.spawnTrap(p, t.n.Int("npc_id"), t.n.Int("skill_id"), t.n.Int("time"))
				}
			},
		},
		"summonservant": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply: func(e *effect, t *effectTemplate) {
				if p, ok := e.effector.(*player); ok {
					e.s.spawnServant(p, t.n.Int("npc_id"), t.n.Int("skill_id"))
				}
			},
		},
		"signet": {calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) }, apply: (*effect).applyAdd},
		"carvesignet": {
			calculate: func(e *effect, t *effectTemplate) { e.calculateDamage(t, false) },
			apply: func(e *effect, t *effectTemplate) {
				applyDamage(e, t)
				fx := e.effected.fxc()
				next := int32(1)
				if placed := fx.abnormal[t.n.Str("signet")]; placed != nil {
					next = placed.tmpl.ID - t.n.Int("signetid") + 2
					if next > t.n.Int("signetlvl") || next > 5 {
						return
					}
					placed.end()
				}
				tmpl := e.s.data.Skills[t.n.Int("signetid")+next-1]
				if tmpl == nil {
					return
				}
				sig := e.s.newEffect(e.effector, e.effected, tmpl, next, 0)
				sig.initialize()
				sig.apply()
			},
		},
		"signetburst": {
			calculate: func(e *effect, t *effectTemplate) {
				placed := e.effected.fxc().abnormal[t.n.Str("signet")]
				if placed == nil {
					return
				}
				e.r1 = t.value(e.level) * placed.level / 5
				e.addSuccess(t)
				placed.end()
			},
			apply: applyDamage,
		},
		"aura": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply:     (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				e.periodic[t.position] = e.s.every(time.Millisecond, 6500*time.Millisecond, func() {
					if e.stopped {
						return
					}
					p, ok := e.effector.(*player)
					if !ok {
						return
					}
					members := []*player{p}
					if p.group != nil {
						members = p.group.members
					}
					for _, m := range members {
						if m == p || inRange3D(p, m, float32(t.n.Int("distance")+4)) {
							e.s.applyAura(m, t.n.Int("skill_id"))
						}
					}
				})
			},
		},
		"resurrect": {
			calculate: func(e *effect, t *effectTemplate) {
				if p, ok := e.effected.(*player); ok && p.dead {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				caster, byPlayer := e.effector.(*player)
				if p, ok := e.effected.(*player); ok && byPlayer {
					w := wire.Packet(smResurrect)
					w.S(caster.Name)
					w.H(uint16(e.tmpl.ID))
					w.D(0)
					p.conn.send(w)
				}
			},
		},
		"rebirth":    {calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) }, apply: (*effect).applyAdd},
		"stagger":    knock(data.StaggerResistance, effectStagger, spellStagger, true),
		"stumble":    knock(data.StumbleResistance, effectStumble, spellStumble, true),
		"openaerial": knock(data.OpenareialResistance, effectAerial, spellOpenAerial, true),
		"spin":       knock(data.SpinResistance, effectSpin, spellSpin, false),
		"closeaerial": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t); e.spellStatus = spellCloseAerial },
			apply:     func(e *effect, t *effectTemplate) { e.effected.fxc().removeByEffectID(8224) },
		},
		"dash": {
			calculate: func(e *effect, t *effectTemplate) { e.calculateDamage(t, false) },
			apply:     applyDamage,
		},
		"movebehind": {
			calculate: func(e *effect, t *effectTemplate) {
				if _, ok := e.effector.(*player); ok {
					e.calculateDamage(t, false)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				applyDamage(e, t)
				p, ok := e.effector.(*player)
				if !ok {
					return
				}
				_, x, y, z := e.effected.loc()
				radian := float64(e.effected.cheading()) * 3 * math.Pi / 180 // a heading is 3 degrees
				e.s.updatePosition(p, x+float32(math.Cos(math.Pi+radian)*1.3), y+float32(math.Sin(math.Pi+radian)*1.3), z+0.25, e.effected.cheading())
				p.conn.send(playerMove(p))
			},
		},
		"pulled": {
			calculate: func(e *effect, t *effectTemplate) {
				if _, ok := e.effector.(*player); ok {
					e.addSuccess(t)
				}
			},
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				e.effected.fxc().set(effectCannotMove)
				e.s.later(time.Second, func() {
					if p, ok := e.effected.(*player); ok && !p.dead {
						_, x, y, z := e.effector.loc()
						e.s.updatePosition(p, x, y, z+0.25, byte(p.Heading))
						p.broadcast(forcedMove(e.effector, p), true)
					}
				})
			},
			end: func(e *effect, t *effectTemplate) { e.effected.fxc().unset(effectCannotMove) },
		},
		"alwaysblock": always(statusBlock), "alwaysdodge": always(statusDodge), "alwaysparry": always(statusParry),
		"alwaysresist":         always(statusResist),
		"spellatkdraininstant": drain(true), "skillatkdraininstant": drain(false),
		"dispeldebuffphysical": dispelDebuff("PHYSICAL"), "dispeldebuffmental": dispelDebuff("MAGICAL"),
		"delaydamage": {
			calculate: func(e *effect, t *effectTemplate) { e.calculateDamage(t, true) },
			apply: func(e *effect, t *effectTemplate) {
				e.s.later(time.Duration(t.n.Int("delay"))*time.Millisecond, func() {
					if !e.effected.isDead() {
						e.s.gotHit(e.effected, e.effector, e.tmpl.ID, statusRegular, e.r1)
					}
				})
			},
		},
		"switchhpmp": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply: func(e *effect, t *effectTemplate) {
				hp, _ := e.effected.hitPoints()
				mp := e.effected.mana()
				e.s.increaseHP(e.effected, statusNaturalHP, mp-hp)
				e.s.increaseMP(e.effected, statusNaturalMP, hp-mp)
			},
		},
		"reflector": {calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) }},
		"provoker": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply:     (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				provoke := func(other creature) {
					if rnd(0, 100) > t.n.Int("prob2") {
						return
					}
					target := other
					if t.n.Str("provoke_target") == "ME" {
						target = e.effector
					}
					provoked := e.s.data.Skills[t.n.Int("skill_id")]
					if provoked == nil {
						return
					}
					pe := e.s.newEffect(e.effector, target, provoked, provoked.Level, 0)
					pe.initialize()
					pe.apply()
				}
				fx := e.effected.fxc()
				switch t.n.Str("provoke_type") {
				case "ATTACK":
					if fx.attackHooks == nil {
						fx.attackHooks = map[*effect][]func(creature){}
					}
					fx.attackHooks[e] = append(fx.attackHooks[e], provoke)
				case "ATTACKED":
					if fx.attackedHooks == nil {
						fx.attackedHooks = map[*effect][]func(creature){}
					}
					fx.attackedHooks[e] = append(fx.attackedHooks[e], provoke)
				}
			},
		},
		"transform": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply: func(e *effect, t *effectTemplate) {
				// Another transformation under it ends.
				for _, other := range e.effected.fxc().list() {
					if other == e || other.tmpl.ID == e.tmpl.ID {
						continue
					}
					for _, ot := range other.templates {
						if ot.kind == "transform" {
							other.end()
							break
						}
					}
				}
				e.addToEffected()
			},
			start: func(e *effect, t *effectTemplate) {
				if !transformFlyingOK[e.tmpl.ID] {
					e.effected.fxc().set(effectShapeChange)
				}
				setModel(e.effected, t.n.Int("model"))
				e.effected.broadcast(transformPacket(e.effected), true)
			},
			end: func(e *effect, t *effectTemplate) {
				e.effected.fxc().unset(effectShapeChange)
				setModel(e.effected, 0)
				e.effected.broadcast(transformPacket(e.effected), true)
			},
		},
		"hide": {
			calculate: func(e *effect, t *effectTemplate) { e.addSuccess(t) },
			apply:     (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				startBuff(e, t)
				e.effected.fxc().set(effectInvisible)
				if p, ok := e.effected.(*player); ok {
					p.visualState = hideVisualState(t.n.Int("value"))
					p.broadcast(playerState(p), true)
				}
			},
			end: func(e *effect, t *effectTemplate) {
				e.effected.fxc().unset(effectInvisible)
				if p, ok := e.effected.(*player); ok {
					p.visualState = 0
					p.broadcast(playerState(p), true)
				}
			},
		},
	}
	for kind, h := range more {
		effectHandlers[kind] = h
	}
	for kind, h := range portedEffects() {
		effectHandlers[kind] = h
	}
}

// portedEffects are SearchEffect, ReturnPointEffect, MpUseOverTimeEffect, OneTimeBoostSkillAttackEffect,
// MagicCounterAtkEffect and PetOrderUseUltraSkillEffect.
func portedEffects() map[string]effectHandler {
	seeState := func(t *effectTemplate) byte {
		switch v := t.n.Int("value"); v {
		case 1, 2: // SEARCH1, SEARCH2
			return byte(v)
		}
		return 0
	}
	return map[string]effectHandler{
		"search": {
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				if p, ok := e.effected.(*player); ok {
					p.seeState |= seeState(t)
					p.broadcast(playerState(p), true)
				}
			},
			end: func(e *effect, t *effectTemplate) {
				if p, ok := e.effected.(*player); ok {
					p.seeState &^= seeState(t)
					p.broadcast(playerState(p), true)
				}
			},
		},
		"returnpoint": {
			calculate: func(e *effect, t *effectTemplate) {
				if e.item != nil {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				p, ok := e.effector.(*player)
				if !ok {
					return
				}
				portal := e.s.data.NamedPortal(e.item.ReturnWorld, e.item.ReturnAlias)
				if portal == nil {
					e.s.log.Warn("no return portal", "item", e.item.ID, "world", e.item.ReturnWorld, "alias", e.item.ReturnAlias)
					return
				}
				e.s.teleportTo(p, e.item.ReturnWorld, portal.Exit.X, portal.Exit.Y, portal.Exit.Z, byte(p.Heading), 500*time.Millisecond)
			},
		},
		"mpuseovertime": {
			calculate: func(e *effect, t *effectTemplate) {
				if e.effected.mana() >= e.effected.gameStats().current(data.MaxMP)*t.n.Int("value")/100 {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				required := e.effected.gameStats().current(data.MaxMP) * t.n.Int("value") / 100
				interval := time.Duration(t.n.Int("checktime")) * time.Millisecond
				if interval <= 0 {
					return
				}
				e.periodic[t.position] = e.s.every(0, interval, func() {
					if e.stopped {
						return
					}
					if e.effected.mana() < required {
						e.end()
					}
					e.s.reduceMP(e.effected, required)
				})
			},
		},
		"onetimeboostskillattack": {
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				startBuff(e, t)
				stop, count := t.n.Int("count"), int32(0)
				e.effected.fxc().onSkillUse(e, func(sk *skill) {
					if count < stop && sk.tmpl.Type == "PHYSICAL" {
						count++
					}
					if count == stop {
						e.end()
					}
				})
			},
			end: endBuff,
		},
		"magiccounteratk": {
			calculate: func(e *effect, t *effectTemplate) {
				if e.resisted(noResist) {
					e.addSuccess(t)
				}
			},
			apply: (*effect).applyAdd,
			start: func(e *effect, t *effectTemplate) {
				percent, maxDamage := t.n.Int("percent"), t.n.Int("maxdmg")
				e.effected.fxc().onSkillUse(e, func(sk *skill) {
					if sk.tmpl.Type != "MAGICAL" {
						return
					}
					_, maxHP := e.effected.hitPoints()
					if damage := maxHP / 100 * percent; damage <= maxDamage {
						e.s.gotHit(e.effected, e.effector, e.tmpl.ID, statusDamage, damage)
					} else {
						e.s.gotHit(e.effected, e.effector, 0, statusRegular, maxDamage)
					}
				})
			},
		},
		"petorderuseultraskill": {
			calculate: func(e *effect, t *effectTemplate) {
				if _, ok := e.effector.(*player); ok && e.effected != nil {
					e.addSuccess(t)
				}
			},
			apply: func(e *effect, t *effectTemplate) {
				p := e.effector.(*player)
				if p.summon == nil {
					return
				}
				w := wire.Packet(smSummonUseskill)
				w.D(p.summon.cid())
				w.H(uint16(e.s.data.PetSkills[[2]int32{e.tmpl.ID, p.summon.npc.ID}]))
				w.C(1)
				w.D(e.effected.cid())
				p.conn.send(w)
			},
		},
	}
}

func filterAlways(list []*alwaysHit, e *effect) []*alwaysHit {
	var kept []*alwaysHit
	for _, a := range list {
		if a.effect != e {
			kept = append(kept, a)
		}
	}
	return kept
}

// playerMove is SM_PLAYER_MOVE: the client is told where its player is.
func playerMove(p *player) *wire.Writer {
	w := wire.Packet(smPlayerMove)
	w.F(p.X)
	w.F(p.Y)
	w.F(p.Z)
	w.C(byte(p.Heading))
	return w
}

// applyAura is AuraEffect.applyAuraTo: the player is under the aura's skill, and those who see it are shown.
func (s *Server) applyAura(p *player, skill int32) {
	tmpl := s.data.Skills[skill]
	if tmpl == nil {
		return
	}
	e := s.newEffect(p, p, tmpl, tmpl.Level, 0)
	e.initialize()
	e.apply()
	w := wire.Packet(smMantraEffect)
	w.D(0)
	w.D(p.ID)
	w.H(uint16(skill))
	p.broadcast(w, false)
}
