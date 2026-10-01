package game

import (
	"math"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

func init() {
	handlers[cmAttack] = (*conn).attack
}

// Attack statuses, AttackStatus: the result of one hit, as SM_ATTACK sends it.
const (
	statusDodge         int8 = 0
	statusOffDodge      int8 = 1
	statusParry         int8 = 2
	statusOffParry      int8 = 3
	statusBlock         int8 = 4
	statusOffBlock      int8 = 5
	statusResist        int8 = 6
	statusNormalHit     int8 = 10
	statusOffNormalHit  int8 = 11
	statusCritical      int8 = -54
	statusOffCritical   int8 = -37
	statusCriticalDodge int8 = -64
)

// Kinds of SM_ATTACK_STATUS (its TYPE): what changed, and how.
const (
	statusNaturalHP byte = 3
	statusRegular   byte = 5
	statusDamage    byte = 7
	statusMP        byte = 21
	statusNaturalMP byte = 22
	statusFPRings   byte = 23
	statusFP        byte = 25
	statusNaturalFP byte = 26
)

// attackResult is one hit of an attack.
type attackResult struct {
	damage int32
	status int8
	shield byte
}

// Weapon types used by the damage formulas (WeaponType).
const (
	weaponDagger = "DAGGER_1H"
	weaponSword  = "SWORD_1H"
	weaponMace   = "MACE_1H"
	weaponSword2 = "SWORD_2H"
	weaponPole   = "POLEARM_2H"
	weaponStaff  = "STAFF_2H"
	weaponBow    = "BOW"
)

// server is the game server the player is in.
func (p *player) server() *Server { return p.conn.s }

// equipped is the item worn in a slot, as Equipment.getEquippedItemsBySlot.
func (p *player) equipped(slot int32) *data.ItemTemplate {
	for _, item := range p.equipment {
		if item.Slot == slot {
			return p.server().data.Items[item.ItemID]
		}
	}
	return nil
}

// weaponType is the type of the weapon in the slot, or "" if there is none.
func (p *player) weaponType(slot int32) string {
	if t := p.equipped(slot); t != nil && t.IsWeapon() {
		return t.WeaponType
	}
	return ""
}

func (p *player) shieldEquipped() bool {
	t := p.equipped(data.SlotSubHand)
	return t != nil && t.IsArmor() && t.ArmorType == "SHIELD"
}

// physicalAttack is AttackUtil.calculateAttackResult: the hits of a normal attack.
func (s *Server) physicalAttack(attacker, target creature) []attackResult {
	list := s.physicalAttackHits(attacker, target)
	target.fxc().applyShields(list)
	return list
}

func (s *Server) physicalAttackHits(attacker, target creature) []attackResult {
	damage := s.physicalDamage(attacker, target, 0)
	status := s.physicalStatus(attacker, target)
	stats := attacker.gameStats()
	if p, ok := attacker.(*player); ok && p.weaponType(data.SlotSubHand) != "" {
		off := statusOffNormalHit
		switch status {
		case statusBlock:
			off = statusOffBlock
		case statusDodge:
			off = statusOffDodge
		case statusCritical:
			off = statusOffCritical
		case statusParry:
			off = statusOffParry
		}
		offDamage := s.offHandDamage(p, target)
		mainHits := rnd(1, stats.current(data.MainHandHits))
		offHits := rnd(1, stats.current(data.OffHandHits))
		return append(s.splitDamage(attacker, target, mainHits, damage, status),
			s.splitDamage(attacker, target, offHits, offDamage, off)...)
	}
	return s.splitDamage(attacker, target, rnd(1, stats.current(data.MainHandHits)), damage, status)
}

// splitDamage is AttackUtil.splitPhysicalDamage: a multiple hit's first blow and its echoes.
func (s *Server) splitDamage(attacker, target creature, hits, damage int32, status int8) []attackResult {
	var list []attackResult
	for i := range hits {
		d := damage
		if i != 0 {
			d = data.Round(float32(damage) * 0.1)
		}
		switch status {
		case statusBlock, statusOffBlock:
			reduce := target.gameStats().current(data.DamageReduce)
			d -= data.Round(float32(d*reduce) / 100)
		case statusDodge, statusOffDodge:
			d = 0
		case statusCritical:
			d = weaponCritical(d, attacker.(*player).weaponType(data.SlotMainHand))
		case statusOffCritical:
			d = weaponCritical(d, attacker.(*player).weaponType(data.SlotSubHand))
		case statusParry, statusOffParry:
			d = int32(float64(d) * 0.5)
		}
		list = append(list, attackResult{damage: d, status: status})
	}
	return list
}

// weaponCritical is AttackUtil.calculateWeaponCritical.
func weaponCritical(damage int32, weapon string) int32 {
	switch weapon {
	case weaponDagger:
		return data.Round(float32(damage) * 2.3)
	case weaponSword:
		return data.Round(float32(damage) * 2.2)
	case weaponMace:
		return damage * 2
	case weaponSword2, weaponPole, weaponStaff, weaponBow:
		return data.Round(float32(damage) * 1.8)
	}
	return data.Round(float32(damage) * 1.5)
}

// npcRankMultiplier is StatFunctions.calculateRankMultipler.
func npcRankMultiplier(rank string) int32 {
	switch rank {
	case "JUNK", "NORMAL":
		return 2
	case "ELITE":
		return 3
	case "HERO":
		return 4
	case "LEGENDARY":
		return 5
	}
	return 1
}

// physicalDamage is StatFunctions.calculatePhysicDamageToTarget.
func (s *Server) physicalDamage(attacker, target creature, skillDamage int32) int32 {
	ags, tgs := attacker.gameStats(), target.gameStats()
	var result int32
	switch a := attacker.(type) {
	case *player:
		totalMin, totalMax := ags.current(data.MinDamages), ags.current(data.MaxDamages)
		average := data.Round(float32(totalMin+totalMax) / 2)
		mainHandAttack := ags.base(data.MainHandPower)
		if a.weaponType(data.SlotMainHand) != "" {
			if average < 1 {
				average = 1
			}
			min := data.Round(float32(((mainHandAttack*100)/average)*totalMin) / 100)
			max := data.Round(float32(((mainHandAttack*100)/average)*totalMax) / 100)
			base := rnd(min, max)
			result = data.Round(float32(base)*(float32(ags.current(data.Power))*0.01+float32(float32(ags.base(data.MainHandPower))*0.2)*0.01) +
				float32(ags.bonus(data.MainHandPower)) + float32(skillDamage))
		} else {
			base := rnd(16, 20)
			result = data.Round(float32(base) * (float32(ags.current(data.Power)) * 0.01))
		}
		result = s.adjustDamage(attacker, target, result)
	case *object:
		multiplier := float64(npcRankMultiplier(a.npc.Rank))
		hpGaugeMod := float64(1 + a.npc.HPGauge/10)
		base := ags.current(data.MainHandPower)
		max := int32(float64(base)*multiplier*hpGaugeMod + float64(base*a.npc.Level/10))
		min := max - base
		result += rnd(min, max)
	}
	result -= data.Round(float32(tgs.current(data.PhysicalDefense)) * 0.10)
	if result <= 0 {
		result = 1
	}
	return result
}

// offHandDamage is StatFunctions.calculateOffHandPhysicDamageToTarget.
func (s *Server) offHandDamage(p *player, target creature) int32 {
	ags, tgs := p.stats, target.gameStats()
	totalMin, totalMax := ags.current(data.MinDamages), ags.current(data.MaxDamages)
	average := max(data.Round(float32(totalMin+totalMax)/2), 1)
	offHandAttack := ags.base(data.OffHandPower)
	min := data.Round(float32(((offHandAttack*100)/average)*totalMin) / 100)
	max := data.Round(float32(((offHandAttack*100)/average)*totalMax) / 100)
	base := rnd(min, max)
	damage := data.Round(float32(base)*(float32(ags.current(data.Power))*0.01+float32(float32(ags.base(data.OffHandPower))*0.2)*0.01) +
		float32(ags.bonus(data.OffHandPower)))
	damage = s.adjustDamage(p, target, damage)
	damage -= data.Round(float32(tgs.current(data.PhysicalDefense)) * 0.10)
	for i := float32(0.25); i <= 1; i += 0.25 {
		if rnd(0, 100) < 50 {
			damage = int32(float32(damage) * i)
			break
		}
	}
	if damage <= 0 {
		damage = 1
	}
	return damage
}

// adjustDamage is StatFunctions.adjustDamages: a player hits monsters above its level weaker, and other players softer.
func (s *Server) adjustDamage(attacker, target creature, damage int32) int32 {
	_, attackerIsPlayer := attacker.(*player)
	_, targetIsPlayer := target.(*player)
	switch {
	case attackerIsPlayer && !targetIsPlayer:
		if differ := target.clevel() - attacker.clevel(); differ > 2 {
			if differ < 10 {
				damage -= data.Round(float32(damage) * (float32(differ) - 2) / 10)
			} else {
				damage -= data.Round(float32(damage) * 0.80)
			}
		}
	case attackerIsPlayer && targetIsPlayer:
		damage = data.Round(float32(damage) * 0.60)
		attackBonus := float32(attacker.gameStats().current(data.PvpAttackRatio)) * 0.001
		defenceBonus := float32(target.gameStats().current(data.PvpDefendRatio)) * 0.001
		damage = data.Round(float32(damage) + float32(damage)*attackBonus - float32(damage)*defenceBonus)
	}
	return damage
}

// accuracy is the attacker's main hand accuracy, the average of both hands with a weapon in each.
func accuracy(attacker creature) int32 {
	stats := attacker.gameStats()
	if p, ok := attacker.(*player); ok && p.weaponType(data.SlotSubHand) != "" {
		return data.Round(float32(stats.current(data.MainHandAccuracy)+stats.current(data.OffHandAccuracy)) / 2)
	}
	return stats.current(data.MainHandAccuracy)
}

// avoidRate is the chance, in percent, of a hit avoided by the stat, at most cap and at least 1.
func avoidRate(stat, accuracy, cap int32) float64 {
	rate := (stat - accuracy) / 10
	if rate > cap {
		rate = cap
	}
	if rate <= 0 {
		return 1
	}
	return float64(rate)
}

// criticalRate is StatFunctions.calculatePhysicalCriticalRate.
func criticalRate(attacker, target creature) float64 {
	stats := attacker.gameStats()
	var critical int32
	if p, ok := attacker.(*player); ok && p.weaponType(data.SlotSubHand) != "" {
		critical = data.Round(float32((stats.current(data.MainHandCritical)+stats.current(data.OffHandCritical))/2 -
			target.gameStats().current(data.CriticalResist)))
	} else {
		critical = stats.current(data.MainHandCritical) - target.gameStats().current(data.CriticalResist)
	}
	var rate float64
	switch {
	case critical <= 440:
		rate = float64(float32(critical) * 0.1)
	case critical <= 600:
		rate = float64(float32(440*0.1) + float32(critical-440)*0.05)
	default:
		rate = float64(float32(440*0.1) + float32(160*0.05) + float32(critical-600)*0.02)
	}
	return math.Max(rate, 1)
}

// physicalStatus is AttackUtil.calculatePhysicalStatus: whether a blow is dodged, parried, blocked or critical.
func (s *Server) physicalStatus(attacker, target creature) int8 {
	// A blind attacker misses (its observer answers true whatever the dice say).
	if len(attacker.fxc().blinded) > 0 {
		return statusDodge
	}
	acc := accuracy(attacker)
	tgs := target.gameStats()
	if target.fxc().guarantees(statusDodge) || chance(avoidRate(tgs.current(data.Evasion), acc, 30)) {
		return statusDodge
	}
	if p, ok := target.(*player); ok {
		if target.fxc().guarantees(statusParry) || p.weaponType(data.SlotMainHand) != "" && chance(avoidRate(tgs.current(data.Parry), acc, 40)) {
			return statusParry
		}
		if target.fxc().guarantees(statusBlock) || p.shieldEquipped() && chance(avoidRate(tgs.current(data.Block), acc, 50)) {
			return statusBlock
		}
	}
	if p, ok := attacker.(*player); ok && p.weaponType(data.SlotMainHand) != "" && chance(criticalRate(attacker, target)) {
		return statusCritical
	}
	return statusNormalHit
}

// attackPacket is SM_ATTACK: an attack and each of its hits.
func attackPacket(attacker, target creature, counter int32, when int32, kind byte, list []attackResult) *wire.Writer {
	ahp, amax := attacker.hitPoints()
	thp, tmax := target.hitPoints()
	w := wire.Packet(smAttack)
	w.D(attacker.cid())
	w.C(byte(counter))
	w.H(uint16(when))
	w.C(kind)
	w.D(target.cid())
	w.C(byte(100 * thp / max(tmax, 1)))
	w.C(byte(100 * ahp / max(amax, 1)))
	switch list[0].status {
	case -60, statusBlock:
		w.H(32)
	case -62, statusParry:
		w.H(64)
	case statusCriticalDodge, statusDodge:
		w.H(128)
	case -58, statusResist:
		w.H(256)
	default:
		w.H(0)
	}
	w.C(byte(len(list)))
	for _, hit := range list {
		w.D(hit.damage)
		w.C(byte(hit.status))
		w.C(hit.shield)
		if hit.shield == 1 {
			w.B(make([]byte, 20))
		}
	}
	w.C(0)
	return w
}

// attackStatus is SM_ATTACK_STATUS: what happened to a creature's life, and how much it has left.
func attackStatus(c creature, kind byte, skillID int32, value int32) *wire.Writer {
	hp, maxHP := c.hitPoints()
	w := wire.Packet(smAttackStatus)
	w.D(c.cid())
	if kind == statusDamage {
		w.D(-value)
	} else {
		w.D(value)
	}
	w.C(kind)
	w.C(byte(100 * int64(hp) / int64(max(maxHP, 1))))
	w.H(uint16(skillID))
	w.H(0xa6)
	return w
}

// lookAt is SM_LOOKATOBJECT: a creature turns to what it targets.
func (s *Server) lookAt(c creature) *wire.Writer {
	w := wire.Packet(smLookatobject)
	w.D(c.cid())
	if target := s.creatureByID(c.targetOf()); target != nil {
		w.D(target.cid())
		w.C(byte(int(math.Abs(float64(128 - int32(target.cheading()))))))
	} else {
		w.D(0)
		w.C(c.cheading())
	}
	return w
}

// attack is CM_ATTACK: the player hits what it targets.
func (c *conn) attack(r *wire.Reader) {
	p := c.player
	target := r.D()
	r.C()
	r.H()
	r.C()
	if p == nil || r.Err != nil || p.dead {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.playerAttack(p, s.creatureByID(target))
}

// playerAttack is PlayerController.attackTarget.
func (s *Server) playerAttack(p *player, target creature) {
	if s.restrictedInPrison(p, "attack") {
		return
	}
	if target == nil || target.isDead() || !p.canAttack() {
		return
	}
	if math.Abs(float64(p.Z-func() float32 { _, _, _, z := target.loc(); return z }())) > 6 {
		return
	}
	if o, ok := target.(*object); ok && !s.canAttackNpc(p, o) {
		return
	}
	if o, ok := target.(*object); ok && p.conn != nil && p.conn.flyingReconnaissanceAttack(o) {
		return
	}
	if o, ok := target.(*object); ok && p.conn != nil && p.conn.gaphyrksLoveAttack(o) {
		return
	}
	if other, ok := target.(*player); ok && (other == p || !s.isEnemyPlayer(p, other)) {
		return
	}
	speed := time.Duration(p.stats.current(data.AttackSpeed)) * time.Millisecond
	if now := time.Now(); now.Sub(p.lastAttack) < speed {
		return
	} else {
		p.lastAttack = now
	}
	p.fx.attacking(target)
	results := s.physicalAttack(p, target)
	var damage int32
	for _, hit := range results {
		damage += hit.damage
	}
	p.broadcast(attackPacket(p, target, p.attackCounter, int32(time.Now().UnixMilli()), 0, results), true)
	s.gotHit(target, p, 0, statusRegular, damage)
	p.attackCounter++
}

// canAttack is Creature.canAttack: nothing stops the player fighting.
func (p *player) canAttack() bool {
	return p.state&stateResting == 0 && p.state&statePrivateShop != statePrivateShop && !p.dead && p.cast == nil &&
		!p.fx.isState(stateCantAttack)
}

// canAttackNpc is PlayerRestrictions.canAttack for an npc: monsters always, others when they are aggressive to it.
func (s *Server) canAttackNpc(p *player, o *object) bool {
	if o.kisk != nil {
		return o.kisk.ownerRace != p.Race
	}
	switch o.npc.Type {
	case "ATTACKABLE", "AGGRESSIVE":
		return true
	}
	return s.aggressiveTo(o, p)
}

// gotHit is Creature.getController().onAttack: the target takes the damage and reacts.
func (s *Server) gotHit(target, attacker creature, skillID int32, kind byte, damage int32) {
	switch t := target.(type) {
	case *player:
		s.playerHit(t, attacker, skillID, kind, damage)
	case *object:
		s.npcHit(t, attacker, skillID, kind, damage)
	}
}
