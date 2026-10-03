package game

import (
	"time"

	"aionlightning/game/data"
)

// Instances: copies of a dungeon map for a group or a player (AL-Game's InstanceService and WorldMapInstance).
// A player's or a group's instance is the one registered for them; it and its spawns are made when someone
// enters, and go when it has been empty for a minute.

const instanceCheck = time.Minute

// instance is a WorldMapInstance of an instance map.
type instance struct {
	world, id  int32
	registered map[int32]bool // the players (or the group) it is for, by object id
	group      *group
	check      *task
}

func (s *Server) instanceCount(world int32) int32 {
	var n int32
	for k := range s.instances {
		if k[0] == world {
			n++
		}
	}
	return n
}

// registeredInstance is InstanceService.getRegisteredInstance.
func (s *Server) registeredInstance(world, key int32) *instance {
	for k, in := range s.instances {
		if k[0] == world && in.registered[key] {
			return in
		}
	}
	return nil
}

// newInstance is InstanceService.getNextAvailableInstance: a new copy of the map, with its npcs.
func (s *Server) newInstance(world int32) *instance {
	if s.instances == nil {
		s.instances, s.nextInstance = map[[2]int32]*instance{}, map[int32]int32{}
	}
	id := s.nextInstance[world]
	s.nextInstance[world]++
	in := &instance{world: world, id: id, registered: map[int32]bool{}}
	s.instances[[2]int32{world, id}] = in
	s.spawnMap(world, id)
	in.check = s.every(instanceCheck, instanceCheck, func() { s.checkInstance(in) })
	s.log.Info("instance created", "map", world, "instance", id)
	return in
}

// playersIn are the players standing in the instance.
func (s *Server) playersIn(in *instance) []*player {
	var list []*player
	for _, p := range s.spawned {
		if p.WorldID == in.world && p.instance == in.id {
			list = append(list, p)
		}
	}
	return list
}

// anyOnline is whether one of the players is in the game, even if it is still loading a map.
func (s *Server) anyOnline(ids map[int32]bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range ids {
		if s.players[id] != nil {
			return true
		}
	}
	return false
}

// checkInstance is EmptyInstanceCheckerTask: an instance nobody is in is destroyed.
func (s *Server) checkInstance(in *instance) {
	switch {
	case in.group != nil && len(in.group.members) == 0:
	case in.group == nil && len(s.playersIn(in)) == 0 && !s.anyOnline(in.registered):
	default:
		return
	}
	s.destroyInstance(in)
}

// destroyInstance is InstanceService.destroyInstance: its npcs are gone, and those still in it go to the entry point.
func (s *Server) destroyInstance(in *instance) {
	in.check.cancel()
	delete(s.instances, [2]int32{in.world, in.id})
	for _, id := range sortedKeys(s.byID) {
		o := s.byID[id]
		if o.worldID != in.world || o.instance != in.id {
			continue
		}
		for _, t := range o.timers {
			t.cancel()
		}
		o.respawn.cancel()
		o.decay.cancel()
		if o.npc != nil && o.ai != nil {
			o.ai.handleEvent(evDespawn)
		}
		s.removeObject(o)
		delete(s.byID, id)
		s.ids.release(id)
	}
	for _, p := range s.playersIn(in) {
		s.leaveInstance(p)
	}
	s.log.Info("instance destroyed", "map", in.world, "instance", in.id)
}

// leaveInstance is InstanceService.moveToEntryPoint: the player is taken out of an instance map.
func (s *Server) leaveInstance(p *player) {
	portal := s.data.InstancePortal(p.WorldID, p.Race)
	if portal == nil {
		return
	}
	if at := portal.EntryFor(p.Race); at != nil {
		s.teleportToInstance(p, at.MapID, 0, at.X, at.Y, at.Z, byte(p.Heading), 0)
	}
}

// instanceLogin is InstanceService.onPlayerLogin: a player who logs in inside an instance map goes back to its own
// instance if it still exists, and to the entry point if not.
func (s *Server) instanceLogin(p *player) {
	m := s.data.WorldMaps[p.WorldID]
	if m == nil || !m.Instance {
		return
	}
	portal := s.data.InstancePortal(p.WorldID, p.Race)
	if portal == nil {
		return
	}
	key := p.ID
	if portal.Group && p.group != nil {
		key = p.group.id
	}
	if in := s.registeredInstance(p.WorldID, key); in != nil {
		p.instance = in.id
		return
	}
	if at := portal.EntryFor(p.Race); at != nil {
		p.WorldID, p.instance, p.X, p.Y, p.Z = at.MapID, 0, at.X, at.Y, at.Z
	}
}

// Messages of portals.
const (
	msgPortalRace        = 901354  // STR_MOVE_PORTAL_ERROR_INVALID_RACE
	msgInstanceLevel     = 1400179 // STR_MSG_CANT_INSTANCE_ENTER_LEVEL
	msgInstanceGroupOnly = 1390256 // STR_MSG_ENTER_ONLY_PARTY_DON
)

// portalUse is PortalController.analyzePortation: whether the player may go through, and to which instance.
func (s *Server) portalUse(p *player, portal *data.Portal) {
	if portal.TitleID != 0 && p.TitleID != portal.TitleID {
		return
	}
	if portal.Race != "" && portal.Race != "ALL" && portal.Race != p.Race {
		p.conn.send(systemMessage(msgPortalRace))
		return
	}
	if (portal.MaxLevel != 0 && int32(p.level) > portal.MaxLevel) || int32(p.level) < portal.MinLevel {
		p.conn.send(systemMessage(msgInstanceLevel))
		return
	}
	world := portal.Exit.MapID
	transfer := func(in *instance) {
		s.teleportToInstance(p, world, in.id, portal.Exit.X, portal.Exit.Y, portal.Exit.Z, 0, 0)
	}
	switch {
	case portal.Group && p.group == nil:
		p.conn.send(systemMessage(msgInstanceGroupOnly))
	case portal.Group:
		in := s.registeredInstance(world, p.group.id)
		if in == nil {
			in = s.newInstance(world)
			in.group = p.group
			in.registered[p.group.id] = true
		}
		transfer(in)
	default:
		if in := s.registeredInstance(world, p.ID); in != nil {
			transfer(in)
			return
		}
		if portal.Instance {
			in := s.newInstance(world)
			in.registered[p.ID] = true
			transfer(in)
			return
		}
		s.teleportToInstance(p, world, 0, portal.Exit.X, portal.Exit.Y, portal.Exit.Z, 0, 0)
	}
}
