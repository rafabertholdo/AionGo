package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Summons: a creature a spirit master's skill calls, that the player commands from its panel (AL-Game's Summon,
// SummonController and the summon packets). It is an npc of the world whose master is the player.
// ponytail: a summon in guard or rest mode doesn't heal itself, and it can be told to attack but doesn't pick its own targets.

func init() {
	handlers[cmSummonCommand] = (*conn).summonCommand
	handlers[cmSummonAttack] = (*conn).summonAttack
	handlers[cmSummonMove] = (*conn).summonMove
	handlers[cmSummonEmotion] = (*conn).summonEmotion
	handlers[cmSummonCastspell] = (*conn).summonCastspell
}

// SummonMode ids, and how a summon is let go (UnsummonType).
const (
	summonAttack  = 0
	summonGuard   = 1
	summonRest    = 2
	summonRelease = 3

	unsummonCommand  = 0
	unsummonDistance = 1
	unsummonLogout   = 2
	unsummonOther    = 3

	msgSummonUnsummon    = 1200011 // its name is a description of nameId*2+1
	msgSummonDismissed   = 1200006
	msgSummonTooFar      = 1300073
	msgSummonRest        = 1200010
	msgSummonGuard       = 1200009
	msgSummonAttackMode  = 1200008
	summonReleaseDelay   = 5 * time.Second
	summonDistanceChecks = 2 * time.Second
)

// createSummon is PlayerController.createSummon: the player gets a summon of the npc, one level above the npc's for each
// level of the skill after the first.
func (s *Server) createSummon(p *player, npcID, skillLevel int32) {
	t := s.data.Npcs[npcID]
	if t == nil {
		return
	}
	level := t.Level + skillLevel - 1
	stats := s.data.SummonStatsFor(npcID, level)
	if stats == nil && isSiegeWeapon(npcID) {
		// The siege quest summons have NPC templates, but no summon-stats rows.
		stats = &data.SummonStats{MaxHP: npcMaxHP(t), MaxMP: t.Stats.MaxMP,
			PDefense: t.Stats.PDef, MResist: t.Stats.MDef,
			MainHandAttack: t.Stats.Power, RunSpeed: t.Stats.RunSpeedFight}
	}
	if stats == nil {
		return
	}
	if p.summon != nil {
		s.releaseSummon(p.summon, unsummonOther)
	}
	o := s.newOwned(p, npcID, 2, byte(p.Heading))
	if o == nil {
		return
	}
	o.summonLevel, o.summonMode = level, summonGuard
	g := &gameStats{}
	for _, v := range []struct {
		stat  data.Stat
		value int32
	}{
		{data.MaxHP, stats.MaxHP}, {data.MaxMP, stats.MaxMP}, {data.MainHandPower, stats.MainHandAttack},
		{data.PhysicalDefense, stats.PDefense}, {data.MagicalResist, stats.MResist}, {data.AttackSpeed, 2000},
		{data.Speed, data.Round(stats.RunSpeed * 1000)}, {data.RegenHp, level + 3}, {data.Knowledge, 100},
		{data.BoostCastingTime, 100},
	} {
		g.initStat(v.stat, v.value)
	}
	o.stats, o.maxHP, o.hp = g, stats.MaxHP, stats.MaxHP
	p.summon = o
	// It enters the world once, with its stats: a delete and re-add makes the client fade it out.
	s.addObject(o)
	p.conn.send(s.summonPanel(o))
	o.broadcast(s.emote(o, emoteStartEmote2, 0), true)
	o.broadcast(s.summonUpdate(o), true)
	o.timers = append(o.timers, s.every(summonDistanceChecks, summonDistanceChecks, func() {
		if o.dead || p.summon != o {
			return
		}
		if !p.spawned || p.WorldID != o.worldID || p.instance != o.instance || !inRange3D(p, o, visibilityDistance) {
			s.releaseSummon(o, unsummonDistance)
		}
	}))
}

func isSiegeWeapon(id int32) bool { return id == 201054 || id == 201055 }

func isNochsanaSiegeTarget(summon *object, target creature) bool {
	gate, ok := target.(*object)
	return ok && summon.npc != nil && summon.owner != nil && isSiegeWeapon(summon.npc.ID) &&
		summon.worldID == nochsanaWorld && gate.worldID == nochsanaWorld &&
		summon.instance == gate.instance && summon.owner.WorldID == gate.worldID &&
		summon.owner.instance == gate.instance && gate.npc != nil && gate.npc.ID == 256694 && !gate.dead
}

// The 1.9 client sends CM_SUMMON_COMMAND for the siege pet's Attack button,
// but does not drive its movement or attacks with follow-up packets. Pursue
// only this gate; ordinary summons retain their existing client control.
func (s *Server) startNochsanaSiege(o *object, target creature) bool {
	if !isNochsanaSiegeTarget(o, target) || !inRange3D(o, target, 50) {
		return false
	}
	o.targetID = target.cid()
	if o.siegeTask == nil || o.siegeTask.cancelled {
		o.siegeTask = s.every(0, time.Second, func() { s.nochsanaSiegeStep(o) })
		o.timers = append(o.timers, o.siegeTask)
	}
	return true
}

