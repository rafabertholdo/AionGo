package game

import (
	"math"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

func init() {
	handlers[cmCastspell] = (*conn).castSpell
}

// A skill being used, as AL-Game's Skill: who uses it, on what, and how far along it is.
type skill struct {
	s        *Server
	tmpl     *data.SkillTemplate
	effector creature
	level    int32
	first    creature   // the first target, the one the rest of the targets are found from
	targets  []creature // everyone it affects
	// targetType is 0 for a creature and 1 for a point, which x, y and z are.
	targetType byte
	x, y, z    float32
	rangeCheck bool
	firstRange int32
	duration   int32
	chain      bool
	moves      int32 // how many times the effector had set out when it began
	mpChange   int32
	item       *data.ItemTemplate // the item it is used from, if any
	cast       *task
	cancelled  bool
}

// System messages of skills.
const (
	msgAttackTooFar  = 1300546 // STR_ATTACK_TOO_FAR_FROM_TARGET
	msgInvalidTarget = 1300010 // INVALID_TARGET
)

// castSpell is CM_CASTSPELL: the player uses a skill on its target or a point.
func (c *conn) castSpell(r *wire.Reader) {
	p := c.player
	id := int32(r.H())
	r.C()
	targetType := r.C()
	var x, y, z float32
	switch targetType {
	case 0:
		r.D()
	case 1:
		x, y, z = r.F(), r.F(), r.F()
	}
	r.H()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.visualState == visualBlinking {
		s.stopProtection(p)
	}
	if p.dead {
		return
	}
	tmpl := s.data.Skills[id]
	if tmpl == nil || tmpl.Passive() || !p.hasSkill(id) {
		return
	}
	s.playerUseSkill(p, tmpl, targetType, x, y, z)
}

// playerUseSkill is PlayerController.useSkill.
func (s *Server) playerUseSkill(p *player, tmpl *data.SkillTemplate, targetType byte, x, y, z float32) {
	level := int32(1)
	for _, known := range p.skills {
		if known.ID == tmpl.ID {
			level = known.Level
		}
	}
	sk := &skill{s: s, tmpl: tmpl, effector: p, level: level, first: s.creatureByID(p.targetID)}
	sk.targetType, sk.x, sk.y, sk.z = targetType, x, y, z
	if !s.canUseSkill(p, sk) {
		return
	}
	sk.use()
}

// canUseSkill is PlayerRestrictions.canUseSkill.
func (s *Server) canUseSkill(p *player, sk *skill) bool {
	if s.restrictedInPrison(p, "use skills") {
		return false
	}
	if p.cast != nil {
		return false
	}
	if !p.canAttack() && sk.tmpl.ID != 1968 {
		return false
	}
	if sk.tmpl.Type == "MAGICAL" && p.fx.isSet(effectSilence) {
		return false
	}
	if sk.tmpl.Type == "PHYSICAL" && p.fx.isSet(effectBlockade) {
		return false
	}
	return !p.skillDisabled(sk.tmpl.ID)
}

// skillDisabled is Creature.isSkillDisabled: the skill is cooling down.
func (p *player) skillDisabled(id int32) bool {
	until, ok := p.cooldowns[id]
	if !ok {
		return false
	}
	if until.Before(time.Now()) {
		delete(p.cooldowns, id)
		return false
	}
	return true
}

func (p *player) setCooldown(id int32, until time.Time) {
	if p.cooldowns == nil {
		p.cooldowns = map[int32]time.Time{}
	}
	p.cooldowns[id] = until
}

// canUse is Skill.canUseSkill: whether the skill's conditions hold, and who it will affect.
func (sk *skill) canUse() bool {
	s := sk.s
	sk.rangeCheck = true
	if !sk.setProperties(sk.tmpl.InitProperties) || !sk.check(sk.tmpl.StartConditions) || !sk.setProperties(sk.tmpl.SetProperties) {
		return false
	}
	sk.effector.setCasting(sk)
	kept := sk.targets[:0]
	for _, target := range sk.targets {
		if target == nil {
			target = sk.effector
		}
		if p, ok := sk.effector.(*player); ok {
			if !s.canAffectBySkill(p, target, sk) && sk.tmpl.ID != 1968 {
				continue
			}
		} else if sk.effector.fxc().isState(stateCantAttack) && sk.tmpl.ID != 1968 {
			continue
		}
		kept = append(kept, target)
	}
	sk.targets = kept
	sk.effector.setCasting(nil)
	return !(sk.targetType == 0 && len(sk.targets) == 0)
}

// canAffectBySkill is PlayerRestrictions.canAffectBySkill.
func (s *Server) canAffectBySkill(p *player, target creature, sk *skill) bool {
	if target.isDead() && !skillHasEffect(sk.tmpl, "resurrect") {
		p.conn.send(systemMessage(msgInvalidTarget))
		return false
	}
	if skillHasEffect(sk.tmpl, "itemhealfp") && p.state&stateFlying == 0 {
		p.conn.send(systemMessage(1300538))
		return false
	}
	if p.fx.isState(stateCantAttack) && sk.tmpl.ID != 1968 {
		return false
	}
	return p.state&statePrivateShop == 0
}

// statePrivateShop is CreatureState.PRIVATE_SHOP.
const statePrivateShop = 5 << 1

func skillHasEffect(t *data.SkillTemplate, kind string) bool {
	for _, e := range t.Effects {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// use is Skill.useSkill.
func (sk *skill) use() {
	if !sk.canUse() {
		return
	}
	s := sk.s
	sk.mpChange = 0
	sk.effector.setCasting(sk)
	sk.moves = sk.effector.moveCount()
	sk.duration = sk.tmpl.Duration
	boost := sk.effector.gameStats().current(data.BoostCastingTime)
	sk.duration += data.Round(float32(sk.tmpl.Duration) * float32(100-boost) / 100)
	if sk.tmpl.Cooldown != 0 {
		if p, ok := sk.effector.(*player); ok {
			p.setCooldown(sk.tmpl.ID, time.Now().Add(time.Duration(sk.tmpl.Cooldown*100+sk.duration)*time.Millisecond))
		}
	}
	sk.duration = max(sk.duration, 0)
	if sk.tmpl.IsActive() || sk.tmpl.IsToggle() {
		sk.startCast()
	}
	if sk.duration > 0 {
		sk.cast = s.later(time.Duration(sk.duration)*time.Millisecond, sk.endCast)
	} else {
		sk.endCast()
	}
}

func (sk *skill) targetID() int32 {
	if sk.first != nil {
		return sk.first.cid()
	}
	return 0
}

// startCast is Skill.startCast: everyone who sees the effector is told it begins to cast.
func (sk *skill) startCast() {
	sk.effector.broadcast(sk.castStart(), true)
}

// castStart is SM_CASTSPELL.
func (sk *skill) castStart() *wire.Writer {
	w := wire.Packet(smCastspell)
	w.D(sk.effector.cid())
	w.H(uint16(sk.tmpl.ID))
	w.C(byte(sk.level))
	w.C(sk.targetType)
	switch sk.targetType {
	case 0:
		w.D(sk.targetID())
	case 1:
		w.F(sk.x)
		w.F(sk.y)
		w.F(sk.z)
	}
	w.H(uint16(sk.duration))
	w.D(0)
	return w
}

// endCast is Skill.endCast: the cast is done, so the skill takes effect.
func (sk *skill) endCast() {
	s := sk.s
	if sk.cancelled || sk.effector.casting() != sk {
		return
	}
	if !sk.checkEndCast() {
		if p, ok := sk.effector.(*player); ok {
			s.cancelSkill(p)
			p.conn.send(systemMessage(msgAttackTooFar))
		}
		return
	}
	sk.effector.setCasting(nil)
	if !sk.check(sk.tmpl.UseConditions) {
		return
	}
	var status int32
	var effects []*effect
	if len(sk.tmpl.Effects) > 0 {
		for _, target := range sk.targets {
			e := s.newEffect(sk.effector, target, sk.tmpl, sk.level, 0)
			e.initialize()
			status = e.spellStatus
			effects = append(effects, e)
		}
	}
	sk.chain = true
	if prob := sk.tmpl.ChainProb; prob != 0 {
		sk.chain = rnd(0, 99) < prob
	}
	if sk.tmpl.IsActive() || sk.tmpl.IsToggle() {
		sk.effector.broadcast(sk.castEnd(status, effects), true)
	}
	for _, action := range sk.tmpl.Actions {
		sk.act(action)
	}
	for _, e := range effects {
		e.apply()
	}
	if penalty := sk.tmpl.PenaltySkill; penalty != 0 {
		if t := s.data.Skills[penalty]; t != nil {
			(&skill{s: s, tmpl: t, effector: sk.effector, level: 1, first: sk.first}).use()
		}
	}
}

// checkEndCast is Skill.checkEndCast: the target is still near enough.
func (sk *skill) checkEndCast() bool {
	if !sk.rangeCheck || sk.first == nil || sk.first == sk.effector {
		return true
	}
	return inRange3D(sk.effector, sk.first, float32(sk.firstRange)+4+2)
}

// castEnd is SM_CASTSPELL_END.
func (sk *skill) castEnd(status int32, effects []*effect) *wire.Writer {
	w := wire.Packet(smCastspellEnd)
	w.D(sk.effector.cid())
	w.C(sk.targetType)
	switch sk.targetType {
	case 0:
		w.D(sk.targetID())
	case 1:
		w.F(sk.x)
		w.F(sk.y)
		w.F(sk.z + 0.4)
	}
	w.H(uint16(sk.tmpl.ID))
	w.C(byte(sk.level))
	w.D(sk.tmpl.Cooldown)
	w.H(560)
	w.C(0)
	if sk.chain {
		w.H(32)
	} else {
		w.H(0)
	}
	w.C(0)
	w.H(uint16(len(effects)))
	for _, e := range effects {
		w.D(e.effected.cid())
		w.C(0)
		target := sk.first
		if target == nil {
			target = sk.effector
		}
		thp, tmax := target.hitPoints()
		ahp, amax := sk.effector.hitPoints()
		w.C(byte(100 * int64(thp) / int64(max(tmax, 1))))
		w.C(byte(100 * int64(ahp) / int64(max(amax, 1))))
		w.C(byte(status))
		switch status {
		case 1, 2, 4, 8:
			_, x, y, z := target.loc()
			w.F(x)
			w.F(y)
			w.F(z + 0.4)
		case 16:
			w.C(target.cheading())
		}
		w.C(16)
		w.C(0)
		w.C(1)
		w.C(0)
		w.D(e.r1)
		w.C(byte(e.attackStatus))
		w.C(e.shield)
		if e.shield == 1 {
			w.B(make([]byte, 20))
		}
	}
	return w
}

// act is one of the skill's actions, done when it takes effect.
func (sk *skill) act(action *data.Node) {
	s := sk.s
	switch action.Name {
	case "mpuse":
		value := action.Int("value") + action.Int("delta")*sk.level
		if percent := sk.mpChange; percent != 0 {
			value += value / (100 / percent)
		}
		s.reduceMP(sk.effector, value)
	case "hpuse":
		s.reduceHP(sk.effector, action.Int("value")+action.Int("delta")*sk.level, nil)
	case "dpuse":
		if p, ok := sk.effector.(*player); ok {
			s.useDP(p, action.Int("value"))
		}
	case "itemuse":
		if p, ok := sk.effector.(*player); ok {
			s.removeItemsByID(p, action.Int("itemid"), int64(action.Int("count")))
		}
	}
}

// check is whether every condition holds (Skill.checkConditions).
func (sk *skill) check(conditions data.Nodes) bool {
	for _, c := range conditions {
		if !sk.verify(c) {
			return false
		}
	}
	return true
}

func (sk *skill) verify(c *data.Node) bool {
	switch c.Name {
	case "mp":
		hp, _ := sk.effector.hitPoints()
		_ = hp
		return sk.effector.mana() > c.Int("value")+c.Int("delta")*sk.level
	case "hp":
		hp, _ := sk.effector.hitPoints()
		return hp > c.Int("value")+c.Int("delta")*sk.level
	case "dp":
		p, ok := sk.effector.(*player)
		return ok && p.dp >= c.Int("value")
	case "playermove":
		moved := sk.effector.moveCount() != sk.moves
		return c.Bool("allow") == moved
	case "arrowcheck":
		return true
	case "target":
		if c.Str("value") != "NONE" && sk.first == nil {
			return false
		}
		switch c.Str("value") {
		case "NPC":
			_, ok := sk.first.(*object)
			return ok
		case "PC":
			_, ok := sk.first.(*player)
			return ok
		}
		return false
	}
	return true
}

// setProperties is Skill.setProperties.
func (sk *skill) setProperties(properties data.Nodes) bool {
	for _, p := range properties {
		if !sk.setProperty(p) {
			return false
		}
	}
	return true
}

func (sk *skill) setProperty(p *data.Node) bool {
	s := sk.s
	switch p.Name {
	case "firsttarget":
		switch p.Str("value") {
		case "ME", "PASSIVE":
			sk.first = sk.effector
		case "TARGETORME":
			sk.first = s.targetOrMe(sk)
		case "TARGET":
			if sk.first == nil {
				return false
			}
		case "MYPET":
			return false
		case "POINT":
			sk.rangeCheck = false
			return true
		}
		if sk.first != nil {
			sk.targets = append(sk.targets, sk.first)
		}
	case "firsttargetrange":
		if !sk.rangeCheck {
			return true
		}
		if sk.first == nil {
			return false
		}
		sk.firstRange = p.Int("value")
		if inRange3D(sk.effector, sk.first, float32(p.Int("value")+4)) {
			return true
		}
		if pl, ok := sk.effector.(*player); ok {
			pl.conn.send(systemMessage(msgAttackTooFar))
		}
		return false
	case "targetrange":
		s.targetRange(sk, p)
	case "targetrelation":
		s.targetRelation(sk, p.Str("value"))
	}
	return true
}

// targetOrMe is the TARGETORME first target: the target if it is a friend, else the effector.
func (s *Server) targetOrMe(sk *skill) creature {
	if sk.first == nil {
		return sk.effector
	}
	if s.isEnemyOf(sk.effector, sk.first) {
		return sk.effector
	}
	return sk.first
}

// targetRange is TargetRangeProperty: who besides the first target the skill reaches.
func (s *Server) targetRange(sk *skill, p *data.Node) {
	if p.Str("value") != "AREA" || sk.first == nil {
		return
	}
	distance := float32(p.Int("distance") + 4)
	count := int32(0)
	for _, c := range s.creaturesNear(sk.first, distance) {
		if count >= p.Int("maxcount") {
			break
		}
		if c == sk.first {
			continue
		}
		sk.targets = append(sk.targets, c)
		count++
	}
}

// creaturesNear lists the creatures within r of c: those it would have in its known list.
func (s *Server) creaturesNear(c creature, r float32) []creature {
	var list []creature
	world, x, y, _ := c.loc()
	around := cellAt(world, instanceOf(c), x, y)
	for dx := int32(-1); dx <= 1; dx++ {
		for dy := int32(-1); dy <= 1; dy++ {
			k := cell{around.world, around.inst, around.x + dx, around.y + dy}
			for _, o := range s.grid[k] {
				if o.npc != nil && !o.dead && inRange3D(c, o, r) {
					list = append(list, o)
				}
			}
			for _, p := range s.pcells[k] {
				if inRange3D(c, p, r) {
					list = append(list, p)
				}
			}
		}
	}
	return list
}

// targetRelation is TargetRelationProperty: only enemies, or only friends.
func (s *Server) targetRelation(sk *skill, relation string) {
	switch relation {
	case "ENEMY":
		kept := sk.targets[:0]
		for _, t := range sk.targets {
			if s.isEnemyOf(sk.effector, t) {
				kept = append(kept, t)
			}
		}
		sk.targets = kept
	case "FRIEND":
		kept := sk.targets[:0]
		for _, t := range sk.targets {
			if !s.isEnemyOf(sk.effector, t) {
				kept = append(kept, t)
			}
		}
		sk.targets = kept
		if len(kept) == 0 {
			sk.first = sk.effector
			sk.targets = append(sk.targets, sk.effector)
		} else {
			sk.first = kept[0]
		}
	}
}

// isEnemyOf is Creature.isEnemy: whether a creature treats another as an enemy.
func (s *Server) isEnemyOf(a, b creature) bool {
	switch x := a.(type) {
	case *player:
		switch y := b.(type) {
		case *object:
			return isMonster(y) || s.aggressiveTo(y, x)
		case *player:
			return s.isEnemyPlayer(x, y)
		}
	case *object:
		if x.owner != nil {
			return s.isEnemyOf(x.owner, b)
		}
		return s.isEnemy(x, b)
	}
	return false
}

// cancelSkill is CreatureController.cancelCurrentSkill: what the creature is casting stops.
func (s *Server) cancelSkill(c creature) {
	sk := c.casting()
	if sk == nil {
		return
	}
	sk.cancelled = true
	sk.cast.cancel()
	if p, ok := c.(*player); ok {
		delete(p.cooldowns, sk.tmpl.ID)
	}
	c.setCasting(nil)
	w := wire.Packet(smSkillCancel)
	w.D(c.cid())
	w.H(uint16(sk.tmpl.ID))
	c.broadcast(w, true)
}

// cancelOnHit is what CreatureController.onAttack does to a cast: damage may interrupt it.
func (s *Server) cancelOnHit(c creature, damage int32) {
	sk := c.casting()
	if sk == nil || sk.tmpl.CancelRate <= 0 {
		return
	}
	concentration := c.gameStats().current(data.Concentration) / 10
	_, maxHP := c.hitPoints()
	cancel := float32(sk.tmpl.CancelRate-concentration) + float32(damage)/float32(max(maxHP, 1))*50
	if float32(rnd(0, 99)) < cancel {
		s.cancelSkill(c)
	}
}

var _ = math.Pi
