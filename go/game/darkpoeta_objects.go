package game

// Dark Poeta barricades lose one HP for each damaging hit. Apply the cap after
// avoidance and shields so misses and fully absorbed hits remain zero.
func darkPoetaDamage(target creature, damage int32) int32 {
	if damage > 0 && darkPoetaBarricade(target) {
		return 1
	}
	return damage
}

func darkPoetaBarricade(target creature) bool {
	o, ok := target.(*object)
	if !ok || o.worldID != darkPoetaWorld || o.npc == nil {
		return false
	}
	switch o.npc.ID {
	case 700517, 700556, 700558:
		return true
	}
	return false
}

// Keep separate normal-attack hits separate at the barricade's HP/aggro hook.
// Collapsing a three-hit attack before trimming would lose only one HP.
func (s *Server) applyAttackHits(target, attacker creature, results []attackResult) {
	if darkPoetaBarricade(target) {
		for _, hit := range results {
			s.gotHit(target, attacker, 0, statusRegular, hit.damage)
		}
		return
	}
	var damage int32
	for _, hit := range results {
		damage += hit.damage
	}
	s.gotHit(target, attacker, 0, statusRegular, damage)
}

// The Brownie Bomb is self/area cast; the rock wall has no attack cursor.
// Observe successful casts within the skill's seven-metre area in this run.
func (s *Server) darkPoetaSpell(sk *skill) {
	p, ok := sk.effector.(*player)
	if !ok || sk.tmpl.ID != 18130 || p.WorldID != darkPoetaWorld || p.dead || !p.spawned || sk.item == nil || sk.item.ID != 164000096 {
		return
	}
	if s.darkPoetaOf(p.WorldID, p.instance) == nil {
		return
	}
	for _, o := range s.byID {
		if o.worldID != p.WorldID || o.instance != p.instance || o.npc == nil || o.npc.ID != 700516 || o.dead || distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 7 {
			continue
		}
		// Retire its identity as well as its visual object; stale spell events cannot
		// remove it again, and the next instance gets a fresh static spawn.
		s.despawnNpc(o, true)
		o.respawn.cancel()
		delete(s.byID, o.id)
		s.ids.release(o.id)
	}
}
