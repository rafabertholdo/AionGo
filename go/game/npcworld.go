package game

// How npcs come into the players' view and leave it: World.spawn, World.despawn and KnownList for an npc.

// addObject is World.spawn for an npc: it stands in the map, and those near it see it, and it them.
func (s *Server) addObject(o *object) {
	c := cellAt(o.worldID, o.instance, o.x, o.y)
	s.grid[c] = append(s.grid[c], o)
	s.updateNpcKnown(o)
}

// removeObject is World.despawn for an npc: those who see it are told it is gone.
func (s *Server) removeObject(o *object) {
	for _, p := range o.watchers {
		s.forgetObject(p, o, deleteLeaving)
	}
	s.removeFromGrid(o)
}

func (s *Server) removeFromGrid(o *object) {
	c := cellAt(o.worldID, o.instance, o.x, o.y)
	list := s.grid[c]
	for i, other := range list {
		if other == o {
			list[i] = list[len(list)-1]
			s.grid[c] = list[:len(list)-1]
			return
		}
	}
}

// moveObject is World.setPosition: the npc is put somewhere without anyone being told.
func (s *Server) moveObject(o *object, x, y, z float32, heading byte) {
	s.removeFromGrid(o)
	o.x, o.y, o.z, o.heading = x, y, z, heading
}

// moveNpcTo is World.updatePosition for an npc: it is there now, and who sees it is looked at again if update.
func (s *Server) moveNpcTo(o *object, x, y, z float32, heading byte, update bool) {
	old := cellAt(o.worldID, o.instance, o.x, o.y)
	o.x, o.y, o.z, o.heading = x, y, z, heading
	if now := cellAt(o.worldID, o.instance, x, y); now != old {
		list := s.grid[old]
		for i, other := range list {
			if other == o {
				list[i] = list[len(list)-1]
				s.grid[old] = list[:len(list)-1]
				break
			}
		}
		s.grid[now] = append(s.grid[now], o)
	}
	if update {
		s.updateNpcKnown(o)
	}
}

// updateNpcKnown is KnownList.updateKnownList for an npc: players out of its range lose sight of it, those in range see it.
func (s *Server) updateNpcKnown(o *object) {
	for _, p := range o.watchers {
		if !o.inRange(p) {
			s.forgetObject(p, o, deleteOutOfRange)
		}
	}
	around := cellAt(o.worldID, o.instance, o.x, o.y)
	for dx := int32(-1); dx <= 1; dx++ {
		for dy := int32(-1); dy <= 1; dy++ {
			for _, p := range s.pcells[cell{around.world, around.inst, around.x + dx, around.y + dy}] {
				if o.watchers[p.ID] == nil && o.inRange(p) {
					s.seeObject(p, o)
				}
			}
		}
	}
}

// seeObject makes the player see the npc or gatherable, which sees it back (PlayerController.see, NpcController.see).
func (s *Server) seeObject(p *player, o *object) {
	p.seen[o.id] = o
	if o.npc == nil {
		p.conn.send(gatherableInfo(o))
		return
	}
	o.watchers[p.ID] = p
	p.conn.send(s.npcInfo(o, p))
	if o.rift != nil && o.rift.master {
		p.conn.send(riftStatus(o))
	}
	if len(s.data.QuestStarts[o.npc.ID]) > 0 {
		p.conn.send(s.nearbyQuests(p))
	}
	if o.move.walking {
		p.conn.send(s.emote(o, emoteWalk, 0))
	}
	o.ai.handleEvent(evSeePlayer)
}

// forgetObject makes the player lose sight of the npc or gatherable (Controller.notSee).
func (s *Server) forgetObject(p *player, o *object, speed byte) {
	delete(p.seen, o.id)
	p.conn.send(deleteObject(o.id, speed))
	if o.npc != nil {
		if len(s.data.QuestStarts[o.npc.ID]) > 0 {
			p.conn.send(s.nearbyQuests(p))
		}
	}
	if p.targetID == o.id {
		p.targetID = 0
		p.broadcast(s.lookAt(p), false)
	}
	if o.npc == nil {
		return
	}
	delete(o.watchers, p.ID)
	if o.targetID == p.ID {
		o.targetID = 0
		o.broadcast(s.lookAt(o), true)
	}
	delete(o.aggro, p.ID)
	o.ai.handleEvent(evNotSeePlayer)
}

// addPlayerCell and removePlayerCell keep the squares of players.
func (s *Server) addPlayerCell(p *player) {
	c := cellAt(p.WorldID, p.instance, p.X, p.Y)
	if s.pcells[c] == nil {
		s.pcells[c] = map[int32]*player{}
	}
	s.pcells[c][p.ID] = p
	p.cell = c
}

func (s *Server) removePlayerCell(p *player) {
	delete(s.pcells[p.cell], p.ID)
	if len(s.pcells[p.cell]) == 0 {
		delete(s.pcells, p.cell)
	}
}
