package game

import (
	"math"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// aggroInfo is what an npc holds against a creature: the damage it did and how much it hates it (AggroInfo).
type aggroInfo struct {
	id     int32
	damage int32
	hate   int32
}

// initNpc gives a spawned npc its stats, life and behaviour (SpawnEngine.spawnObject, Npc's constructor).
func (s *Server) initNpc(o *object) {
	o.stats = newNpcStats(o.npc)
	o.maxHP = o.stats.current(data.MaxHP)
	o.hp = o.maxHP
	o.aggro = map[int32]*aggroInfo{}
	o.watchers = map[int32]*player{}
	o.fx = newEffectController(o)
	o.move = mover{directionChanged: true, distance: 2}
	o.state = npcState(o.npc)
	o.ai = newNpcAI(s, o)
}

// npcState is what NpcController.onRespawn sets: the template's state, or idle.
func npcState(t *data.NpcTemplate) uint16 {
	if t.State != 0 {
		return uint16(t.State)
	}
	return stateActive | stateNpcIdle
}

// newNpcStats is NpcGameStats: the template's numbers scaled by the npc's level.
func newNpcStats(t *data.NpcTemplate) *gameStats {
	g := &gameStats{}
	level := float32(t.Level)
	pdef := float32(t.Stats.PDef)
	for _, v := range []struct {
		stat  data.Stat
		value int32
	}{
		{data.MaxHP, npcMaxHP(t)},
		{data.MaxMP, t.Stats.MaxMP},
		{data.AttackSpeed, 2000},
		{data.PhysicalDefense, data.Round(float32((pdef/level)-1)*pdef + 10*level)},
		{data.Evasion, data.Round(float32(t.Stats.Evasion)*2.3 + level*10)},
		{data.MagicalResist, t.Stats.MDef},
		{data.MainHandPower, t.Stats.Power},
		{data.MainHandAccuracy, data.Round(float32(t.Stats.Accuracy)*2.3 + level*10)},
		{data.MainHandCritical, t.Stats.Crit},
		{data.Speed, data.Round(t.Stats.RunSpeedFight * 1000)},
		{data.MagicalAccuracy, 1500},
		{data.BoostMagicalSkill, 1000},
		{data.AttackRange, 2000},
		{data.Parry, t.Stats.Parry},
		{data.Block, t.Stats.Block},
	} {
		g.initStat(v.stat, v.value)
	}
	return g
}

// ---- who is whose enemy

// aggressiveTo is Npc.isAggressiveTo: the npc attacks the creature on sight.
func (s *Server) aggressiveTo(o *object, c creature) bool {
	p, ok := c.(*player)
	if !ok {
		return false
	}
	// The quest's aerial scouting movie must not attract nearby monsters.
	if p.transformed == flyingReconnaissanceModelID {
		return false
	}
	// Player.isAggroFrom: not for npcs ten levels below it.
	if o.npc.Level+10 <= int32(p.level) {
		return false
	}
	race, _ := raceGender(p.Character)
	return s.data.Tribes.AggroIcon(race == 1, o.npc.Tribe)
}

// isEnemy is Npc.isEnemy for a creature.
func (s *Server) isEnemy(o *object, c creature) bool {
	switch other := c.(type) {
	case *player:
		t := s.data.Tribes
		return !(t.IsSupport(o.npc.Tribe, other.tribe()) || t.IsFriendly(o.npc.Tribe, other.tribe()))
	case *object:
		t := s.data.Tribes
		return t.IsAggressive(o.npc.Tribe, other.npc.Tribe) || t.IsHostile(o.npc.Tribe, other.npc.Tribe)
	}
	return false
}

// isAggressive is Npc.isAggressive: whether the npc attacks anything on sight.
func (s *Server) isAggressive(o *object) bool {
	t := s.data.Tribes
	return t.HasAggressive(o.npc.Tribe) || t.IsGuard(o.npc.Tribe) || t.HasHostile(o.npc.Tribe)
}

// isMonster is whether the npc fights back: the npcs Monster is made of.
func isMonster(o *object) bool {
	return o.npc.Type == "ATTACKABLE" || o.npc.Type == "AGGRESSIVE"
}

// hasWalkRoutes is Npc.hasWalkRoutes.
func (s *Server) hasWalkRoutes(o *object) bool {
	return o.walker > 0 || o.randomWalk > 0
}

// ---- aggro list

func (o *object) hateList() []*aggroInfo {
	list := make([]*aggroInfo, 0, len(o.aggro))
	for _, info := range o.aggro {
		list = append(list, info)
	}
	return list
}

// addDamage is AggroList.addDamage: the creature hurt the npc, which hates it for the damage.
func (s *Server) addDamage(o *object, c creature, damage int32) {
	if c == nil || !s.isEnemy(o, c) {
		return
	}
	info := o.aggroFor(c)
	info.damage += damage
	info.hate += damage
	o.ai.handleEvent(evAttacked)
}

// addHate is AggroList.addHate.
func (s *Server) addHate(o *object, c creature, hate int32) {
	if c == nil || c.cid() == o.id || !s.isEnemy(o, c) {
		return
	}
	o.aggroFor(c).hate += hate
	o.ai.handleEvent(evAttacked)
}

func (o *object) aggroFor(c creature) *aggroInfo {
	info := o.aggro[c.cid()]
	if info == nil {
		info = &aggroInfo{id: c.cid()}
		o.aggro[c.cid()] = info
	}
	return info
}

// mostHated is AggroList.getMostHated: the creature it hates most that it still sees.
func (s *Server) mostHated(o *object) creature {
	var found creature
	var most int32
	for id, info := range o.aggro {
		c := s.creatureByID(id)
		if c == nil || c.isDead() || !o.sees(c) {
			info.hate = 0
		}
		if info.hate > most && c != nil {
			found, most = c, info.hate
		}
	}
	return found
}

// sees is KnownList.knowns: the npc has the creature in its view.
func (o *object) sees(c creature) bool {
	switch other := c.(type) {
	case *player:
		_, ok := o.watchers[other.ID]
		return ok
	case *object:
		return inRange3D(o, other, visibilityDistance)
	}
	return false
}

// mostDamage is AggroList.getMostDamage: the player who did the most damage of those the npc still sees.
func (s *Server) mostDamage(o *object) *player {
	var found *player
	var most int32
	for id, info := range o.aggro {
		p := s.spawned[id]
		if p == nil || !o.sees(p) {
			continue
		}
		if info.damage > most {
			found, most = p, info.damage
		}
	}
	return found
}

// ---- life

// reduceHP is CreatureLifeStats.reduceHp for an npc.
func (s *Server) reduceNpcHP(o *object, value int32, attacker creature) {
	o.hp -= value
	if o.hp < 0 {
		o.hp = 0
	}
	if o.hp == 0 && !o.dead {
		o.dead = true
		s.npcDied(o, attacker)
	}
}

// increaseNpcHP is CreatureLifeStats.increaseHp for an npc.
func (s *Server) increaseNpcHP(o *object, kind byte, value int32) {
	if o.dead {
		return
	}
	hp := min(o.hp+value, o.maxHP)
	if hp != o.hp {
		o.hp = hp
		o.broadcast(attackStatus(o, statusRegular, 0, 0), true)
	}
}

// npcHit is NpcController.onAttack.
func (s *Server) npcHit(o *object, attacker creature, skillID int32, kind byte, damage int32) {
	if o.summonLevel != 0 {
		s.summonHit(o, attacker, skillID, kind, damage)
		return
	}
	if o.dead {
		return
	}
	if o.kisk != nil && o.hp == o.maxHP {
		s.kiskTell(o, systemMessage(msgKiskAttacked))
	}
	if p, ok := attacker.(*player); ok && p.conn != nil {
		damage = p.conn.ascensionBossDamage(o, damage)
	}
	s.cancelOnHit(o, damage)
	o.fx.attacked(attacker)
	s.addDamage(o, attacker, damage)
	// The npcs that stand by it join in, if they are near.
	for _, other := range s.npcsNear(o, 10) {
		if s.data.Tribes.IsSupport(other.npc.Tribe, o.npc.Tribe) {
			s.addHate(other, attacker, 10)
		}
	}
	s.reduceNpcHP(o, damage, attacker)
	o.broadcast(attackStatus(o, kind, skillID, damage), true)
	if p, ok := attacker.(*player); ok && p.conn != nil {
		p.conn.ascensionAttack(o)
		p.conn.javaAttack(o)
	}
}

// npcsNear lists the other npcs within r of o.
func (s *Server) npcsNear(o *object, r float32) []*object {
	var list []*object
	around := cellAt(o.worldID, o.instance, o.x, o.y)
	for dx := int32(-1); dx <= 1; dx++ {
		for dy := int32(-1); dy <= 1; dy++ {
			for _, other := range s.grid[cell{around.world, around.inst, around.x + dx, around.y + dy}] {
				if other != o && other.npc != nil && !other.dead && inRange3D(o, other, r) {
					list = append(list, other)
				}
			}
		}
	}
	return list
}

// npcAttack is NpcController.attackTarget: the npc hits its target.
func (s *Server) npcAttack(o *object, target creature) {
	if o.dead || !s.canAttackOut(o) {
		return
	}
	if target == nil || target.isDead() {
		o.ai.handleEvent(evMostHatedChanged)
		return
	}
	results := s.physicalAttack(o, target)
	var damage int32
	for _, hit := range results {
		damage += hit.damage
	}
	o.broadcast(attackPacket(o, target, o.attackCounter, 274, 0, results), true)
	s.gotHit(target, o, 0, statusRegular, damage)
	o.attackCounter++
}

func (s *Server) canAttackOut(o *object) bool {
	return !o.dead && o.cast == nil && !o.fx.isState(stateCantAttack)
}

// npcDied is NpcController.onDie.
func (s *Server) npcDied(o *object, attacker creature) {
	if o.kisk != nil {
		s.kiskDied(o, attacker)
		return
	}
	// CreatureController.onDie
	o.move.stop()
	o.cast = nil
	o.fx.removeAll()
	o.state |= stateDead
	o.decay = s.later(decayTime(o), func() { s.despawnNpc(o, false) })
	s.scheduleRespawn(o)
	var by int32
	if attacker != nil {
		by = attacker.cid()
	}
	o.broadcast(emotionPacket(o.id, emoteDie, o.state, 0, 0, by, 0, 0, 0, 0, 0, 0), true)
	s.reward(o)
	o.ai.handleEvent(evDied)
	o.targetID = 0
	o.broadcast(s.lookAt(o), true)
}

// stateDead is CreatureState.DEAD.
const stateDead = 3 << 1

// decayTime is RespawnService.scheduleDecayTask: a corpse stays for most of the respawn time, but no more than 4 minutes.
func decayTime(o *object) time.Duration {
	seconds := min(data.Round(float32(o.interval)*0.8), 240)
	return time.Duration(seconds) * time.Second
}

func (s *Server) scheduleRespawn(o *object) {
	if o.noRespawn {
		return
	}
	o.respawn = s.later(time.Duration(o.interval)*time.Second, func() { s.respawnNpc(o) })
}

// despawnNpc is NpcController.onDespawn: the npc leaves the world.
func (s *Server) despawnNpc(o *object, forced bool) {
	if forced {
		o.decay.cancel()
	}
	o.ai.handleEvent(evDespawn)
	s.removeObject(o)
}

// respawnNpc is RespawnService's task: the npc stands again where it spawned.
func (s *Server) respawnNpc(o *object) {
	o.decay.cancel()
	s.moveObject(o, o.homeX, o.homeY, o.homeZ, o.heading)
	o.dead = false
	o.state = npcState(o.npc)
	o.hp = o.maxHP
	o.aggro = map[int32]*aggroInfo{}
	o.loot = nil
	o.targetID = 0
	o.attackCounter = 0
	s.addObject(o)
	o.ai.handleEvent(evRespawned)
}

// reward is MonsterController.doReward: the player who did the most damage gets its experience and drops.
func (s *Server) reward(o *object) {
	if !isMonster(o) {
		return
	}
	p := s.mostDamage(o)
	if p == nil {
		return
	}
	if p.group != nil {
		s.groupReward(p.group, o)
		return
	}
	s.giveExp(p, s.soloExp(p, o))
	s.recordQuestKill(o, p)
	s.registerDrop(o, p)
}

// xpPercent is XPRewardEnum: how much of its experience a monster gives for its level over the player's.
func xpPercent(difference int32) int32 {
	table := map[int32]int32{-11: 0, -10: 1, -9: 10, -8: 20, -7: 30, -6: 40, -5: 50, -4: 60, -3: 90, -2: 100, -1: 100,
		0: 100, 1: 105, 2: 110, 3: 115, 4: 120}
	if difference < -11 {
		return 0
	}
	if difference > 4 {
		return 120
	}
	return table[difference]
}

// soloExp is StatFunctions.calculateSoloExperienceReward, at the regular experience rate.
func (s *Server) soloExp(p *player, o *object) int64 {
	percent := xpPercent(o.npc.Level - int32(p.level))
	return int64(math.Floor(float64(o.npc.Stats.MaxXP) * float64(percent) * 1 / 100))
}

// ---- movement

// mover is MoveController: an npc walking to a place or following its target.
type mover struct {
	task             *task
	directionChanged bool
	tx, ty, tz       float32
	follow           bool
	stopped          bool
	counter          int
	speed            float32
	distance         float32
	walking          bool
}

func (m *mover) setDirection(x, y, z float32) {
	if x != m.tx || y != m.ty || z != m.tz {
		m.directionChanged = true
	}
	m.tx, m.ty, m.tz = x, y, z
}

func (m *mover) scheduled() bool { return m.task != nil && !m.task.cancelled }

func (m *mover) stop() {
	m.walking = false
	if m.task != nil {
		m.task.cancel()
		m.task = nil
	}
}

// schedule starts moving: a step every half second.
func (s *Server) scheduleMove(o *object) {
	m := &o.move
	if m.scheduled() {
		return
	}
	if m.speed == 0 {
		m.speed = float32(o.stats.current(data.Speed) / 1000)
	}
	m.task = s.every(0, 500*time.Millisecond, func() { s.moveStep(o) })
}

// moveStep is MoveController.move.
func (s *Server) moveStep(o *object) {
	m := &o.move
	if o.dead || o.cast != nil || o.fx.isState(stateCantMove) {
		if !m.stopped {
			m.stopped = true
			s.stopMoving(o)
		}
		return
	}
	if m.follow {
		if t := s.creatureByID(o.targetID); t != nil {
			_, x, y, z := t.loc()
			m.setDirection(x, y, z)
		}
	}
	d := distance3D(o.x, o.y, o.z, m.tx, m.ty, m.tz)
	if d <= float64(m.distance) {
		if !m.stopped {
			m.stopped = true
			s.stopMoving(o)
		}
		return
	}
	m.stopped = false
	var x2, y2, z2 float32
	mod := float32(1)
	speed := float64(m.speed)
	if d < speed*0.5 {
		x2, y2, z2 = m.tx-o.x, m.ty-o.y, m.tz-o.z
	} else {
		x2 = float32(float64(m.tx-o.x) / d * speed * 0.5)
		y2 = float32(float64(m.ty-o.y) / d * speed * 0.5)
		z2 = float32(float64(m.tz-o.z) / d * speed * 0.5)
		mod = 0.5
	}
	heading := byte(int(math.Atan2(float64(y2), float64(x2)) * 180 / math.Pi / 3))
	if m.directionChanged {
		o.broadcast(movePacket(o.id, o.x, o.y, o.z, heading, moveStartKeyboard, &[3]float32{x2 / mod, y2 / mod, z2 / mod}, nil), false)
		m.directionChanged = false
	}
	m.counter++
	s.moveNpcTo(o, o.x+x2, o.y+y2, o.z+z2, heading, m.counter%5 == 0)
}

// stopMoving is CreatureController.stopMoving: the npc is where it is, and those who see it are told.
func (s *Server) stopMoving(o *object) {
	s.moveNpcTo(o, o.x, o.y, o.z, o.heading, true)
	o.broadcast(movePacket(o.id, o.x, o.y, o.z, o.heading, moveStop, nil, nil), false)
}

// emote is an npc's SM_EMOTION.
func (s *Server) emote(o *object, kind byte, target int32) *wire.Writer {
	speed := float32(o.stats.current(data.Speed)) / speedScale
	return emotionPacket(o.id, kind, o.state, speed, 0, target, 0, 0, 0, 0,
		uint16(o.stats.base(data.AttackSpeed)), uint16(o.stats.current(data.AttackSpeed)))
}