func (s *Server) stopNochsanaSiege(o *object) {
	o.siegeTask.cancel()
	o.siegeTask = nil
	o.targetID = 0
	if o.move.scheduled() {
		s.stopMoving(o)
	}
	o.move.stop()
	s.cancelSkill(o)
}

func (s *Server) nochsanaSiegeStep(o *object) {
	target := s.creatureByID(o.targetID)
	if o.dead || o.summonMode != summonAttack || o.owner == nil || !o.owner.spawned ||
		!isNochsanaSiegeTarget(o, target) {
		s.stopNochsanaSiege(o)
		if !o.dead && o.summonMode == summonAttack {
			o.summonMode = summonGuard
			if o.owner != nil && o.owner.conn != nil {
				o.owner.conn.send(s.summonUpdate(o))
			}
		}
		return
	}
	if !inRange3D(o, target, 6) {
		o.move.follow = true
		o.move.distance = 5
		o.move.speed = 2
		if !o.move.scheduled() {
			s.scheduleMove(o)
		}
		return
	}
	if o.move.scheduled() {
		s.stopMoving(o)
		o.move.stop()
	}
	if o.cast != nil {
		return
	}
	skillID := int32(18008)
	if o.npc.ID == 201055 {
		skillID = 17814
	}
	if tmpl := s.data.Skills[skillID]; tmpl != nil {
		(&skill{s: s, tmpl: tmpl, effector: o, level: 1, first: target}).use()
	}
}

func (s *Server) summonPanel(o *object) *wire.Writer {
	g := o.stats
	w := wire.Packet(smSummonPanel)
	w.D(o.id)
	w.H(uint16(o.summonLevel))
	w.D(0)
	w.D(0)
	w.D(o.hp)
	w.D(g.current(data.MaxHP))
	w.D(g.current(data.MainHandPower))
	w.H(uint16(g.current(data.PhysicalDefense)))
	w.H(uint16(g.current(data.MagicalResist)))
	w.D(0)
	w.H(0)
	return w
}

func (s *Server) summonUpdate(o *object) *wire.Writer {
	g := o.stats
	w := wire.Packet(smSummonUpdate)
	w.C(byte(o.summonLevel))
	w.H(uint16(o.summonMode))
	w.D(0)
	w.D(0)
	w.D(o.hp)
	w.D(g.current(data.MaxHP))
	w.D(g.current(data.MainHandPower))
	w.H(uint16(g.current(data.PhysicalDefense)))
	w.H(uint16(g.current(data.MagicalResist)))
	for _, stat := range []data.Stat{data.MainHandAccuracy, data.CriticalResist, data.BoostMagicalSkill, data.MagicalAccuracy,
		data.MagicalCritical, data.Parry, data.Evasion} {
		w.H(uint16(g.current(stat)))
	}
	w.D(g.base(data.MaxHP))
	w.D(g.base(data.MainHandPower))
	w.H(uint16(g.base(data.PhysicalDefense)))
	w.H(uint16(g.base(data.MagicalResist)))
	for _, stat := range []data.Stat{data.MainHandAccuracy, data.CriticalResist, data.BoostMagicalSkill, data.MagicalAccuracy,
		data.MagicalCritical, data.Parry, data.Evasion} {
		w.H(uint16(g.base(stat)))
	}
	return w
}

// releaseSummon is SummonController.release: the summon is let go, and gone five seconds later.
func (s *Server) releaseSummon(o *object, how int) {
	if o.summonMode == summonRelease {
		return
	}
	if o.siegeTask != nil {
		s.stopNochsanaSiege(o)
	}
	o.summonMode = summonRelease
	p := o.owner
	nameID := o.npc.NameID*2 + 1
	switch how {
	case unsummonCommand:
		p.conn.send(systemMessage(msgSummonUnsummon, descriptionID(nameID)))
		p.conn.send(s.summonUpdate(o))
	case unsummonDistance:
		p.conn.send(systemMessage(msgSummonTooFar))
		p.conn.send(s.summonUpdate(o))
	}
	finish := func() {
		for _, t := range o.timers {
			t.cancel()
		}
		o.timers = nil
		if p.summon == o {
			p.summon = nil
		}
		o.dead = true
		s.removeObject(o)
		delete(s.byID, o.id)
		s.ids.release(o.id)
		if how != unsummonLogout {
			p.conn.send(systemMessage(msgSummonDismissed, descriptionID(nameID)))
			w := wire.Packet(smSummonOwnerRemove)
			w.D(o.id)
			p.conn.send(w)
			panel := wire.Packet(smSummonPanelRemove)
			panel.D(0)
			p.conn.send(panel)
		}
	}
	if how == unsummonLogout {
		finish()
		return
	}
	o.timers = append(o.timers, s.later(summonReleaseDelay, finish))
}

