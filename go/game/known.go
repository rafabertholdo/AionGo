package game

import (
	"math"

	"aionlightning/wire"
)

// visibilityDistance is how close two players in a map must be to see each
// other, and how far apart their heights may be (KnownList).
const visibilityDistance = 95

// inRange is KnownList.checkObjectInRange: in the same map, within the distance
// in the plane and in height.
// ponytail: every spawned player is checked on each move (AL-Game keeps map
// regions to skip the far ones); a grid when a map holds hundreds of players.
func inRange(a, b *player) bool {
	if a.WorldID != b.WorldID || a.instance != b.instance || math.Abs(float64(a.Z-b.Z)) > visibilityDistance {
		return false
	}
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx+dy*dy < visibilityDistance*visibilityDistance
}

// Removal animation speeds of SM_DELETE: none when an object is out of range, 15 when it leaves.
const (
	deleteOutOfRange = 0
	deleteLeaving    = 15
)

func deleteObject(id int32, speed byte) *wire.Writer {
	w := wire.Packet(smDelete)
	w.D(id)
	w.C(speed)
	return w
}

// enemy is Player.isEnemyPlayer, without duels.
func enemy(p, other *player) bool {
	return p.Race != other.Race
}

// spawn is World.spawn: p appears in its map, seen by the players near it, who it sees.
func (s *Server) spawn(p *player) {
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.spawnLocked(p)
}

// spawnLocked is spawn for a caller that holds the world lock.
func (s *Server) spawnLocked(p *player) {
	p.spawned = true
	s.spawned[p.ID] = p
	s.addPlayerCell(p)
	s.updateKnown(p)
}

// despawn is World.despawn: p disappears from the players who see it.
func (s *Server) despawn(p *player) {
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.despawnLocked(p)
}

// despawnLocked is despawn for a caller that holds the world lock.
func (s *Server) despawnLocked(p *player) {
	if !p.spawned {
		return
	}
	p.spawned = false
	delete(s.spawned, p.ID)
	for _, other := range p.known {
		s.forget(other, p, deleteLeaving)
	}
	clear(p.known)
	for _, o := range p.seen {
		s.forgetObject(p, o, deleteLeaving)
	}
	s.removePlayerCell(p)
}

// updateKnown is KnownList.updateKnownList: p forgets the players out of its
// range and finds those in it, each seeing the other. The caller holds visMu.
func (s *Server) updateKnown(p *player) {
	for _, other := range p.known {
		if !inRange(p, other) {
			s.forget(p, other, deleteOutOfRange)
			s.forget(other, p, deleteOutOfRange)
		}
	}
	for _, other := range s.spawned {
		if other == p || p.known[other.ID] != nil || !inRange(p, other) {
			continue
		}
		p.known[other.ID] = other
		other.known[p.ID] = p
		p.conn.send(s.playerInfo(other, enemy(p, other)))
		other.conn.send(s.playerInfo(p, enemy(other, p)))
	}

	for _, o := range p.seen {
		if !o.inRange(p) {
			s.forgetObject(p, o, deleteOutOfRange)
		}
	}
	around := cellAt(p.WorldID, p.instance, p.X, p.Y)
	for dx := int32(-1); dx <= 1; dx++ {
		for dy := int32(-1); dy <= 1; dy++ {
			for _, o := range s.grid[cell{around.world, around.inst, around.x + dx, around.y + dy}] {
				if p.seen[o.id] == nil && o.inRange(p) {
					s.seeObject(p, o)
				}
			}
		}
	}
}

// forget makes p stop seeing other, which its client is told to remove.
func (s *Server) forget(p, other *player, speed byte) {
	delete(p.known, other.ID)
	p.conn.send(deleteObject(other.ID, speed))
}

// broadcast sends w to the players who see p, and to p as well if toSelf.
// The caller holds visMu.
func (p *player) broadcast(w *wire.Writer, toSelf bool) {
	if toSelf {
		p.conn.send(w)
	}
	for _, other := range p.known {
		other.conn.send(w)
	}
}
