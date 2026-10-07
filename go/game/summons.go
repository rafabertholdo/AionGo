package game

import (
	"time"

	"aionlightning/game/data"
)

// Traps and servants: npcs a player's skill puts into the world for a while, that use a skill of their own on the
// player's enemies (AL-Game's Trap, Servant, TrapAi and ServantAi).
// ponytail: the summons that follow their owner and take commands (SummonEffect) aren't ported.

const (
	servantLifetime = 60 * time.Second
	servantInterval = 10 * time.Second // ServantSkillUseDesire's execution interval
	trapInterval    = 2 * time.Second  // TrapExplodeDesire's
)

// spawnOwned puts an npc of the player's making where the player stands.
func (s *Server) spawnOwned(owner *player, npcID int32, objectType uint16) *object {
	return s.spawnOwnedAt(owner, npcID, objectType, byte(owner.Heading))
}

// spawnOwnedAt is spawnOwned facing a given way.
func (s *Server) spawnOwnedAt(owner *player, npcID int32, objectType uint16, heading byte) *object {
	o := s.newOwned(owner, npcID, objectType, heading)
	if o != nil {
		s.addObject(o)
	}
	return o
}

// newOwned is the owned npc before it is put in the world.
func (s *Server) newOwned(owner *player, npcID int32, objectType uint16, heading byte) *object {
	t := s.data.Npcs[npcID]
	if t == nil {
		return nil
	}
	o := &object{id: s.ids.nextID(), worldID: owner.WorldID, x: owner.X, y: owner.Y, z: owner.Z, heading: heading, instance: owner.instance,
		npc: t, homeX: owner.X, homeY: owner.Y, homeZ: owner.Z, owner: owner, objectType: objectType}
	s.initNpc(o)
	s.byID[o.id] = o
	return o
}

// despawnOwned is NpcController.onDespawn for a trap or servant: it leaves the world for good.
func (s *Server) despawnOwned(o *object) {
	if o.dead {
		return
	}
	for _, t := range o.timers {
		t.cancel()
	}
	o.timers = nil
	o.dead = true
	s.removeObject(o)
	delete(s.byID, o.id)
	s.ids.release(o.id)
}

// spawnTrap is SummonTrapEffect.applyEffect: the trap waits, and explodes into its skill at the first enemy that comes near.
func (s *Server) spawnTrap(owner *player, npcID, skillID, seconds int32) {
	o := s.spawnOwned(owner, npcID, 32)
	if o == nil {
		return
	}
	lifetime := s.later(time.Duration(seconds)*time.Second, func() { s.despawnOwned(o) })
	watch := s.every(trapInterval, trapInterval, func() {
		if o.dead || s.spawned[owner.ID] == nil {
			return
		}
		for _, c := range s.creaturesNear(o, float32(o.npc.SRange)) {
			if c == creature(o) || c.isDead() || !s.isEnemyOf(owner, c) {
				continue
			}
			tmpl := s.data.Skills[skillID]
			if tmpl == nil {
				return
			}
			for _, t := range o.timers {
				t.cancel()
			}
			(&skill{s: s, tmpl: tmpl, effector: o, level: 1, first: c}).use()
			s.later(time.Duration(tmpl.Duration)*time.Millisecond+time.Second, func() { s.despawnOwned(o) })
			return
		}
	})
	o.timers = []*task{watch, lifetime}
}

// spawnServant is SummonServantEffect.applyEffect: the servant uses its skill on what its owner has selected, and
// loses a third of its life each time; it is gone after a minute.
func (s *Server) spawnServant(owner *player, npcID, skillID int32) {
	o := s.spawnOwned(owner, npcID, 1024)
	if o == nil {
		return
	}
	lifetime := s.later(servantLifetime, func() { s.despawnOwned(o) })
	watch := s.every(servantInterval, servantInterval, func() {
		target := s.creatureByID(owner.targetID)
		if o.dead || target == nil || target.isDead() || !s.isEnemyOf(owner, target) {
			return
		}
		tmpl := s.data.Skills[skillID]
		if tmpl == nil {
			return
		}
		(&skill{s: s, tmpl: tmpl, effector: o, level: 1, first: target}).use()
		s.reduceNpcHP(o, int32(data.Round(float32(o.maxHP)/3)), nil)
	})
	o.timers = []*task{watch, lifetime}
}