// summonHit is SummonController.onAttack.
func (s *Server) summonHit(o *object, attacker creature, skillID int32, kind byte, damage int32) {
	if o.dead || o.summonMode == summonRelease {
		return
	}
	o.fx.attacked(attacker)
	damage = min(damage, o.hp)
	o.hp -= damage
	o.broadcast(attackStatus(o, kind, skillID, damage), true)
	o.owner.conn.send(s.summonUpdate(o))
	if o.hp <= 0 {
		o.broadcast(s.emote(o, emoteDie, 0), true)
		s.releaseSummon(o, unsummonOther)
	}
}

// summonCommand is CM_SUMMON_COMMAND: the mode of the summon, or to let it go.
func (c *conn) summonCommand(r *wire.Reader) {
	mode := r.C()
	r.D()
	r.D()
	target := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := p.summon
		if o == nil || o.summonMode == summonRelease {
			return
		}
		nameID := descriptionID(o.npc.NameID*2 + 1)
		switch mode {
		case summonAttack:
			creature := s.creatureByID(target)
			if creature == nil || isSiegeWeapon(o.npc.ID) && !s.startNochsanaSiege(o, creature) {
				return
			}
			o.summonMode = summonAttack
			p.conn.send(systemMessage(msgSummonAttackMode, nameID))
		case summonGuard:
			if o.siegeTask != nil {
				s.stopNochsanaSiege(o)
			}
			o.summonMode = summonGuard
			p.conn.send(systemMessage(msgSummonGuard, nameID))
		case summonRest:
			if o.siegeTask != nil {
				s.stopNochsanaSiege(o)
			}
			o.summonMode = summonRest
			p.conn.send(systemMessage(msgSummonRest, nameID))
		case summonRelease:
			s.releaseSummon(o, unsummonCommand)
			return
		default:
			return
		}
		p.conn.send(s.summonUpdate(o))
	})
}

// summonAttack is CM_SUMMON_ATTACK and SummonController.attackTarget.
func (c *conn) summonAttack(r *wire.Reader) {
	r.D()
	target := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := p.summon
		t := s.creatureByID(target)
		if o == nil || t == nil || t.isDead() || o.dead || o.summonMode == summonRelease || !p.canAttack() || !s.isEnemyOf(o, t) {
			return
		}
		if isNochsanaSiegeTarget(o, t) && !inRange3D(o, t, 6) {
			return
		}
		if !s.canAttackOut(o) {
			return
		}
		speed := time.Duration(o.stats.current(data.AttackSpeed)) * time.Millisecond
		if now := time.Now(); now.Sub(o.lastAttack) < speed {
			return
		} else {
			o.lastAttack = now
		}
		o.fx.attacking(t)
		results := s.physicalAttack(o, t)
		var damage int32
		for _, hit := range results {
			damage += hit.damage
		}
		o.broadcast(attackPacket(o, t, o.attackCounter, 274, 0, results), true)
		s.gotHit(t, o, 0, statusRegular, damage)
		o.attackCounter++
	})
}

// summonMove is CM_SUMMON_MOVE: where the client says the summon is.
func (c *conn) summonMove(r *wire.Reader) {
	r.D()
	x, y, z := r.F(), r.F(), r.F()
	heading, kind := r.C(), r.C()
	var to [3]float32
	if kind == moveStartMouse || kind == moveStartKeyboard {
		to = [3]float32{r.F(), r.F(), r.F()}
	}
	if r.Err != nil || !finitePosition(x, y, z) || !finitePosition(to[0], to[1], to[2]) {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := p.summon
		if o == nil || o.dead {
			return
		}
		switch kind {
		case moveStartMouse, moveStartKeyboard:
			s.moveNpcTo(o, x, y, z, heading, true)
			o.broadcast(movePacket(o.id, x, y, z, heading, kind, &to, nil), true)
		case moveValidateMouse, moveValidateKeyboard:
			start := byte(moveStartKeyboard)
			if kind == moveValidateMouse {
				start = moveStartMouse
			}
			o.broadcast(movePacket(o.id, x, y, z, heading, start, &to, nil), true)
		case moveStop:
			o.broadcast(movePacket(o.id, x, y, z, heading, kind, nil, nil), true)
			s.moveNpcTo(o, x, y, z, heading, true)
		}
	})
}

// summonEmotion is CM_SUMMON_EMOTION: the summon flies, lands, or draws and puts away its weapon.
func (c *conn) summonEmotion(r *wire.Reader) {
	r.D()
	kind := r.C()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := p.summon
		if o == nil {
			return
		}
		switch kind {
		case emoteFly, emoteLand:
		case emoteAttackMode:
			o.state |= stateWeapon
		case emoteNeutralMode:
			o.state &^= stateWeapon
		default:
			return
		}
		o.broadcast(s.emote(o, kind, 0), true)
	})
}

// summonCastspell is CM_SUMMON_CASTSPELL and SummonController.useSkill.
func (c *conn) summonCastspell(r *wire.Reader) {
	r.D()
	id := int32(r.H())
	r.C()
	target := r.D()
	r.F()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := p.summon
		t := s.creatureByID(target)
		tmpl := s.data.Skills[id]
		if o == nil || t == nil || tmpl == nil || o.cast != nil {
			return
		}
		(&skill{s: s, tmpl: tmpl, effector: o, level: 1, first: t}).use()
	})
}
