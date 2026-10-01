package game

import (
	"math"
	"strconv"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// A player's life, as AL-Game's PlayerLifeStats keeps it: hit points, mana and flight time, which come back
// on their own, and are told to the client in the next hundred milliseconds after they change.

const (
	restoreDelay  = 1700 * time.Millisecond
	restorePeriod = 6000 * time.Millisecond
	flushPeriod   = 100 * time.Millisecond
)

// System messages of life and experience.
const (
	msgExp = 1370002 // "You have gained %0 XP."
	msgDie = 1340000
)

// startWorldTasks starts what runs for the whole server: the flush of life changes to clients.
func (s *Server) startWorldTasks() {
	s.every(flushPeriod, flushPeriod, func() {
		for _, p := range s.spawned {
			if p.dirtyHP {
				p.dirtyHP = false
				p.conn.send(statUpdate(smStatupdateHp, p.life.HP, p.stats.current(data.MaxHP)))
				s.updateGroupOf(p, groupMovement)
			}
			if p.dirtyMP {
				p.dirtyMP = false
				p.conn.send(statUpdate(smStatupdateMp, p.life.MP, p.stats.current(data.MaxMP)))
				s.updateGroupOf(p, groupMovement)
			}
		}
		s.flushEffects()
	})
	// GroupAllianceUpdater: where the members are, every two seconds.
	s.every(groupInterval, groupInterval, func() {
		for _, p := range s.spawned {
			s.updateGroupOf(p, groupMovement)
			s.updateAllianceOf(p)
		}
	})
}

// playerHit is PlayerController.onAttack: the player takes the damage.
func (s *Server) playerHit(p *player, attacker creature, skillID int32, kind byte, damage int32) {
	if p.dead {
		return
	}
	// Damage is cut to what kills, so that the aggro list doesn't count a hit's excess.
	damage = min(damage, p.life.HP+1)
	s.cancelOnHit(p, damage)
	p.recordDamage(attacker, damage)
	p.fx.attacked(attacker)
	s.reducePlayerHP(p, damage, attacker)
	p.broadcast(attackStatus(p, kind, skillID, damage), true)
}

// reducePlayerHP is CreatureLifeStats.reduceHp for a player.
func (s *Server) reducePlayerHP(p *player, value int32, attacker creature) {
	hp := p.life.HP - value
	if hp <= 0 {
		// Losing a duel doesn't kill: DuelService.onDie.
		if a, ok := attacker.(*player); ok && s.duels[a.ID] == p.ID {
			s.loseDuel(p)
			p.life.HP = 1
			p.dirtyHP = true
			s.triggerRestore(p)
			return
		}
		hp = 0
		if !p.dead {
			p.dead = true
		}
	}
	p.life.HP = hp
	p.dirtyHP = true
	s.triggerRestore(p)
	if p.dead {
		s.playerDied(p, attacker)
	}
}

func (s *Server) reducePlayerMP(p *player, value int32) {
	p.life.MP = max(p.life.MP-value, 0)
	p.dirtyMP = true
	s.triggerRestore(p)
}

// increasePlayerHP is CreatureLifeStats.increaseHp for a player.
func (s *Server) increasePlayerHP(p *player, kind byte, value int32) {
	if p.dead {
		return
	}
	hp := min(p.life.HP+value, p.stats.current(data.MaxHP))
	if hp == p.life.HP {
		return
	}
	p.life.HP = hp
	p.dirtyHP = true
	s.lifeChanged(p, kind, value)
}

func (s *Server) increasePlayerMP(p *player, kind byte, value int32) {
	if p.dead {
		return
	}
	mp := min(p.life.MP+value, p.stats.current(data.MaxMP))
	if mp == p.life.MP {
		return
	}
	p.life.MP = mp
	p.dirtyMP = true
	s.lifeChanged(p, kind, value)
}

// lifeChanged is PlayerLifeStats.sendAttackStatusPacketUpdate: the player is told, and the others see its health.
func (s *Server) lifeChanged(p *player, kind byte, value int32) {
	p.conn.send(attackStatus(p, kind, 0, value))
	p.broadcast(attackStatus(p, statusRegular, 0, 0), false)
}

// triggerRestore starts the regeneration that runs while life or mana is missing.
func (s *Server) triggerRestore(p *player) {
	if p.restore != nil || p.dead {
		return
	}
	p.restore = s.every(restoreDelay, restorePeriod, func() {
		if p.dead || (p.life.HP == p.stats.current(data.MaxHP) && p.life.MP == p.stats.current(data.MaxMP)) {
			p.restore.cancel()
			p.restore = nil
			return
		}
		hp, mp := p.stats.current(data.RegenHp), p.stats.current(data.RegenMp)
		if p.state&stateResting != 0 {
			hp *= 8
			mp *= 8
		}
		s.increasePlayerHP(p, statusNaturalHP, hp)
		s.increasePlayerMP(p, statusNaturalMP, mp)
	})
}

// playerDied is PlayerController.onDie.
// ponytail: experience loss, the summon, duels, pvp and quest hooks wait for their milestones.
func (s *Server) playerDied(p *player, attacker creature) {
	rebirth := s.rebirthPercent(p)
	item := s.selfReviveStone(p) != nil
	p.state |= stateDead
	p.rebirthPercent = rebirth
	s.cancelFp(p)
	p.cast = nil
	p.fx.removeAll()
	if o, byNpc := attacker.(*object); ((byNpc && o.owner == nil) || attacker == creature(p)) && p.level > 4 {
		s.calculateExpLoss(p)
	}
	s.pvpReward(p)
	if p.conn != nil {
		p.conn.sealingAbyssGateDeath()
		p.conn.ascensionDeath()
	}
	if p.restore != nil {
		p.restore.cancel()
		p.restore = nil
	}
	var by int32
	if attacker != nil {
		by = attacker.cid()
	}
	p.broadcast(emotionPacket(p.ID, emoteDie, p.state, float32(p.stats.current(data.Speed))/speedScale, 0, by, 0, 0, 0, 0, 0, 0), true)
	die := wire.Packet(smDie)
	die.Bool(rebirth != 0) // skill revive
	die.Bool(item)
	die.D(kiskRemaining(p))
	p.conn.send(die)
	p.conn.send(systemMessage(msgDie))
}

// expLossParams is XPLossEnum: the percent of a level's experience lost dying, up to each level.
var expLossParams = []struct {
	level int
	param float64
}{{8, 2.997997521}, {9, 2.998974359}, {10, 2.999872482}, {16, 2.999258215}, {20, 2.999859021}, {21, 2.999782255},
	{22, 2.999856511}, {24, 2.999925915}, {33, 2.999791422}, {41, 1.369142798}, {44, 1.081953696}, {50, 1.041314239}}

// calculateExpLoss is PlayerCommonData.calculateExpLoss: two thirds of what dying costs can be recovered by a soul healer.
func (s *Server) calculateExpLoss(p *player) {
	need := int64(0)
	if p.level != s.data.MaxLevel() {
		need = s.data.ExpStart(p.level+1) - s.data.ExpStart(p.level)
	}
	var lost int64
	if p.level >= 8 {
		for _, e := range expLossParams {
			if p.level <= e.level {
				lost = int64(math.Round(float64(need/100) * e.param))
				break
			}
		}
	}
	unrecoverable := int64(int32(float64(lost) * 0.33333333))
	recoverable := int64(int32(lost)) - unrecoverable
	shown := func() int64 { return p.Exp - s.data.ExpStart(p.level) }
	all := recoverable + p.RecoverExp
	if shown() > unrecoverable {
		p.Exp -= unrecoverable
	} else {
		p.Exp -= shown()
	}
	if shown() > recoverable {
		p.RecoverExp = all
		p.Exp -= recoverable
	} else {
		p.RecoverExp += shown()
		p.Exp -= shown()
	}
	p.conn.send(s.expUpdate(p))
}

// resetRecoverableExp is what a soul healer does: the experience that could be recovered is given back.
func (s *Server) resetRecoverableExp(p *player) {
	back := p.RecoverExp
	p.RecoverExp = 0
	s.setExp(p, p.Exp+back)
}

// giveExp is PlayerCommonData.addExp: the experience, and a level if it is enough.
func (s *Server) giveExp(p *player, exp int64) {
	if exp == 0 {
		return
	}
	// PlayerCommonData.addExp: the new experience first (SM_STATUPDATE_EXP, or the level-up), then the message; the old
	// server's log of a kill shows that order.
	s.setExp(p, p.Exp+exp)
	p.conn.send(systemMessage(msgExp, strconv.FormatInt(exp, 10)))
}

// setExp is PlayerCommonData.setExp.
func (s *Server) setExp(p *player, exp int64) {
	maxLevel := s.data.MaxLevel()
	if class := p.Class; startingClass(class) {
		maxLevel = 10
	}
	if maxExp := s.data.ExpStart(maxLevel); exp > maxExp {
		exp = maxExp
	}
	level := 1
	for level+1 != maxLevel && exp >= s.data.ExpStart(level+1) {
		level++
	}
	p.Exp = exp
	if level != p.level {
		p.level = level
		s.levelUp(p)
		return
	}
	p.conn.send(s.expUpdate(p))
}

// expUpdate is SM_STATUPDATE_EXP.
func (s *Server) expUpdate(p *player) *wire.Writer {
	w := wire.Packet(smStatupdateExp)
	w.Q(p.Exp - s.data.ExpStart(p.level))
	w.Q(p.RecoverExp)
	need := int64(0)
	if p.level != s.data.MaxLevel() {
		need = s.data.ExpStart(p.level+1) - s.data.ExpStart(p.level)
	}
	w.Q(need)
	w.Q(0)
	w.Q(0)
	return w
}

// levelUp is PlayerController.upgradePlayer: the stats of the new level, full life, and the news.
func (s *Server) levelUp(p *player) {
	p.stats = s.playerStats(p)
	p.life.HP = p.stats.current(data.MaxHP)
	p.life.MP = p.stats.current(data.MaxMP)
	p.life.FP = p.stats.current(data.FlyTime)
	p.dirtyHP, p.dirtyMP = true, true
	update := wire.Packet(smLevelUpdate)
	update.D(p.ID)
	update.H(0)
	update.H(uint16(p.level))
	update.H(0)
	p.broadcast(update, true)
	p.conn.send(s.statsInfo(p))
	if p.level == 10 {
		s.switchCraftingSkill(p)
	}
	s.learnNewSkills(p)
	s.classChangeDialog(p)
	if p.conn != nil {
		p.conn.questLevelUp()
	}
	s.updateGroupOf(p, groupUpdate)
}
