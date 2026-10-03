package game

// These non-attackable soldiers act out the aerial battle in the introductory
// campaign visions. Their neutral tribes and unequal levels are unsuitable for
// ordinary combat: the scene must keep playing while the player talks below it.
func aerialBattleActor(o *object) bool {
	if o.npc == nil || o.spawnSpot.Fly == 0 || (o.worldID != 310010000 && o.worldID != 320010000) {
		return false
	}
	switch o.npc.ID {
	case 205003, 205004, 205005, 205006, 205021, 205022:
		return true
	}
	return false
}

type aerialBattleDesire struct{ paced }

func (d *aerialBattleDesire) same(other desire) bool {
	_, ok := other.(*aerialBattleDesire)
	return ok
}

func (d *aerialBattleDesire) handle(a *npcAI) bool {
	d.interval, d.strength = 2, aiActive.priority()
	s, o := a.s, a.o
	if o.dead || s.byID[o.id] != o {
		return false
	}
	var target *object
	nearest := float64(visibilityDistance)
	for _, other := range s.npcsNear(o, visibilityDistance) {
		if !aerialBattleActor(other) || other.npc.Race == o.npc.Race {
			continue
		}
		distance := dist(o, other)
		if distance < nearest || (distance == nearest && target != nil && other.id < target.id) {
			target, nearest = other, distance
		}
	}
	if target == nil {
		return true
	}
	if o.targetID != target.id {
		o.targetID = target.id
		o.state |= stateWeapon
		o.broadcast(s.lookAt(o), true)
		o.broadcast(s.emote(o, emoteAttackMode, target.id), true)
	}
	o.broadcast(attackPacket(o, target, o.attackCounter, 274, 0,
		[]attackResult{{status: statusNormalHit}}), true)
	o.attackCounter++
	return true
}
